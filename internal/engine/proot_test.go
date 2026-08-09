package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestNewProotAdapterRejectsRelativePaths(t *testing.T) {
	t.Parallel()
	_, err := NewProotAdapter(ProotConfig{CandidateID: "proot-native", Rootfs: "rootfs", DataDir: "/tmp/data", SharedRoot: "/tmp/shared"})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("NewProotAdapter() error = %v, want absolute-path validation", err)
	}
}

func TestProotAdapterInitializesPersistentStateAndBuildsVector(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	rootfs := filepath.Join(directory, "rootfs")
	if err := os.MkdirAll(rootfs, 0o700); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewProotAdapter(ProotConfig{
		CandidateID:  "proot-qemu-user-x86_64",
		ProotPath:    "/usr/bin/proot",
		QEMUUserPath: "/usr/bin/qemu-x86_64",
		Rootfs:       rootfs,
		DataDir:      filepath.Join(directory, "data"),
		SharedRoot:   filepath.Join(directory, "shared"),
	})
	if err != nil {
		t.Fatalf("NewProotAdapter() error = %v", err)
	}
	if err := adapter.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	status, err := adapter.Status(context.Background())
	if err != nil || status.State != StateStopped {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	args := adapter.prootArgs([]string{"podman", "info"})
	joined := strings.Join(args, " ")
	for _, arg := range args {
		if arg == "--" {
			t.Fatalf("proot args contain unsupported option separator: %q", args)
		}
	}
	for _, expected := range []string{"-r " + rootfs, "-q /usr/bin/qemu-x86_64", "podman info"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("proot args %q missing %q", joined, expected)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "data", "state.json")); err != nil {
		t.Fatalf("persistent state missing: %v", err)
	}
}

func TestProotPreflightReportsMissingRootfsWithoutQualification(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	adapter, err := NewProotAdapter(ProotConfig{CandidateID: "proot-native", Rootfs: filepath.Join(directory, "missing"), DataDir: filepath.Join(directory, "data"), SharedRoot: filepath.Join(directory, "shared")})
	if err != nil {
		t.Fatalf("NewProotAdapter() error = %v", err)
	}
	preflight, err := adapter.Preflight(context.Background(), platform.Device{HostArch: "arm64"})
	if err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if preflight.Available || len(preflight.Reasons) == 0 {
		t.Fatalf("missing rootfs was available: %#v", preflight)
	}
}
