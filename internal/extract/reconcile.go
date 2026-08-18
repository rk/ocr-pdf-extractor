package extract

import (
	"strings"
)

const reconcileSystemPrompt = `You merge two OCR passes of the same PDF page into one document.

Rules:
- The TEXT OCR pass is authoritative for narrative wording, reading order, and paragraphs.
- The TABLE OCR pass may contain structured tables (often as Markdown pipe tables) that are missing or broken in the text pass.
- Insert only genuine tables from the table pass that are not already adequately represented in the text pass.
- If the text pass already contains all page content adequately, return it unchanged with minimal edits.
- Ignore prose that was incorrectly wrapped in HTML tables or giant single-cell table blobs.
- Prefer Markdown pipe tables for inserted tables.
- Do not invent missing content.
- Do not add commentary, preamble, or explanations.
- Never wrap output in fenced code blocks (no triple backticks).
- Do not add empty Markdown anchor links like [](#section).
- Output only the merged page content.`

func reconcilePage(textOCR, tableOCR string, opts Options) (string, error) {
	textOCR = strings.TrimSpace(textOCR)
	tableOCR = strings.TrimSpace(tableOCR)

	if tableOCR == "" || !needsReconcile(textOCR, tableOCR) {
		return textOCR, nil
	}
	if textOCR == "" {
		return tableOCR, nil
	}

	prompt := "Merge the following two OCR passes of the same page.\n\n" +
		"--- TEXT OCR (authoritative) ---\n" + textOCR + "\n\n" +
		"--- TABLE OCR (tables only; ignore prose-in-table) ---\n" + tableOCR

	merged, err := ollamaGenerate(opts, opts.ollamaModel(), reconcileSystemPrompt, prompt, nil, 0.1)
	if err != nil {
		return "", err
	}
	merged = stripModelOutput(merged)
	if strings.TrimSpace(merged) == "" {
		return textOCR, nil
	}
	return merged, nil
}
