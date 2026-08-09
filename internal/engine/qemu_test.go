package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestNewQEMUAdapterRequiresSafeAbsoluteConfiguration(t *testing.T) {
	t.Parallel()
	_, err := NewQEMUAdapter(QEMUConfig{CandidateID: "qemu-system-x86_64", QEMUPath: "qemu", SystemImage: "system.qcow2", DataImage: "/data", RuntimeDir: "/run", SharedRoot: "/shared", SSHPrivateKey: "/key", KnownHosts: "/known", SSHPort: 2222, CPUs: 2, MemoryMiB: 512})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("NewQEMUAdapter() error = %v", err)
	}
}

func TestQEMUAdapterInitializesStateAndUsesTCGByDefault(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	system := filepath.Join(directory, "system.qcow2")
	data := filepath.Join(directory, "data.qcow2")
	key := filepath.Join(directory, "guest_ed25519")
	known := filepath.Join(directory, "known_hosts")
	for _, path := range []string{system, data, key, known} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	adapter, err := NewQEMUAdapter(QEMUConfig{CandidateID: "qemu-system-x86_64", QEMUPath: "/usr/bin/qemu-system-x86_64", SystemImage: system, DataImage: data, RuntimeDir: filepath.Join(directory, "run"), SharedRoot: filepath.Join(directory, "shared"), SSHPrivateKey: key, KnownHosts: known, SSHPort: 2222, CPUs: 2, MemoryMiB: 512})
	if err != nil {
		t.Fatalf("NewQEMUAdapter() error = %v", err)
	}
	if err := adapter.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	status, err := adapter.Status(context.Background())
	if err != nil || status.State != StateStopped {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	args := strings.Join(adapter.qemuArgs(), " ")
	if !strings.Contains(args, "-accel tcg,thread=multi") || strings.Contains(args, "-enable-kvm") {
		t.Fatalf("QEMU acceleration args = %q", args)
	}
	if !strings.Contains(args, "-qmp unix:") || !strings.Contains(args, "-serial file:") || !strings.Contains(args, "hostfwd=tcp:127.0.0.1:2222-:22") {
		t.Fatalf("QEMU integration args = %q", args)
	}
}

func TestQEMUPreflightRejectsMissingPinnedEnvironment(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	adapter, err := NewQEMUAdapter(QEMUConfig{CandidateID: "qemu-system-aarch64", QEMUPath: "/missing/qemu", SystemImage: filepath.Join(directory, "system.qcow2"), DataImage: filepath.Join(directory, "data.qcow2"), RuntimeDir: filepath.Join(directory, "run"), SharedRoot: filepath.Join(directory, "shared"), SSHPrivateKey: filepath.Join(directory, "key"), KnownHosts: filepath.Join(directory, "known"), SSHPort: 2223, CPUs: 2, MemoryMiB: 512})
	if err != nil {
		t.Fatalf("NewQEMUAdapter() error = %v", err)
	}
	preflight, err := adapter.Preflight(context.Background(), platform.Device{HostArch: "arm64"})
	if err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if preflight.Available || len(preflight.Reasons) == 0 {
		t.Fatalf("missing QEMU environment was available: %#v", preflight)
	}
}

func TestShellQuotePreventsRemoteCommandInjection(t *testing.T) {
	t.Parallel()
	quoted := shellQuote("safe; touch /tmp/should-not-run")
	if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") || strings.Contains(quoted, "safe; touch /tmp/should-not-run' ") {
		t.Fatalf("unsafe remote quoting: %q", quoted)
	}
}
