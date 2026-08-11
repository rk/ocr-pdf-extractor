package extract

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	minImageWidth  = 50
	minImageHeight = 50
)

// substantialImageCount returns the number of non-trivial embedded images
// in the given page range according to pdfimages -list.
func substantialImageCount(inputPath string, firstPage, lastPage int) (int, error) {
	args := []string{"-list"}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	args = append(args, inputPath)

	out, err := exec.Command("pdfimages", args...).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfimages -list: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return parseSubstantialImageCount(string(out)), nil
}

func parseSubstantialImageCount(listOutput string) int {
	count := 0
	for _, line := range strings.Split(listOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "page") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[2] != "image" {
			continue
		}

		width, errW := strconv.Atoi(fields[3])
		height, errH := strconv.Atoi(fields[4])
		if errW != nil || errH != nil {
			continue
		}
		if width >= minImageWidth && height >= minImageHeight {
			count++
		}
	}
	return count
}
