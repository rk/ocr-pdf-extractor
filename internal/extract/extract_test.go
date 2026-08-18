package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestStripCleanupWrapper(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain text", "plain text"},
		{"```\nhello\n```", "hello"},
		{"```text\nhello\nworld\n```", "hello\nworld"},
		{"Here is the corrected text:\n\n```markdown\nhello\n```", "hello"},
	}
	for _, tc := range tests {
		got := stripCleanupWrapper(tc.in)
		if got != tc.want {
			t.Errorf("stripCleanupWrapper(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripMarkdownArtifacts(t *testing.T) {
	in := "**Character Creation** see [westendgames.com](http://westendgames.com)\n## Heading\nbody"
	want := "Character Creation see westendgames.com\nHeading\nbody"
	got := stripMarkdownArtifacts(in)
	if got != want {
		t.Fatalf("stripMarkdownArtifacts() = %q, want %q", got, want)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{1500 * time.Millisecond, "2s"},
		{45 * time.Second, "45s"},
		{2*time.Minute + 5*time.Second, "2m 5s"},
		{1*time.Hour + 2*time.Minute + 3*time.Second, "1h 2m 3s"},
	}
	for _, tc := range tests {
		got := formatDuration(tc.in)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCleanupEnabled(t *testing.T) {
	if (Options{}).cleanupEnabled() {
		t.Fatal("expected cleanup disabled by default")
	}
	if !(Options{Cleanup: true}).cleanupEnabled() {
		t.Fatal("expected Cleanup to enable cleanup")
	}
	if !(Options{CleanupMarkdown: true}).cleanupEnabled() {
		t.Fatal("expected CleanupMarkdown to enable cleanup")
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
