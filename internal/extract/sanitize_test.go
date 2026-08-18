package extract

import (
	"strings"
	"testing"
)

func TestStripModelOutputRemovesPreambleAndFence(t *testing.T) {
	in := "Here is the merged Markdown document for this page:\n\n```markdown\n# Title\n\nBody text.\n```"
	want := "# Title\n\nBody text."
	got := stripModelOutput(in)
	if got != want {
		t.Fatalf("stripModelOutput() = %q, want %q", got, want)
	}
}

func TestSanitizeGlmOCROutputStripsFenceSpam(t *testing.T) {
	in := "Space\nWEO\nDIEN\n```markdown\n\nSpace\nWEO\n```\n```\n```\n```\n"
	got := sanitizeGlmOCROutput(in)
	if strings.Contains(got, "```") {
		t.Fatalf("sanitizeGlmOCROutput() still contains fences: %q", got)
	}
	if !strings.Contains(got, "Space") {
		t.Fatalf("sanitizeGlmOCROutput() = %q, want salvageable text", got)
	}
}

func TestIsDegenerateOCROutput(t *testing.T) {
	good := "Chapter 1\n\nThis is normal body text for a page."
	if isDegenerateOCROutput(good) {
		t.Fatal("expected normal text not to be degenerate")
	}

	bad := strings.Repeat("```\n", 50) + "Space\nWEO"
	if !isDegenerateOCROutput(bad) {
		t.Fatal("expected fence spam to be degenerate")
	}
}

func TestNeedsReconcileSkipsRedundantTOC(t *testing.T) {
	text := "> Contents\n\nIntroduction 3\nIntroductory Adventure 4\nKey Terms 8\n"
	table := "| Chapter | Title |\n| --- | --- |\n| Introduction | 3 |\n| Introductory Adventure | 4 |\n| Key Terms | 8 |\n"
	if needsReconcile(text, table) {
		t.Fatal("needsReconcile() = true for redundant TOC table, want false")
	}
}

func TestNeedsReconcileKeepsMissingTable(t *testing.T) {
	text := "Generic Standard Difficulties\n\nEasy tasks use a difficulty of 3."
	table := "| Level | Number |\n| --- | --- |\n| Easy | 3 |\n| Difficult | 15 |\n"
	if !needsReconcile(text, table) {
		t.Fatal("needsReconcile() = false when table adds missing rows, want true")
	}
}
