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
	minCharsPerPage     = 50
	wholeDocCharFactor  = 50
)

var pageCountRE = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

// Extract runs the full PDF text extraction pipeline.
func Extract(inputPath, outputPath string) error {
	if err := validateInput(inputPath); err != nil {
		return err
	}

	pageCount, err := pageCount(inputPath)
	if err != nil {
		return fmt.Errorf("reading page count: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ocr-pdf-extractor-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	text, usedFastPath, err := tryWholeDocumentText(inputPath, pageCount)
	if err != nil {
		return err
	}
	if usedFastPath {
		fmt.Fprintf(os.Stderr, "using pdftotext (whole document)\n")
		return writeOutput(outputPath, text)
	}

	var builder strings.Builder
	for page := 1; page <= pageCount; page++ {
		fmt.Fprintf(os.Stderr, "page %d/%d\n", page, pageCount)

		pageText, err := extractPage(inputPath, tmpDir, page)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}

		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(pageText)
	}

	return writeOutput(outputPath, builder.String())
}

func validateInput(inputPath string) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", inputPath)
		}
		return fmt.Errorf("input file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("input path is a directory: %s", inputPath)
	}
	return nil
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

func tryWholeDocumentText(inputPath string, pageCount int) (string, bool, error) {
	text, err := pdftotext(inputPath, 0, 0)
	if err != nil {
		return "", false, err
	}

	threshold := wholeDocCharFactor * pageCount
	if len(strings.TrimSpace(text)) >= threshold {
		return text, true, nil
	}
	return "", false, nil
}

func extractPage(inputPath, tmpDir string, page int) (string, error) {
	text, err := pdftotext(inputPath, page, page)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(text)) >= minCharsPerPage {
		fmt.Fprintf(os.Stderr, "  using pdftotext\n")
		return text, nil
	}

	fmt.Fprintf(os.Stderr, "  using pdfimages+tesseract\n")
	return ocrPage(inputPath, tmpDir, page)
}

func pdftotext(inputPath string, firstPage, lastPage int) (string, error) {
	args := []string{"-layout"}
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

func ocrPage(inputPath, tmpDir string, page int) (string, error) {
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
		fmt.Fprintf(os.Stderr, "  warning: no images extracted from page %d\n", page)
		return "", nil
	}

	var parts []string
	for _, image := range images {
		text, err := tesseract(image)
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

func tesseract(imagePath string) (string, error) {
	cmd := exec.Command("tesseract", imagePath, "stdout", "-l", "eng")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tesseract on %s: %w: %s", filepath.Base(imagePath), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("tesseract on %s: %w", filepath.Base(imagePath), err)
	}
	return string(out), nil
}

func writeOutput(outputPath, text string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	tmpPath := outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing output: %w", err)
	}
	return nil
}
