package report

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteAtomic publishes r at path through a temporary file and a rename, creating the parent
// directory when it is missing. A reader on the other side of a shared filesystem therefore sees
// either the previous report or the new one, never a half-written file.
func WriteAtomic(path string, r *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
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
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
