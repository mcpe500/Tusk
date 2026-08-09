package engine

import (
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestBuildEvidenceFingerprintChangesWithProbeAndPolicy(t *testing.T) {
	t.Parallel()
	device := platform.Device{HostOS: "linux", HostArch: "amd64", Fingerprint: "device", Commands: map[string]string{"qemu-system-x86_64": "/missing"}}
	candidate := Candidate{ID: "qemu-system-x86_64", RequiredCommands: []string{"qemu-system-x86_64"}}
	balanced := BalancedPolicy()
	first, err := BuildEvidenceFingerprint(device, candidate, "image@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", balanced)
	if err != nil {
		t.Fatalf("BuildEvidenceFingerprint() error = %v", err)
	}
	second, err := BuildEvidenceFingerprint(device, candidate, "image@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", balanced)
	if err != nil {
		t.Fatalf("BuildEvidenceFingerprint() error = %v", err)
	}
	if first == second {
		t.Fatal("probe image change did not invalidate fingerprint")
	}
}
