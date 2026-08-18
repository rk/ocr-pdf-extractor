package extract

import (
	"fmt"
	"os"
	"path/filepath"
)

func scratchPath(outputPath string) string {
	dir := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	if dir == "" || dir == "." {
		return "." + base + "-scratch"
	}
	return filepath.Join(dir, "."+base+"-scratch")
}

func pageScratchStem(page int) string {
	return fmt.Sprintf("page-%03d", page)
}

func scratchPageTextPath(scratchDir string, page int) string {
	return filepath.Join(scratchDir, pageScratchStem(page)+"-text.md")
}

func scratchPageTablePath(scratchDir string, page int) string {
	return filepath.Join(scratchDir, pageScratchStem(page)+"-table.md")
}

func scratchPageMergedPath(scratchDir string, page int) string {
	return filepath.Join(scratchDir, pageScratchStem(page)+".md")
}

func ensureScratchDir(scratchDir string) error {
	return os.MkdirAll(scratchDir, 0o755)
}

func writeScratchFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeScratchDir(scratchDir string) error {
	return os.RemoveAll(scratchDir)
}
