package extract

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOllamaURL   = "http://127.0.0.1:11434"
	DefaultOllamaModel = "ministral-3:latest"
	DefaultCleanupDPI  = 120

	cleanupSystemPrompt = `You are correcting OCR and spelling errors in extracted PDF text using the page image as ground truth.

Rules:
- Compare the extracted text to the attached page image and fix OCR mistakes, broken words, glued words, and obvious spelling errors.
- Prefer the image when the extracted text conflicts with what is visibly printed.
- Preserve reading order, headings, lists, and paragraph structure as plain text.
- Keep game terms, die codes (for example 3D+1), names, URLs, and numbers unchanged when they look intentional.
- Never use Markdown or markup of any kind (no **, *, _, #, backticks, or [links](url)).
- Do not add commentary, invented headings, or explanations.
- Do not invent missing content; if text is truncated or unclear, leave it alone.
- Output only the corrected plain text.`
)

var (
	mdBoldRE  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdLinkRE  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdHeading = regexp.MustCompile(`(?m)^#{1,6}\s+`)
)

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	System  string                 `json:"system,omitempty"`
	Images  []string               `json:"images,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (o Options) ollamaURL() string {
	if o.OllamaURL != "" {
		return strings.TrimRight(o.OllamaURL, "/")
	}
	return DefaultOllamaURL
}

func (o Options) ollamaModel() string {
	if o.OllamaModel != "" {
		return o.OllamaModel
	}
	return DefaultOllamaModel
}

func (o Options) cleanupDPI() int {
	if o.CleanupDPI > 0 {
		return o.CleanupDPI
	}
	return DefaultCleanupDPI
}

func checkOllama(opts Options) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(opts.ollamaURL() + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", opts.ollamaURL(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned HTTP %d from %s/api/tags", resp.StatusCode, opts.ollamaURL())
	}
	return nil
}

func renderPagePNG(inputPath, tmpDir string, page, dpi int) (string, error) {
	pageDir := filepath.Join(tmpDir, fmt.Sprintf("cleanup-%d", page))
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return "", fmt.Errorf("creating cleanup temp dir: %w", err)
	}

	prefix := filepath.Join(pageDir, "page")
	cmd := exec.Command(
		"pdftoppm",
		"-f", strconv.Itoa(page),
		"-l", strconv.Itoa(page),
		"-png",
		"-r", strconv.Itoa(dpi),
		"-singlefile",
		inputPath,
		prefix,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pngPath := prefix + ".png"
	if _, err := os.Stat(pngPath); err != nil {
		return "", fmt.Errorf("pdftoppm did not produce %s: %w", pngPath, err)
	}
	return pngPath, nil
}

func cleanupPage(inputPath, tmpDir, text string, page int, opts Options) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text, nil
	}

	pngPath, err := renderPagePNG(inputPath, tmpDir, page, opts.cleanupDPI())
	if err != nil {
		return "", err
	}
	defer os.Remove(pngPath)

	img, err := os.ReadFile(pngPath)
	if err != nil {
		return "", fmt.Errorf("reading page image: %w", err)
	}

	return cleanupWithImage(text, img, opts)
}

func cleanupWithImage(text string, imagePNG []byte, opts Options) (string, error) {
	reqBody := ollamaGenerateRequest{
		Model:  opts.ollamaModel(),
		System: cleanupSystemPrompt,
		Prompt: "Here is the extracted text for this page image. Correct it using the image as ground truth:\n\n" + text,
		Stream: false,
		Images: []string{base64.StdEncoding.EncodeToString(imagePNG)},
		Options: map[string]interface{}{
			"temperature": 0.1,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encoding ollama request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Post(opts.ollamaURL()+"/api/generate", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama generate HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed ollamaGenerateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("ollama: %s", parsed.Error)
	}

	cleaned := stripCleanupWrapper(parsed.Response)
	cleaned = stripMarkdownArtifacts(cleaned)
	if strings.TrimSpace(cleaned) == "" {
		return text, nil
	}
	return cleaned, nil
}

func stripCleanupWrapper(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

func stripMarkdownArtifacts(s string) string {
	s = mdLinkRE.ReplaceAllString(s, "$1")
	s = mdBoldRE.ReplaceAllString(s, "$1")
	s = mdHeading.ReplaceAllString(s, "")
	return s
}
