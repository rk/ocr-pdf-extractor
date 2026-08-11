package extract

import (
	"fmt"
	"os"
	"path/filepath"
)

func validatePaths(inputPath, outputPath string) error {
	if err := validateInput(inputPath); err != nil {
		return err
	}

	absIn, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("input path: %w", err)
	}
	absOut, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("output path: %w", err)
	}

	if samePath, err := pathsEqual(absIn, absOut); err != nil {
		return err
	} else if samePath {
		return fmt.Errorf("input and output paths must differ")
	}

	return nil
}

func pathsEqual(a, b string) (bool, error) {
	aEval, err := evalSymlinks(a)
	if err != nil {
		return false, err
	}
	bEval, err := evalSymlinks(b)
	if err != nil {
		return false, err
	}
	return aEval == bEval, nil
}

func evalSymlinks(path string) (string, error) {
	eval, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Clean(path), nil
		}
		return "", err
	}
	return eval, nil
}

func validateInput(inputPath string) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", inputPath)
		}
		return fmt.Errorf("input file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("input path is a directory: %s", inputPath)
	}
	return nil
}
