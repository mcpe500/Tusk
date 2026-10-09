package artifact

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHostingGateAcceptsVerifiedLabArtifacts(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("verified guest image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/system.qcow2" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion, ReleaseVersion: "lab", GeneratedAt: "2026-10-10T00:00:00Z", KeyID: "test",
		Artifacts: []Artifact{{
			ID: "lab-system", Backend: "qemu-system-aarch64", Architecture: "arm64",
			URL: server.URL + "/system.qcow2", MediaType: "application/octet-stream",
			SHA256: sha256Hex(payload), Size: int64(len(payload)),
		}},
	}
	if err := manifest.Sign(private); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := DownloadVerifiedLabArtifact(context.Background(), Downloader{Client: server.Client(), AllowInsecure: true}, manifestPath, public, "lab-system", filepath.Join(t.TempDir(), "system.qcow2"))
	if err != nil {
		t.Fatalf("DownloadVerifiedLabArtifact() error = %v", err)
	}
	if result.SHA256 != manifest.Artifacts[0].SHA256 || result.Path == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
