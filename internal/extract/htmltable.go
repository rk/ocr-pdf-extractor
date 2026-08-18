package extract

import (
	"html"
	"regexp"
	"strings"
)

var tableBlockRE = regexp.MustCompile(`(?is)<table\b[^>]*>.*?</table>`)

func normalizeTableOCR(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(strings.ToLower(raw), "<table") {
		return raw
	}

	var parts []string
	last := 0
	for _, loc := range tableBlockRE.FindAllStringIndex(raw, -1) {
		if loc[0] > last {
			if s := strings.TrimSpace(stripHTML(raw[last:loc[0]])); s != "" {
				parts = append(parts, s)
			}
		}
		block := raw[loc[0]:loc[1]]
		if md, ok := htmlTableToMarkdown(block); ok {
			parts = append(parts, md)
		}
		last = loc[1]
	}
	if last < len(raw) {
		if s := strings.TrimSpace(stripHTML(raw[last:])); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

func htmlTableToMarkdown(tableHTML string) (string, bool) {
	if !isGenuineTable(tableHTML) {
		return "", false
	}
	rows := parseHTMLTableRows(tableHTML)
	if len(rows) == 0 {
		return "", false
	}
	return rowsToMarkdownTable(rows), true
}

func isGenuineTable(tableHTML string) bool {
	rows := parseHTMLTableRows(tableHTML)
	if len(rows) < 2 {
		return false
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols < 2 {
		return false
	}
	// Reject prose stuffed into one wide cell per row.
	singleWide := 0
	for _, row := range rows {
		nonEmpty := 0
		longest := 0
		for _, cell := range row {
			cell = strings.TrimSpace(cell)
			if cell == "" {
				continue
			}
			nonEmpty++
			if len(cell) > longest {
				longest = len(cell)
			}
		}
		if nonEmpty <= 1 && longest > 120 {
			singleWide++
		}
	}
	if singleWide > len(rows)/2 {
		return false
	}
	// Typical data tables have relatively short cells in header/first rows.
	shortCells := 0
	totalCells := 0
	for _, row := range rows {
		for _, cell := range row {
			cell = strings.TrimSpace(cell)
			if cell == "" {
				continue
			}
			totalCells++
			if len(cell) <= 80 {
				shortCells++
			}
		}
	}
	if totalCells == 0 {
		return false
	}
	return float64(shortCells)/float64(totalCells) >= 0.35
}

var trRE = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
var cellRE = regexp.MustCompile(`(?is)<t[hd]\b[^>]*>(.*?)</t[hd]>`)
var brRE = regexp.MustCompile(`(?i)<br\s*/?>`)
var tagRE = regexp.MustCompile(`(?s)<[^>]+>`)

func parseHTMLTableRows(tableHTML string) [][]string {
	var rows [][]string
	for _, tr := range trRE.FindAllStringSubmatch(tableHTML, -1) {
		if len(tr) < 2 {
			continue
		}
		var cells []string
		for _, cell := range cellRE.FindAllStringSubmatch(tr[1], -1) {
			if len(cell) < 2 {
				continue
			}
			cells = append(cells, cleanTableCell(cell[1]))
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	return rows
}

func cleanTableCell(s string) string {
	s = brRE.ReplaceAllString(s, " ")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func stripHTML(s string) string {
	s = brRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

func rowsToMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}
	var b strings.Builder
	for i, row := range rows {
		b.WriteString("|")
		for _, cell := range row {
			b.WriteString(" ")
			b.WriteString(strings.ReplaceAll(cell, "|", "\\|"))
			b.WriteString(" |")
		}
		b.WriteString("\n")
		if i == 0 {
			b.WriteString("|")
			for range row {
				b.WriteString(" --- |")
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
