package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rk/ocr-pdf-extractor/internal/extract"
)

func main() {
	forceOCR := flag.Bool("force-ocr", false, "skip pdftotext and always use pdfimages+tesseract (slow path)")
	lang := flag.String("lang", "eng", "Tesseract language code")
	minChars := flag.Int("min-chars-per-page", extract.DefaultMinCharsPerPage, "minimum trimmed characters for pdftotext fast path per page")
	maxPages := flag.Int("max-pages", 0, "process only N pages starting at -first-page (0 means through end)")
	firstPage := flag.Int("first-page", 1, "first page to process (1-based)")
	layout := flag.Bool("layout", false, "preserve physical multi-column layout (pdftotext -layout); default is reading order")
	cleanup := flag.Bool("cleanup", false, "per-page OCR/spelling cleanup via Ollama using page image + text")
	cleanupMarkdown := flag.Bool("cleanup-markdown", false, "with cleanup, emit Markdown inferred from visual formatting (implies -cleanup)")
	ollamaURL := flag.String("ollama-url", extract.DefaultOllamaURL, "Ollama base URL")
	ollamaModel := flag.String("ollama-model", extract.DefaultOllamaModel, "Ollama model for -cleanup (vision-capable recommended)")
	cleanupDPI := flag.Int("cleanup-dpi", extract.DefaultCleanupDPI, "page render DPI for -cleanup vision images")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ocr-pdf-extractor [options] <input.pdf> <output.txt>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := args[0]
	outputPath := args[1]

	if err := extract.CheckDependencies(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	opts := extract.Options{
		ForceOCR:        *forceOCR,
		Lang:            *lang,
		MinCharsPerPage: *minChars,
		MaxPages:        *maxPages,
		FirstPage:       *firstPage,
		Layout:          *layout,
		Cleanup:         *cleanup,
		CleanupMarkdown: *cleanupMarkdown,
		OllamaURL:       *ollamaURL,
		OllamaModel:     *ollamaModel,
		CleanupDPI:      *cleanupDPI,
	}

	if err := extract.Extract(inputPath, outputPath, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
