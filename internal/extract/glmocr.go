package extract

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

const (
	DefaultGlmOCRModel = "glm-ocr:latest"
	GlmOCRTextPrompt   = "Text Recognition:"
	GlmOCRTablePrompt  = "Table Recognition:"
)

func glmOCRPage(imagePNG []byte, prompt string, opts Options) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(imagePNG)
	text, err := ollamaGenerate(opts, opts.glmOCRModel(), "", prompt, []string{b64}, 0)
	if err != nil {
		return "", err
	}
	return sanitizeGlmOCROutput(text), nil
}

func extractWithGlmOCR(inputPath, outputPath string, opts Options) error {
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

	firstPage := 1
	if opts.FirstPage > 1 {
		firstPage = opts.FirstPage
	}
	if firstPage > pageCount {
		return fmt.Errorf("first page %d is beyond document length (%d)", firstPage, pageCount)
	}
	lastPage := pageCount
	if opts.MaxPages > 0 {
		lastPage = firstPage + opts.MaxPages - 1
		if lastPage > pageCount {
			lastPage = pageCount
		}
	}

	if err := checkOllamaModels(opts, opts.glmOCRModel(), opts.ollamaModel()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "glm-ocr enabled (%s + %s)\n", opts.glmOCRModel(), opts.ollamaModel())

	scratchDir := scratchPath(outputPath)
	if err := ensureScratchDir(scratchDir); err != nil {
		return fmt.Errorf("creating scratch dir: %w", err)
	}
	cleanupScratch := !opts.KeepScratch
	if cleanupScratch {
		defer func() {
			if err == nil {
				_ = removeScratchDir(scratchDir)
			}
		}()
	}

	renderDir, err := os.MkdirTemp("", "ocr-pdf-extractor-render-*")
	if err != nil {
		return fmt.Errorf("creating render temp dir: %w", err)
	}
	defer os.RemoveAll(renderDir)

	started := time.Now()
	for page := firstPage; page <= lastPage; page++ {
		fmt.Fprintf(os.Stderr, "page %d/%d\n", page, lastPage)

		fmt.Fprintf(os.Stderr, "  render\n")
		pngPath, err := renderPagePNG(inputPath, renderDir, page, opts.cleanupDPI())
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}
		img, err := os.ReadFile(pngPath)
		if err != nil {
			return fmt.Errorf("page %d: reading render: %w", page, err)
		}
		_ = os.Remove(pngPath)

		fmt.Fprintf(os.Stderr, "  glm-ocr text\n")
		textOCR, err := glmOCRPage(img, GlmOCRTextPrompt, opts)
		if err != nil {
			return fmt.Errorf("page %d text ocr: %w", page, err)
		}
		if err := writeScratchFile(scratchPageTextPath(scratchDir, page), textOCR); err != nil {
			return fmt.Errorf("page %d: writing scratch text: %w", page, err)
		}

		var tableOCR string
		if isDegenerateOCROutput(textOCR) {
			fmt.Fprintf(os.Stderr, "  glm-ocr text low quality; skipping table pass and reconcile (try -cleanup-dpi 200)\n")
		} else {
			fmt.Fprintf(os.Stderr, "  glm-ocr table\n")
			tableRaw, err := glmOCRPage(img, GlmOCRTablePrompt, opts)
			if err != nil {
				return fmt.Errorf("page %d table ocr: %w", page, err)
			}
			tableOCR = normalizeTableOCR(tableRaw)
			if err := writeScratchFile(scratchPageTablePath(scratchDir, page), tableOCR); err != nil {
				return fmt.Errorf("page %d: writing scratch table: %w", page, err)
			}
		}

		var merged string
		if needsReconcile(textOCR, tableOCR) {
			fmt.Fprintf(os.Stderr, "  reconcile\n")
			merged, err = reconcilePage(textOCR, tableOCR, opts)
			if err != nil {
				return fmt.Errorf("page %d reconcile: %w", page, err)
			}
		} else {
			merged = textOCR
		}
		if err := writeScratchFile(scratchPageMergedPath(scratchDir, page), merged); err != nil {
			return fmt.Errorf("page %d: writing scratch merged: %w", page, err)
		}

		if err := out.writePage(merged); err != nil {
			return err
		}
	}

	if err := out.close(); err != nil {
		return err
	}
	reportCleanupRuntime(time.Since(started), lastPage-firstPage+1)
	return nil
}
