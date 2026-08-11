#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PDF_URL="https://ogc.rpglibrary.org/images/4/47/D6_Space_Opera.pdf"
PDF="/tmp/D6_Space_Opera.pdf"
OUT="/tmp/d6-space-opera.txt"
OUT_OCR="/tmp/d6-space-opera-ocr.txt"
BIN="$ROOT/ocr-pdf-extractor"

echo "Downloading sample PDF..."
curl -fsSL -o "$PDF" "$PDF_URL"

echo "Building ocr-pdf-extractor..."
(cd "$ROOT" && go build -o "$BIN" ./cmd/ocr-pdf-extractor)

echo "Running fast-path extraction..."
"$BIN" "$PDF" "$OUT"

if [[ ! -s "$OUT" ]]; then
  echo "FAIL: fast-path output file is empty" >&2
  exit 1
fi

size=$(wc -c < "$OUT")
if [[ "$size" -lt 10240 ]]; then
  echo "FAIL: fast-path output too small ($size bytes, expected > 10 KB)" >&2
  exit 1
fi

if ! grep -qi "space" "$OUT"; then
  echo "FAIL: expected keyword 'space' not found in fast-path output" >&2
  exit 1
fi

echo "PASS (fast path): extracted $(wc -l < "$OUT") lines ($size bytes) to $OUT"

echo "Running force-ocr extraction (first 2 pages)..."
"$BIN" -force-ocr -max-pages 2 "$PDF" "$OUT_OCR"

if [[ ! -s "$OUT_OCR" ]]; then
  echo "FAIL: force-ocr output file is empty" >&2
  exit 1
fi

ocr_size=$(wc -c < "$OUT_OCR")
if [[ "$ocr_size" -lt 100 ]]; then
  echo "FAIL: force-ocr output too small ($ocr_size bytes)" >&2
  exit 1
fi

echo "PASS (force-ocr): extracted $(wc -l < "$OUT_OCR") lines ($ocr_size bytes) to $OUT_OCR"
