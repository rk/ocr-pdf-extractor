# ocr-pdf-extractor

A Go-powered CLI that extracts text from PDFs by wrapping poppler-utils and Tesseract.

## Pipeline

```
pdftopages → pdftotext (fast) | pdfimages → tesseract (OCR)
           → optional Ollama vision cleanup (page image + text)
           → optional GLM-OCR dual pass + Ministral merge (-glm-ocr)
```

1. Try **pdftotext** on the whole document when every page with substantial embedded images also has enough extractable text (fast path for text-native PDFs; skipped when `-cleanup` or `-glm-ocr` is on).
2. Otherwise, for each page:
   - Try **pdftotext** for that page.
   - If insufficient text, extract embedded images with **pdfimages** and run **tesseract** OCR on each.
3. Optionally (`-cleanup`), render the page with **pdftoppm** and send **page image + extracted text** to **Ollama** so a vision model can correct OCR/spelling against the page. Add `-cleanup-markdown` to also infer headings and emphasis from on-page visual formatting.
4. Optionally (`-glm-ocr`), for each page: render with **pdftoppm**, run **GLM-OCR** twice via Ollama (`Text Recognition:` + `Table Recognition:`), normalize table HTML to Markdown, write scratch artifacts under `.[output]-scratch/`, merge with **Ministral text-only** (text pass is authoritative; real tables from the table pass are inserted), then concatenate merged pages into the output file.

`pdftopages` is the per-page orchestration loop implemented in Go (not a separate binary).

## Dependencies

System tools (must be on `PATH`):

```bash
sudo apt install poppler-utils tesseract-ocr   # Debian/Ubuntu
brew install poppler tesseract                   # macOS
```

- `pdfinfo`, `pdftotext`, `pdfimages`, `pdftoppm` — poppler-utils
- `tesseract` — tesseract-ocr

Optional cleanup (`-cleanup`):

- [Ollama](https://ollama.com/) running locally
- A **vision-capable** chat model (default: `ministral-3:latest`)

Optional GLM-OCR merge (`-glm-ocr`):

- [Ollama](https://ollama.com/) running locally
- `glm-ocr:latest` for page OCR (text + table passes)
- `ministral-3:latest` (or `-ollama-model`) for text-only merge

Go 1.26 or later.

## Build

```bash
go build -o ocr-pdf-extractor ./cmd/ocr-pdf-extractor
```

## Usage

```bash
ocr-pdf-extractor [options] <input.pdf> <output.txt>
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-force-ocr` | `false` | Skip `pdftotext` and always use `pdfimages` + `tesseract` (slow path) |
| `-lang` | `eng` | Tesseract language code |
| `-min-chars-per-page` | `50` | Minimum trimmed characters for `pdftotext` fast path per page |
| `-first-page` | `1` | First page to process (1-based) |
| `-max-pages` | `0` (through end) | Process only N pages starting at `-first-page` |
| `-layout` | `false` | Preserve physical multi-column layout (`pdftotext -layout`). Default is reading order (one column at a time), which is usually better for body text. |
| `-cleanup` | `false` | Per-page OCR/spelling cleanup via Ollama using page image + text (slow) |
| `-cleanup-markdown` | `false` | With cleanup, emit Markdown inferred from visual formatting (headings by size, bold/italic, lists). Implies `-cleanup`. |
| `-cleanup-dpi` | `120` | `pdftoppm` DPI for cleanup / `-glm-ocr` page renders |
| `-ollama-url` | `http://127.0.0.1:11434` | Ollama base URL |
| `-ollama-model` | `ministral-3:latest` | Ollama model used by `-cleanup` and `-glm-ocr` merge |
| `-glm-ocr` | `false` | Dual GLM-OCR passes (text + table) per page, merged via Ministral text-only |
| `-glm-ocr-model` | `glm-ocr:latest` | Ollama model for `-glm-ocr` page recognition |
| `-keep-scratch` | `false` | Keep `.[output]-scratch` after a successful `-glm-ocr` run |

The whole-document fast path requires at least `-min-chars-per-page` trimmed characters per page on average, and rejects documents where any page has substantial embedded images but insufficient `pdftotext` output (to avoid returning boilerplate-only text from image-heavy pages).

### Example

Download a sample PDF and extract text:

```bash
curl -L -o /tmp/D6_Space_Opera.pdf \
  "https://ogc.rpglibrary.org/images/4/47/D6_Space_Opera.pdf"

./ocr-pdf-extractor /tmp/D6_Space_Opera.pdf /tmp/d6-space-opera.txt
./ocr-pdf-extractor -force-ocr /tmp/D6_Space_Opera.pdf /tmp/d6-space-opera-ocr.txt
./ocr-pdf-extractor -cleanup -first-page 5 -max-pages 6 /tmp/D6_Space_Opera.pdf /tmp/d6-space-opera-clean.txt
./ocr-pdf-extractor -cleanup-markdown -first-page 9 -max-pages 1 /tmp/D6_Space_Opera.pdf /tmp/d6-space-opera-clean.md
./ocr-pdf-extractor -glm-ocr -first-page 5 -max-pages 6 D6_Space_v2.0_weg51012OGL.pdf ./OpenD6-Space.md
```

Or run the smoke test script:

```bash
./scripts/test-sample.sh
```

Progress is written to stderr; extracted text is written to the output file.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Usage / validation error |
| 2 | Missing external tools |
| 3 | Processing error |
