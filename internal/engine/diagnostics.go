package engine

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`(?i)(["']?(?:password|token|secret|private[_-]?key)["']?\s*[:=]\s*["']?)([^\s,"'}]+)`)
var bearerPattern = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)([^\s]+)`)
var basicPattern = regexp.MustCompile(`(?i)(authorization\s*:\s*basic\s+)([^\s]+)`)

func RedactDiagnostics(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1<redacted>")
	value = bearerPattern.ReplaceAllString(value, "$1<redacted>")
	return basicPattern.ReplaceAllString(value, "$1<redacted>")
}

func CreateDiagnosticBundle(destination, root string, files []string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, relative := range files {
		clean := filepath.Clean(relative)
		if filepath.IsAbs(relative) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("diagnostic path escapes root: %q", relative)
		}
		path := filepath.Join(root, relative)
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		data = []byte(RedactDiagnostics(string(data)))
		header := &tar.Header{Name: filepath.ToSlash(relative), Mode: 0o600, Size: int64(len(data))}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write(data); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func ReadDiagnosticBundle(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var output []byte
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return output, nil
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		output = append(output, []byte(header.Name+"\n")...)
		output = append(output, data...)
	}
}
