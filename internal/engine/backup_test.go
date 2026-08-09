package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupRestoreRoundTripAndChecksum(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "data")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "value"), []byte("persisted"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "backup.tar.gz")
	metadata, err := CreateBackup(context.Background(), source, backup, BackupMetadata{BackendID: "proot-native", Architecture: "arm64", TuskVersion: "test", RuntimeVersion: "test", DataSchema: 1, WorkspaceOutside: true, Stopped: true})
	if err != nil || metadata.Checksum == "" {
		t.Fatalf("CreateBackup() = %#v, %v", metadata, err)
	}
	destination := filepath.Join(root, "restored")
	if err := RestoreBackup(context.Background(), backup, destination, BackendIdentity{BackendID: "proot-native", Architecture: "arm64"}); err != nil {
		t.Fatalf("RestoreBackup() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "value"))
	if err != nil || string(data) != "persisted" {
		t.Fatalf("restored value = %q, %v", data, err)
	}
	if err := os.WriteFile(backup, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RestoreBackup(context.Background(), backup, filepath.Join(root, "bad"), BackendIdentity{BackendID: "proot-native", Architecture: "arm64"}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered backup error = %v", err)
	}
}

func TestSafeArchiveTargetRejectsTraversal(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"../escape", "/absolute", `..\\escape`} {
		if _, err := safeArchiveTarget(t.TempDir(), name); err == nil {
			t.Fatalf("unsafe archive name %q was accepted", name)
		}
	}
}
