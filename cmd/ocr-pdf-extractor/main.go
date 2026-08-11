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
	maxPages := flag.Int("max-pages", 0, "process only the first N pages (0 means all pages)")

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
	}

	if err := extract.Extract(inputPath, outputPath, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
