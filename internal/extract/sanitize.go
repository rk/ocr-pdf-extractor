package extract

import (
	"regexp"
	"strings"
)

var (
	modelPreambleRE = regexp.MustCompile(`(?is)^(?:here is (?:the )?(?:merged|corrected|formatted).*?:\s*)+`)
	fenceLineRE     = regexp.MustCompile("(?m)^\\s*`{3}\\w*\\s*$")
	fencedBlockRE   = regexp.MustCompile("(?s)`{3}\\w*\\n(.*?)`{3}")
)

func stripModelOutput(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	s = modelPreambleRE.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)

	if strings.Contains(s, "```") {
		if inner := extractFencedContent(s); inner != "" {
			s = inner
		} else {
			s = stripCleanupWrapper(s)
		}
	}

	s = fenceLineRE.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func extractFencedContent(s string) string {
	matches := fencedBlockRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return ""
	}
	best := ""
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		inner := strings.TrimSpace(m[1])
		if len(inner) > len(best) {
			best = inner
		}
	}
	return best
}

func sanitizeGlmOCROutput(s string) string {
	s = stripModelOutput(s)
	if s == "" {
		return s
	}

	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "```" || strings.HasPrefix(trimmed, "```") {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func isDegenerateOCROutput(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return true
	}

	fenceOrEmpty := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "```" || strings.HasPrefix(trimmed, "```") {
			fenceOrEmpty++
		}
	}
	return fenceOrEmpty > 10 || float64(fenceOrEmpty)/float64(len(lines)) > 0.5
}

func tableOCRHasUsefulTables(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if !strings.Contains(s, "|") {
		return false
	}
	rows := 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") && strings.Count(line, "|") >= 3 {
			rows++
		}
	}
	return rows >= 2
}

func needsReconcile(textOCR, tableOCR string) bool {
	if !tableOCRHasUsefulTables(tableOCR) {
		return false
	}
	if isDegenerateOCROutput(textOCR) {
		return false
	}
	if tableContentMostlyInText(textOCR, tableOCR) {
		return false
	}
	return true
}

func tableContentMostlyInText(text, table string) bool {
	textLower := strings.ToLower(text)
	matched := 0
	total := 0
	for _, line := range strings.Split(table, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") {
			continue
		}
		cells := splitPipeCells(line)
		if isTableHeaderRow(cells) {
			continue
		}
		for _, cell := range cells {
			cell = strings.TrimSpace(cell)
			if cell == "" {
				continue
			}
			total++
			if strings.Contains(textLower, strings.ToLower(cell)) {
				matched++
			}
		}
	}
	if total == 0 {
		return false
	}
	return float64(matched)/float64(total) >= 0.8
}

func isTableHeaderRow(cells []string) bool {
	if len(cells) == 0 {
		return true
	}
	headerish := 0
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		if len(cell) <= 20 && !strings.ContainsAny(cell, "0123456789") {
			headerish++
		}
	}
	return headerish == len(cells)
}

func splitPipeCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	return strings.Split(line, "|")
}
