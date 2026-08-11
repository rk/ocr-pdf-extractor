#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PDF_URL="https://ogc.rpglibrary.org/images/4/47/D6_Space_Opera.pdf"
PDF="/tmp/D6_Space_Opera.pdf"
OUT="/tmp/d6-space-opera.txt"
BIN="$ROOT/ocr-pdf-extractor"

echo "Downloading sample PDF..."
curl -fsSL -o "$PDF" "$PDF_URL"

echo "Building ocr-pdf-extractor..."
(cd "$ROOT" && go build -o "$BIN" ./cmd/ocr-pdf-extractor)

echo "Running extraction (this may take several minutes)..."
"$BIN" "$PDF" "$OUT"

if [[ ! -s "$OUT" ]]; then
  echo "FAIL: output file is empty" >&2
  exit 1
fi

size=$(wc -c < "$OUT")
if [[ "$size" -lt 10240 ]]; then
  echo "FAIL: output too small ($size bytes, expected > 10 KB)" >&2
  exit 1
fi

if ! grep -qi "space" "$OUT"; then
  echo "FAIL: expected keyword 'space' not found in output" >&2
  exit 1
fi

echo "PASS: extracted $(wc -l < "$OUT") lines ($size bytes) to $OUT"
