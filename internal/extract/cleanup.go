package extract

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultCleanupDPI = 120

	cleanupPlainSystemPrompt = `You are correcting OCR and spelling errors in extracted PDF text using the page image as ground truth.

Rules:
- Compare the extracted text to the attached page image and fix OCR mistakes, broken words, glued words, and obvious spelling errors.
- Prefer the image when the extracted text conflicts with what is visibly printed.
- Preserve reading order, headings, lists, and paragraph structure as plain text.
- Keep game terms, die codes (for example 3D+1), names, URLs, and numbers unchanged when they look intentional.
- Never use Markdown or markup of any kind (no **, *, _, #, backticks, or [links](url)).
- Do not add commentary, invented headings, or explanations.
- Do not invent missing content; if text is truncated or unclear, leave it alone.
- Output only the corrected plain text.`

	cleanupMarkdownSystemPrompt = `You are correcting OCR and spelling errors in extracted PDF text using the page image as ground truth, and converting visible formatting into reasonable Markdown.

Rules:
- Compare the extracted text to the attached page image and fix OCR mistakes, broken words, glued words, and obvious spelling errors.
- Prefer the image when the extracted text conflicts with what is visibly printed.
- Infer Markdown structure from visual formatting on the page:
  - Larger / heavier title text → # or ## headings (use hierarchy by relative size)
  - Section headers → ## or ###
  - Bold or strongly emphasized words → **bold**
  - Italicized words → *italic*
  - Bulleted or dashed lists → Markdown lists
  - Numbered lists → ordered lists
  - Simple tables only when clearly tabular on the page
- Keep game terms, die codes (for example 3D+1), names, URLs, and numbers unchanged when they look intentional.
- Do not wrap the whole page in a fenced code block.
- Do not add commentary, invented sections, or explanations.
- Do not invent missing content; if text is truncated or unclear, leave it alone.
- Output only the corrected Markdown for this page.`
)

var (
	mdBoldRE        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdLinkRE        = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdHeading       = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	modelPreambleRE = regexp.MustCompile(`(?is)^(?:here is (?:the )?(?:merged|corrected|formatted).*?:\s*)+`)
	fenceLineRE     = regexp.MustCompile("(?m)^\\s*`{3}\\w*\\s*$")
	fencedBlockRE   = regexp.MustCompile("(?s)`{3}\\w*\\n(.*?)`{3}")
)

func (o Options) cleanupDPI() int {
	if o.CleanupDPI > 0 {
		return o.CleanupDPI
	}
	return DefaultCleanupDPI
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
	system := cleanupPlainSystemPrompt
	prompt := "Here is the extracted text for this page image. Correct it using the image as ground truth:\n\n" + text
	if opts.CleanupMarkdown {
		system = cleanupMarkdownSystemPrompt
		prompt = "Here is the extracted text for this page image. Correct it and format it as Markdown using the image as ground truth for wording and visual structure:\n\n" + text
	}

	cleaned, err := ollamaGenerate(
		opts,
		opts.ollamaModel(),
		system,
		prompt,
		[]string{base64.StdEncoding.EncodeToString(imagePNG)},
		0.1,
	)
	if err != nil {
		return "", err
	}
	cleaned = stripCleanupWrapper(cleaned)
	if !opts.CleanupMarkdown {
		cleaned = stripMarkdownArtifacts(cleaned)
	}
	if strings.TrimSpace(cleaned) == "" {
		return text, nil
	}
	return cleaned, nil
}

func stripCleanupWrapper(s string) string {
	return stripModelOutput(s)
}

func stripModelOutput(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	s = modelPreambleRE.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)

	if strings.Contains(s, "```") {
		if inner := extractFencedContent(s); inner != "" {
			s = inner
		} else if strings.HasPrefix(s, "```") {
			s = strings.TrimPrefix(s, "```")
			if nl := strings.IndexByte(s, '\n'); nl >= 0 {
				s = s[nl+1:]
			}
			if idx := strings.LastIndex(s, "```"); idx >= 0 {
				s = s[:idx]
			}
			s = strings.TrimSpace(s)
		}
	}

	s = fenceLineRE.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func extractFencedContent(s string) string {
	matches := fencedBlockRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return ""
	}
	best := ""
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		inner := strings.TrimSpace(m[1])
		if len(inner) > len(best) {
			best = inner
		}
	}
	return best
}

func stripMarkdownArtifacts(s string) string {
	s = mdLinkRE.ReplaceAllString(s, "$1")
	s = mdBoldRE.ReplaceAllString(s, "$1")
	s = mdHeading.ReplaceAllString(s, "")
	return s
}

func reportCleanupRuntime(elapsed time.Duration, pages int) {
	minPerPage := 0.0
	if pages > 0 {
		minPerPage = elapsed.Minutes() / float64(pages)
	}
	fmt.Fprintf(os.Stderr, "total runtime %s; %.2f min/page\n", formatDuration(elapsed), minPerPage)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	d -= time.Duration(h) * time.Hour
	m := int(d / time.Minute)
	d -= time.Duration(m) * time.Minute
	s := int(d / time.Second)
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
