package mnz

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

var FileArgType = ArgType{
	Name: ArgTypeFile,
	AvailableSpecs: map[string]interface{}{
		"size":   nil,
		"lines":  nil,
		"sha256": nil,
	},
	RequiredSpecs: []string{"sha256"},
}

func fileSpecs(path string, mNames []string) (map[string]any, error) {
	spec := make(map[string]any)

	for _, mn := range mNames {
		switch mn {

		case "size":
			sz, err := fileSize(path)
			if err != nil {
				return nil, err
			}
			spec[mn] = sz

		case "lines":
			count, err := countLinesInZip(path)
			if err != nil {
				return nil, err
			}
			spec[mn] = count

		case "sha256":
			hash, err := fileSha256(path)
			if err != nil {
				return nil, err
			}
			spec[mn] = hash

		default:
			return nil, fmt.Errorf("spec name '%s' is not available", mn)
		}
	}

	return spec, nil
}

func countLinesInZip(path string) (int64, error) {
	newPath := path
	zipped, err := isZip(path)
	if err != nil {
		return 0, err
	}
	if zipped {
		tmp, errt := os.CreateTemp(filepath.Dir(path), "*_"+filepath.Base(path))
		if errt != nil {
			return 0, fmt.Errorf("failed to open tmp file, error %w", errt)
		}

		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		newPath, err = unzipFile(path, tmp)
		if err != nil {
			return 0, err
		}
	}

	count, errc := countLines(newPath)
	if errc != nil {
		return 0, errc
	}
	return count, nil
}

func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 1024*1024) // using large read buffer to increase throughput
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", fmt.Errorf("failed to get SHA256 hash of file %s, error %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSize(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s, error %w", path, err)
	}
	defer f.Close()

	fi, errf := f.Stat()
	if errf != nil {
		return 0, fmt.Errorf("failed to stat file %s, error %w", path, errf)
	}

	return fi.Size(), nil
}

func isZip(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s, error %w", path, err)
	}
	defer f.Close()

	buff := make([]byte, 512) // why 512 bytes ? see http://golang.org/pkg/net/http/#DetectContentType
	_, err = f.Read(buff)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s, error %w", path, err)
	}

	filetype := http.DetectContentType(buff)
	return filetype == "application/x-gzip" || filetype == "application/zip", nil
}

func countLines(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s, error %w", path, err)
	}
	defer f.Close()

	var lc int64
	scanner := bufio.NewScanner(f)
	// https://stackoverflow.com/questions/8757389/reading-a-file-line-by-line-in-go/16615559#comment41613175_16615559
	const maxCapacity int = 1024 * 1024 // increase buffer more in case of long lines
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	for scanner.Scan() {
		lc++
	}
	if err = scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read file %s, error %w", path, err)
	}

	return lc, err
}

func unzipFile(path string, dst *os.File) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file %s, error %w", path, err)
	}
	defer archive.Close()

	if len(archive.File) > 1 {
		return "", fmt.Errorf("zip %s contains more than one file", path)
	}
	f := archive.File[0]

	if f.FileInfo().IsDir() {
		return "", fmt.Errorf("zip %s contains directory", path)
	}

	fileInArchive, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file in zip %s, error %w", path, err)
	}
	defer fileInArchive.Close()

	if _, err := io.Copy(dst, fileInArchive); err != nil {
		return "", fmt.Errorf("failed to write dst file %s, error %w", dst.Name(), err)
	}

	return dst.Name(), nil
}
