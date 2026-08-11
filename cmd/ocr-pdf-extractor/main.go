package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rk/ocr-pdf-extractor/internal/extract"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ocr-pdf-extractor <input.pdf> <output.txt>\n")
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

	if err := extract.Extract(inputPath, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
