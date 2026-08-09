package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsRedactsSecrets(t *testing.T) {
	t.Parallel()
	if value := RedactDiagnostics(`token=abc "password":"xyz" safe`); strings.Contains(value, "abc") || strings.Contains(value, "xyz") {
		t.Fatalf("secret was not redacted: %q", value)
	}
}

func TestDiagnosticBundleContainsAllowlistedRedactedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "engine.log"), []byte("token=abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "diagnostics.tar.gz")
	if err := CreateDiagnosticBundle(bundle, root, []string{"engine.log"}); err != nil {
		t.Fatalf("CreateDiagnosticBundle() error = %v", err)
	}
	data, err := ReadDiagnosticBundle(bundle)
	if err != nil || strings.Contains(string(data), "abc") {
		t.Fatalf("diagnostic data = %q, err=%v", data, err)
	}
}
