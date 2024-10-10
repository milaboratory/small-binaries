package converter

import (
	"fmt"
	"path/filepath"
	"strings"
)

func DetectTableSeparator(inputName string) (rune, error) {
	ext := strings.ToLower(filepath.Ext(inputName))
	switch ext {
	case ".tsv":
		return '\t', nil
	case ".csv":
		return ',', nil
	case ".scsv":
		return ';', nil
	default:
		return 0, fmt.Errorf("cannot detect table separator from file extension: %s", ext)
	}
}

func Wrap(err error, msg string) error {
	if err == nil {
		return err
	}

	return fmt.Errorf("%s: %w", msg, err)
}

func Wrapf(err error, msg string, args ...any) error {
	if err == nil {
		return err
	}

	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), err)
}
