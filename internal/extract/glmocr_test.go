package extract

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScratchPath(t *testing.T) {
	got := scratchPath("out/book.md")
	want := filepath.Join("out", ".book.md-scratch")
	if got != want {
		t.Fatalf("scratchPath() = %q, want %q", got, want)
	}
}

func TestPageScratchStem(t *testing.T) {
	if got := pageScratchStem(10); got != "page-010" {
		t.Fatalf("pageScratchStem(10) = %q, want page-010", got)
	}
}

func TestOllamaOptionDefaults(t *testing.T) {
	opts := Options{}
	if opts.ollamaURL() != DefaultOllamaURL {
		t.Fatalf("ollamaURL() = %q, want %q", opts.ollamaURL(), DefaultOllamaURL)
	}
	if opts.ollamaModel() != DefaultOllamaModel {
		t.Fatalf("ollamaModel() = %q, want %q", opts.ollamaModel(), DefaultOllamaModel)
	}
	if opts.glmOCRModel() != DefaultGlmOCRModel {
		t.Fatalf("glmOCRModel() = %q, want %q", opts.glmOCRModel(), DefaultGlmOCRModel)
	}
}

func TestHTMLTableToMarkdown(t *testing.T) {
	html := `<table>
<tr><th>Level</th><th>Number</th></tr>
<tr><td>Easy</td><td>3</td></tr>
<tr><td>Difficult</td><td>15</td></tr>
</table>`
	md, ok := htmlTableToMarkdown(html)
	if !ok {
		t.Fatal("htmlTableToMarkdown() ok = false, want true")
	}
	if !strings.Contains(md, "| Level | Number |") {
		t.Fatalf("markdown missing header row: %q", md)
	}
	if !strings.Contains(md, "| Easy | 3 |") {
		t.Fatalf("markdown missing data row: %q", md)
	}
}

func TestIsGenuineTableRejectsProseBlob(t *testing.T) {
	prose := `<table><tr><td>` + strings.Repeat("This is a long paragraph of narrative text that was incorrectly wrapped inside a single table cell by the table recognition model. ", 5) + `</td></tr></table>`
	if isGenuineTable(prose) {
		t.Fatal("isGenuineTable() = true for single-column prose blob, want false")
	}
}

func TestNormalizeTableOCRKeepsRealTables(t *testing.T) {
	raw := `<table><tr><th>Level</th><th>Number</th></tr><tr><td>Easy</td><td>3</td></tr></table>`
	got := normalizeTableOCR(raw)
	if !strings.Contains(got, "| Level | Number |") {
		t.Fatalf("normalizeTableOCR() = %q, want markdown pipe table", got)
	}
}

func TestNormalizeTableOCRSkipsProseTable(t *testing.T) {
	raw := `<table><tr><td>` + strings.Repeat("Narrative prose stuffed into one giant table cell. ", 20) + `</td></tr></table>`
	got := normalizeTableOCR(raw)
	if strings.Contains(got, "|") {
		t.Fatalf("normalizeTableOCR() should not emit pipe table for prose blob, got %q", got)
	}
}
