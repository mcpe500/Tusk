package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallPreservesWorkspaceAndSharesWithoutPurge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, relative := range []string{"config.json", "engines/a/data", "workspaces/project", "shares.json"} {
		path := filepath.Join(root, relative)
		if filepath.Ext(path) == "" {
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Uninstall(UninstallOptions{Root: root}); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	for _, relative := range []string{"workspaces/project", "shares.json", "engines/a/data"} {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("preserved path %s missing: %v", relative, err)
		}
	}
}
