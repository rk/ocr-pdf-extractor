package extract

import (
	"fmt"
	"os"
	"path/filepath"
)

type outputWriter struct {
	tmp       *os.File
	finalPath string
	hasPages  bool
}

func openOutput(outputPath string) (*outputWriter, error) {
	dir := filepath.Dir(outputPath)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".ocr-pdf-extractor-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("output path is not writable: %w", err)
	}

	return &outputWriter{
		tmp:       tmp,
		finalPath: outputPath,
	}, nil
}

func (w *outputWriter) writePage(text string) error {
	if w.hasPages {
		if _, err := w.tmp.WriteString("\n\n"); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}
	if _, err := w.tmp.WriteString(text); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	w.hasPages = true
	return nil
}

func (w *outputWriter) writeAll(text string) error {
	if text == "" {
		return nil
	}
	if _, err := w.tmp.WriteString(text); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	w.hasPages = true
	return nil
}

func (w *outputWriter) close() error {
	tmpPath := w.tmp.Name()
	if err := w.tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing output: %w", err)
	}

	if err := os.Rename(tmpPath, w.finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing output: %w", err)
	}
	return nil
}

func (w *outputWriter) abort() {
	_ = w.tmp.Close()
	_ = os.Remove(w.tmp.Name())
}
