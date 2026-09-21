package report

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteAtomic publishes r at path through a temporary file and a rename, creating the parent
// directory when it is missing. A reader on the other side of a shared filesystem therefore sees
// either the previous report or the new one, never a half-written file.
//
// The directory is 0755 and the file 0644 on purpose: the runner reads them under another uid.
func WriteAtomic(path string, r *Report) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	err = os.WriteFile(tmp, append(data, '\n'), 0o644)
	if err != nil {
		return err
	}
	err = os.Rename(tmp, path)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Read loads a report, for tests and tooling.
func Read(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	err = json.Unmarshal(b, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
