package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceSystemRollsBackAndPreservesData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	current := filepath.Join(root, "system")
	replacement := filepath.Join(root, "replacement")
	data := filepath.Join(root, "data")
	for _, path := range []string{current, replacement, data} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, "version"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "version"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := (UpdateManager{Root: root}).ReplaceSystem(context.Background(), current, replacement, data, func(context.Context) error { return errors.New("unhealthy") })
	if err == nil {
		t.Fatal("unhealthy replacement was accepted")
	}
	version, readErr := os.ReadFile(filepath.Join(current, "version"))
	if readErr != nil || string(version) != "old" {
		t.Fatalf("rollback version = %q, %v", version, readErr)
	}
	if _, err := os.Stat(data); err != nil {
		t.Fatalf("persistent data disappeared: %v", err)
	}
}
