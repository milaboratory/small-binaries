package internal

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// CountLines returns the number of '\n' bytes in the (optionally compressed) file.
// Compression is inferred from the file extension: .gz, .bz2, .zst, else raw.
func CountLines(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var r io.Reader = f
	switch {
	case strings.HasSuffix(path, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, err
		}
		defer gz.Close()
		r = gz
	case strings.HasSuffix(path, ".bz2"):
		r = bzip2.NewReader(f)
	case strings.HasSuffix(path, ".zst"):
		zr, err := zstd.NewReader(f)
		if err != nil {
			return 0, err
		}
		defer zr.Close()
		r = zr
	}

	var count int64
	buf := make([]byte, 1<<20)
	reader := bufio.NewReaderSize(r, 1<<20)
	for {
		n, err := reader.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				count++
			}
		}
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
	}
}
