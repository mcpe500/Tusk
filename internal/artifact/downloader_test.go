package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloaderResumesAndVerifies(t *testing.T) {
	t.Parallel()
	content := []byte("real signed runtime artifact")
	rangeSeen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		rangeHeader := request.Header.Get("Range")
		rangeSeen <- rangeHeader
		if rangeHeader == "bytes=5-" {
			writer.Header().Set("Content-Range", fmt.Sprintf("bytes 5-%d/%d", len(content)-1, len(content)))
			writer.WriteHeader(http.StatusPartialContent)
			_, _ = writer.Write(content[5:])
			return
		}
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(destination+".part", content[:5], 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	artifact := Artifact{
		ID: "test", Backend: "test", Architecture: "test",
		URL: server.URL, MediaType: "application/octet-stream",
		SHA256: hex.EncodeToString(digest[:]), Size: int64(len(content)),
	}
	if err := (Downloader{Client: server.Client(), AllowInsecure: true}).Download(context.Background(), artifact, destination); err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if got := <-rangeSeen; got != "bytes=5-" {
		t.Fatalf("Range = %q", got)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("downloaded content = %q", got)
	}
}

func TestDownloaderDeletesChecksumMismatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("wrong"))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "artifact.bin")
	artifact := Artifact{
		ID: "test", Backend: "test", Architecture: "test",
		URL: server.URL, MediaType: "application/octet-stream",
		SHA256: strings.Repeat("0", 64), Size: 5,
	}
	if err := (Downloader{Client: server.Client(), AllowInsecure: true}).Download(context.Background(), artifact, destination); err == nil {
		t.Fatal("Download() accepted a checksum mismatch")
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial artifact remains after mismatch: %v", err)
	}
}

func TestDownloaderKeepsInterruptedPartialForResume(t *testing.T) {
	t.Parallel()
	content := []byte("resume this artifact")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		requests++
		if requests == 1 {
			_, _ = writer.Write(content[:6])
			return
		}
		if got := request.Header.Get("Range"); got != "bytes=6-" {
			t.Errorf("Range = %q, want bytes=6-", got)
		}
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes 6-%d/%d", len(content)-1, len(content)))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(content[6:])
	}))
	defer server.Close()

	digest := sha256.Sum256(content)
	artifact := Artifact{
		ID: "test", Backend: "test", Architecture: "test",
		URL: server.URL, MediaType: "application/octet-stream",
		SHA256: hex.EncodeToString(digest[:]), Size: int64(len(content)),
	}
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	downloader := Downloader{Client: server.Client(), AllowInsecure: true}
	if err := downloader.Download(context.Background(), artifact, destination); err == nil {
		t.Fatal("first Download() unexpectedly succeeded")
	}
	info, err := os.Stat(destination + ".part")
	if err != nil {
		t.Fatalf("partial file missing: %v", err)
	}
	if info.Size() != 6 {
		t.Fatalf("partial size = %d, want 6", info.Size())
	}
	if err := downloader.Download(context.Background(), artifact, destination); err != nil {
		t.Fatalf("resumed Download() error = %v", err)
	}
}

func TestDownloaderRejectsInsecureURLByDefault(t *testing.T) {
	t.Parallel()
	artifact := Artifact{
		ID: "test", Backend: "test", Architecture: "test",
		URL: "http://example.invalid/artifact", MediaType: "application/octet-stream",
		SHA256: strings.Repeat("0", 64), Size: 1,
	}
	if err := (Downloader{}).Download(context.Background(), artifact, filepath.Join(t.TempDir(), "artifact")); err == nil {
		t.Fatal("Download() accepted an insecure URL")
	}
}

func TestDownloaderRejectsMediaTypeMismatch(t *testing.T) {
	t.Parallel()
	content := []byte("valid bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	digest := sha256.Sum256(content)
	artifact := Artifact{
		ID: "test", Backend: "test", Architecture: "test",
		URL: server.URL, MediaType: "application/octet-stream",
		SHA256: hex.EncodeToString(digest[:]), Size: int64(len(content)),
	}
	err := (Downloader{Client: server.Client(), AllowInsecure: true}).Download(
		context.Background(), artifact, filepath.Join(t.TempDir(), "artifact"),
	)
	if err == nil || !strings.Contains(err.Error(), "Content-Type mismatch") {
		t.Fatalf("Download() error = %v, want Content-Type mismatch", err)
	}
}
