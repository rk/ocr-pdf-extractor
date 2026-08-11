package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSubstantialImageCount(t *testing.T) {
	sample := `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
   1     0 image    1224  1584  rgb     3   8  jpeg   no       739  0   144   144  168K 2.9%
   1     1 stencil     1     1  -       1   1  image  no   [inline]   0.136 0.246    0B   -
   3     7 image      528    25  sep     1   8  image  no       221  0    72    72  488B 3.7%
`

	got := parseSubstantialImageCount(sample)
	if got != 1 {
		t.Fatalf("parseSubstantialImageCount() = %d, want 1 (1224x1584 image only)", got)
	}
}

func TestNaturalLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"img-001-000.jpg", "img-001-001.png", true},
		{"img-001-001.png", "img-002-000.jpg", true},
		{"img-010-000.jpg", "img-002-000.jpg", false},
	}

	for _, tc := range tests {
		got := naturalLess(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestValidatePathsRejectsSameFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := validatePaths(path, path); err == nil {
		t.Fatal("validatePaths() expected error for identical input/output")
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}
	if opts.minChars() != DefaultMinCharsPerPage {
		t.Fatalf("minChars() = %d, want %d", opts.minChars(), DefaultMinCharsPerPage)
	}
	if opts.lang() != "eng" {
		t.Fatalf("lang() = %q, want eng", opts.lang())
	}
}

func TestPageNeedsOCRDecision(t *testing.T) {
	tests := []struct {
		name    string
		images  bool
		text    string
		min     int
		wantOCR bool
	}{
		{"no images", false, "short", 50, false},
		{"images with enough text", true, strings.Repeat("a", 60), 50, false},
		{"images with insufficient text", true, "boilerplate only", 50, true},
		{"images with whitespace padding below threshold", true, "   x   ", 50, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pageNeedsOCRDecision(tc.images, tc.text, tc.min)
			if got != tc.wantOCR {
				t.Fatalf("pageNeedsOCRDecision() = %v, want %v", got, tc.wantOCR)
			}
		})
	}
}

func TestOpenOutputValidatesWritable(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "nested", "out.txt")

	w, err := openOutput(outPath)
	if err != nil {
		t.Fatalf("openOutput() error: %v", err)
	}
	if err := w.writePage("hello"); err != nil {
		t.Fatalf("writePage() error: %v", err)
	}
	if err := w.close(); err != nil {
		t.Fatalf("close() error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("output = %q, want hello", string(data))
	}
}
