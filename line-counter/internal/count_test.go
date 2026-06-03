package internal

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestCountLines_Plain(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("a\nb\nc\n"), 0o644)
	if n, err := CountLines(p); err != nil || n != 3 {
		t.Fatalf("got %d, %v; want 3", n, err)
	}
}

func TestCountLines_Gzip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt.gz")
	f, _ := os.Create(p)
	gz := gzip.NewWriter(f)
	gz.Write([]byte("x\ny\n"))
	gz.Close()
	f.Close()
	if n, err := CountLines(p); err != nil || n != 2 {
		t.Fatalf("got %d, %v; want 2", n, err)
	}
}

func TestCountLines_Zstd(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt.zst")
	f, _ := os.Create(p)
	zw, _ := zstd.NewWriter(f)
	zw.Write([]byte("1\n2\n3\n4\n"))
	zw.Close()
	f.Close()
	if n, err := CountLines(p); err != nil || n != 4 {
		t.Fatalf("got %d, %v; want 4", n, err)
	}
}
