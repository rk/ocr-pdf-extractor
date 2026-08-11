# ocr-pdf-extractor

A Go-powered CLI that extracts text from PDFs by wrapping poppler-utils and Tesseract.

## Pipeline

```
pdftopages → pdftotext (fast) | pdfimages → tesseract (OCR)
```

1. Try **pdftotext** on the whole document (fast path for text-native PDFs).
2. Otherwise, for each page:
   - Try **pdftotext** for that page.
   - If insufficient text, extract embedded images with **pdfimages** and run **tesseract** OCR on each.

`pdftopages` is the per-page orchestration loop implemented in Go (not a separate binary).

## Dependencies

System tools (must be on `PATH`):

```bash
sudo apt install poppler-utils tesseract-ocr   # Debian/Ubuntu
brew install poppler tesseract                   # macOS
```

- `pdfinfo`, `pdftotext`, `pdfimages` — poppler-utils
- `tesseract` — tesseract-ocr

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

| Flag | Description |
|------|-------------|
| `-force-ocr` | Skip `pdftotext` and always use `pdfimages` + `tesseract` (slow path) |

### Example

Download a sample image-only PDF and extract text:

```bash
curl -L -o /tmp/D6_Space_Opera.pdf \
  "https://ogc.rpglibrary.org/images/4/47/D6_Space_Opera.pdf"

./ocr-pdf-extractor /tmp/D6_Space_Opera.pdf /tmp/d6-space-opera.txt
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
