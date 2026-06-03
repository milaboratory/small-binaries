package internal

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func writeGzip(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return p
}

func writeZstd(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return p
}

// TestCountLines covers all supported formats plus boundary cases.
// Each case constructs the minimum input needed for one behavior.
func TestCountLines(t *testing.T) {
	cases := []struct {
		name string
		make func(t *testing.T) string
		want int64
	}{
		{"plain", func(t *testing.T) string { return writeFile(t, "a.txt", []byte("a\nb\nc\n")) }, 3},
		{"gzip", func(t *testing.T) string { return writeGzip(t, "a.txt.gz", []byte("x\ny\n")) }, 2},
		{"zstd", func(t *testing.T) string { return writeZstd(t, "a.txt.zst", []byte("1\n2\n3\n4\n")) }, 4},
		// stdlib has no bzip2 writer, so read a committed fixture (3 lines).
		{"bzip2_fixture", func(t *testing.T) string { return "testdata/sample.txt.bz2" }, 3},
		// boundary: an empty file has zero newlines.
		{"empty", func(t *testing.T) string { return writeFile(t, "empty.txt", []byte("")) }, 0},
		// CONTRACT: count is the number of '\n', so a final line with no
		// trailing newline is not counted. The Task 7 (platforma) consumer
		// parses this exact value, so the semantics must stay pinned.
		{"no_trailing_newline", func(t *testing.T) string { return writeFile(t, "nt.txt", []byte("a\nb\nc")) }, 2},
		// crosses the 1 MiB read buffer many times — guards the chunked
		// accumulation loop (every other case fits in a single Read).
		{"large_multichunk", func(t *testing.T) string {
			return writeFile(t, "big.txt", bytes.Repeat([]byte("x\n"), 600_000))
		}, 600_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CountLines(tc.make(t))
			if err != nil {
				t.Fatalf("CountLines: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CountLines = %d, want %d", got, tc.want)
			}
		})
	}
}

// A missing/unreadable file must surface an error (the CLI turns it into a
// non-zero exit) rather than silently returning 0.
func TestCountLines_MissingFile(t *testing.T) {
	if _, err := CountLines(filepath.Join(t.TempDir(), "does-not-exist.txt")); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}
