package artifact

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestManifestSignAndVerify(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validManifest()
	if err := manifest.Sign(privateKey); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	if err := manifest.Verify(publicKey); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	manifest.ReleaseVersion = "tampered"
	if err := manifest.Verify(publicKey); err == nil {
		t.Fatal("Verify() accepted a tampered manifest")
	}
}

func TestManifestRejectsDuplicateArtifact(t *testing.T) {
	t.Parallel()
	manifest := validManifest()
	manifest.Signature = "not-empty"
	manifest.Artifacts = append(manifest.Artifacts, manifest.Artifacts[0])
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() accepted duplicate artifact IDs")
	}
}

func validManifest() Manifest {
	return Manifest{
		SchemaVersion:  ManifestSchemaVersion,
		ReleaseVersion: "0.1.0",
		GeneratedAt:    "2026-08-09T00:00:00Z",
		KeyID:          "release-2026",
		Artifacts: []Artifact{
			{
				ID:           "probe-arm64",
				Backend:      "qemu-system-aarch64",
				Architecture: "arm64",
				URL:          "https://releases.example.invalid/probe-arm64.qcow2",
				MediaType:    "application/octet-stream",
				SHA256:       strings.Repeat("a", 64),
				Size:         1024,
			},
		},
	}
}
