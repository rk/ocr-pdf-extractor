package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultMinCharsPerPage = 50
)

var pageCountRE = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

// Options configures the extraction pipeline.
type Options struct {
	// ForceOCR skips pdftotext and always uses pdfimages + tesseract.
	ForceOCR bool
	// Lang is the Tesseract language code (default "eng").
	Lang string
	// MinCharsPerPage is the minimum trimmed character count for pdftotext fast path.
	MinCharsPerPage int
	// MaxPages limits processing to N pages starting at FirstPage (0 means through end).
	MaxPages int
	// FirstPage is the first page to process (1-based, default 1).
	FirstPage int
	// Layout passes -layout to pdftotext, preserving physical multi-column layout.
	// When false (default), pdftotext uses reading order (typically one column at a time).
	Layout bool
	// Cleanup runs each page through Ollama with the page image + extracted text.
	Cleanup bool
	// OllamaURL is the Ollama base URL (default http://127.0.0.1:11434).
	OllamaURL string
	// OllamaModel is the Ollama model name (default ministral-3:latest).
	OllamaModel string
	// CleanupDPI is the pdftoppm resolution used for cleanup page images.
	CleanupDPI int
}

func (o Options) minChars() int {
	if o.MinCharsPerPage > 0 {
		return o.MinCharsPerPage
	}
	return DefaultMinCharsPerPage
}

func (o Options) lang() string {
	if o.Lang != "" {
		return o.Lang
	}
	return "eng"
}

// Extract runs the full PDF text extraction pipeline.
func Extract(inputPath, outputPath string, opts Options) error {
	if err := validatePaths(inputPath, outputPath); err != nil {
		return err
	}

	out, err := openOutput(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			out.abort()
		}
	}()

	pageCount, err := pageCount(inputPath)
	if err != nil {
		return fmt.Errorf("reading page count: %w", err)
	}

	lastPage := pageCount
	firstPage := 1
	if opts.FirstPage > 1 {
		firstPage = opts.FirstPage
	}
	if firstPage > pageCount {
		return fmt.Errorf("first page %d is beyond document length (%d)", firstPage, pageCount)
	}
	if opts.MaxPages > 0 {
		lastPage = firstPage + opts.MaxPages - 1
		if lastPage > pageCount {
			lastPage = pageCount
		}
	}

	if opts.Cleanup {
		if err := checkOllama(opts); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "cleanup enabled via ollama vision (%s)\n", opts.ollamaModel())
	}

	tmpDir, err := os.MkdirTemp("", "ocr-pdf-extractor-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Vision cleanup needs each page image, so skip the whole-document fast path.
	useWholeDoc := !opts.ForceOCR && !opts.Cleanup && firstPage == 1
	if useWholeDoc {
		text, usedFastPath, err := tryWholeDocumentText(inputPath, lastPage, opts)
		if err != nil {
			return err
		}
		if usedFastPath {
			fmt.Fprintf(os.Stderr, "using pdftotext (whole document)\n")
			if err := out.writeAll(text); err != nil {
				return err
			}
			return out.close()
		}
	} else if opts.ForceOCR {
		fmt.Fprintf(os.Stderr, "force-ocr: skipping pdftotext fast path\n")
	}

	for page := firstPage; page <= lastPage; page++ {
		fmt.Fprintf(os.Stderr, "page %d/%d\n", page, lastPage)

		pageText, err := extractPage(inputPath, tmpDir, page, opts)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}

		if opts.Cleanup {
			fmt.Fprintf(os.Stderr, "  cleanup via ollama (image+text)\n")
			pageText, err = cleanupPage(inputPath, tmpDir, pageText, page, opts)
			if err != nil {
				return fmt.Errorf("page %d cleanup: %w", page, err)
			}
		}

		if err := out.writePage(pageText); err != nil {
			return err
		}
	}

	return out.close()
}

func pageCount(inputPath string) (int, error) {
	out, err := exec.Command("pdfinfo", inputPath).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w: %s", err, strings.TrimSpace(string(out)))
	}

	match := pageCountRE.FindSubmatch(out)
	if match == nil {
		return 0, fmt.Errorf("pdfinfo: could not parse page count from output")
	}

	count, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: invalid page count: %w", err)
	}
	if count < 1 {
		return 0, fmt.Errorf("pdfinfo: document has no pages")
	}
	return count, nil
}

