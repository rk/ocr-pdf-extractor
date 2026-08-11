package extract

import (
	"fmt"
	"os/exec"
	"strings"
)

var requiredTools = []string{
	"pdfinfo",
	"pdftotext",
	"pdfimages",
	"tesseract",
	"pdftoppm",
}

// CheckDependencies verifies that all external tools are available on PATH.
func CheckDependencies() error {
	var missing []string
	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"missing required tools: %s\ninstall with: sudo apt install poppler-utils tesseract-ocr (Debian/Ubuntu) or brew install poppler tesseract (macOS)",
		strings.Join(missing, ", "),
	)
}
