# ocr-pdf-extractor

A golang powered extractor utility. Leverages
poppler-utils pdftoimage and tesseract to
convert an OCR PDF (probably with only images)
into usable text.

Usage: `ocr-pdf-extractor [input file] [output]`

Expected process: pdftoimages -> (each) -> tesseract -> (append to output)