func tryWholeDocumentText(inputPath string, pageCount int, opts Options) (string, bool, error) {
	minChars := opts.minChars()
	text, err := pdftotext(inputPath, 0, 0, opts.Layout)
	if err != nil {
		return "", false, err
	}

	if len(strings.TrimSpace(text)) < minChars*pageCount {
		return "", false, nil
	}

	for page := 1; page <= pageCount; page++ {
		needs, err := pageNeedsOCR(inputPath, page, opts)
		if err != nil {
			return "", false, err
		}
		if needs {
			return "", false, nil
		}
	}

	return text, true, nil
}

func pageNeedsOCR(inputPath string, page int, opts Options) (bool, error) {
	images, err := substantialImageCount(inputPath, page, page)
	if err != nil {
		return false, err
	}
	if images == 0 {
		return false, nil
	}

	pageText, err := pdftotext(inputPath, page, page, opts.Layout)
	if err != nil {
		return false, err
	}
	return pageNeedsOCRDecision(true, pageText, opts.minChars()), nil
}

func pageNeedsOCRDecision(hasSubstantialImages bool, pageText string, minChars int) bool {
	if !hasSubstantialImages {
		return false
	}
	return len(strings.TrimSpace(pageText)) < minChars
}

func extractPage(inputPath, tmpDir string, page int, opts Options) (string, error) {
	if !opts.ForceOCR {
		text, err := pdftotext(inputPath, page, page, opts.Layout)
		if err != nil {
			return "", err
		}
		if len(strings.TrimSpace(text)) >= opts.minChars() {
			fmt.Fprintf(os.Stderr, "  using pdftotext\n")
			return text, nil
		}
	}

	fmt.Fprintf(os.Stderr, "  using pdfimages+tesseract\n")
	return ocrPage(inputPath, tmpDir, page, opts)
}

func pdftotext(inputPath string, firstPage, lastPage int, layout bool) (string, error) {
	var args []string
	if layout {
		args = append(args, "-layout")
	}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	args = append(args, inputPath, "-")

	cmd := exec.Command("pdftotext", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(out), nil
}

func ocrPage(inputPath, tmpDir string, page int, opts Options) (string, error) {
	pageDir := filepath.Join(tmpDir, fmt.Sprintf("page-%d", page))
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return "", fmt.Errorf("creating page temp dir: %w", err)
	}

	prefix := filepath.Join(pageDir, "img")
	cmd := exec.Command(
		"pdfimages",
		"-all", "-p",
		"-f", strconv.Itoa(page),
		"-l", strconv.Itoa(page),
		inputPath, prefix,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdfimages: %w: %s", err, strings.TrimSpace(string(out)))
	}

	images, err := listImages(pageDir)
	if err != nil {
		return "", err
	}
	if len(images) == 0 {
		return "", fmt.Errorf("no extractable images on page %d for OCR", page)
	}

	var parts []string
	for _, image := range images {
		text, err := tesseract(image, opts.lang())
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func listImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading extracted images: %w", err)
	}

	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".ppm", ".pbm", ".jp2", ".jb2e", ".jb2g":
			images = append(images, filepath.Join(dir, name))
		}
	}

	sort.Slice(images, func(i, j int) bool {
		return naturalLess(filepath.Base(images[i]), filepath.Base(images[j]))
	})
	return images, nil
}

var digitRE = regexp.MustCompile(`(\d+)`)

func naturalLess(a, b string) bool {
	aParts := digitRE.Split(a, -1)
	bParts := digitRE.Split(b, -1)
	aNums := digitRE.FindAllString(a, -1)
	bNums := digitRE.FindAllString(b, -1)

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] != bParts[i] {
			return aParts[i] < bParts[i]
		}
		if i < len(aNums) && i < len(bNums) && aNums[i] != bNums[i] {
			ai, _ := strconv.Atoi(aNums[i])
			bi, _ := strconv.Atoi(bNums[i])
			return ai < bi
		}
	}
	return a < b
}

func tesseract(imagePath, lang string) (string, error) {
	cmd := exec.Command("tesseract", imagePath, "stdout", "-l", lang)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tesseract on %s: %w: %s", filepath.Base(imagePath), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("tesseract on %s: %w", filepath.Base(imagePath), err)
	}
	return string(out), nil
}
