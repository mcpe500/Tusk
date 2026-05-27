package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func DigestHash(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func DigestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyDigest(path, expected string) (bool, error) {
	actual, err := DigestFile(path)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}