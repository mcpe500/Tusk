package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharesRejectOverlapAndPreserveReadOnlyIntent(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	registry, err := DefaultShareRegistry(filepath.Join(directory, "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(directory, "project")
	if err := makeDirectory(project); err != nil {
		t.Fatal(err)
	}
	registry, err = AddShare(registry, project, "project", true)
	if err != nil {
		t.Fatalf("AddShare() error = %v", err)
	}
	translated, err := TranslateHostPath(filepath.Join(project, "file.txt"), directory, registry)
	if err != nil || !strings.HasPrefix(translated, "/mnt/tusk/shares/project") {
		t.Fatalf("TranslateHostPath() = %q, %v", translated, err)
	}
	mount, err := TranslateMount(filepath.Join(project, "file.txt")+":/data:ro", directory, registry)
	if err != nil || !strings.HasSuffix(mount, ":/data:ro") {
		t.Fatalf("TranslateMount() = %q, %v", mount, err)
	}
	if _, err := AddShare(registry, project, "nested", false); err == nil {
		t.Fatal("overlapping share was accepted")
	}
	keyValue, err := TranslateMount("type=bind,source="+filepath.Join(project, "file.txt")+",target=/data", directory, registry)
	if err != nil || !strings.Contains(keyValue, "readonly") {
		t.Fatalf("key/value read-only mount = %q, %v", keyValue, err)
	}
}

func TestTranslateHostPathSuggestsShareForUnregisteredPath(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	registry, err := DefaultShareRegistry(filepath.Join(directory, "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(directory, "outside")
	if err := makeDirectory(outside); err != nil {
		t.Fatal(err)
	}
	_, err = TranslateHostPath(outside, directory, registry)
	if err == nil || !strings.Contains(err.Error(), "tusk share add") {
		t.Fatalf("TranslateHostPath() error = %v", err)
	}
}

func makeDirectory(path string) error {
	return os.MkdirAll(path, 0o700)
}
