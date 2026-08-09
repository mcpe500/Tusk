package engine

import (
	"crypto/ed25519"
	"testing"

	"github.com/mcpe500/Tusk/internal/artifact"
)

func TestVerifyProbeManifestRequiresSignatureAndSelectsMatchingArtifact(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	manifest := artifact.Manifest{
		SchemaVersion:  1,
		ReleaseVersion: "test",
		GeneratedAt:    "2026-01-01T00:00:00Z",
		KeyID:          "test-key",
		Artifacts: []artifact.Artifact{{
			ID:           "probe-arm64",
			Backend:      "proot-native",
			Architecture: "arm64",
			URL:          "https://example.invalid/probe.tar.zst",
			MediaType:    "application/zstd",
			SHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:         1024,
		}},
	}
	if err := manifest.Sign(privateKey); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	selected, err := VerifyProbeManifest(manifest, publicKey, "proot-native", "arm64")
	if err != nil {
		t.Fatalf("VerifyProbeManifest() error = %v", err)
	}
	if selected.ID != "probe-arm64" {
		t.Fatalf("selected artifact = %#v", selected)
	}
	manifest.Signature = ""
	if _, err := VerifyProbeManifest(manifest, publicKey, "proot-native", "arm64"); err == nil {
		t.Fatal("unsigned manifest was accepted")
	}
}
