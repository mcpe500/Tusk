package artifact

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcpe500/Tusk/internal/state"
)

type VerifiedArtifact struct {
	ID     string
	Path   string
	SHA256 string
	Size   int64
}

func DownloadVerifiedLabArtifact(ctx context.Context, downloader Downloader, manifestPath string, publicKey ed25519.PublicKey, artifactID, destination string) (VerifiedArtifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if manifestPath == "" || artifactID == "" || destination == "" {
		return VerifiedArtifact{}, fmt.Errorf("manifest, artifact ID, and destination are required")
	}
	if !filepath.IsAbs(destination) {
		return VerifiedArtifact{}, fmt.Errorf("destination must be absolute")
	}
	var manifest Manifest
	if err := state.ReadJSON(manifestPath, &manifest); err != nil {
		return VerifiedArtifact{}, fmt.Errorf("read lab artifact manifest: %w", err)
	}
	if err := manifest.Verify(publicKey); err != nil {
		return VerifiedArtifact{}, fmt.Errorf("verify lab artifact manifest: %w", err)
	}
	var selected *Artifact
	for index := range manifest.Artifacts {
		if manifest.Artifacts[index].ID == artifactID {
			selected = &manifest.Artifacts[index]
			break
		}
	}
	if selected == nil {
		return VerifiedArtifact{}, fmt.Errorf("artifact %q not found in signed lab manifest", artifactID)
	}
	if !strings.HasPrefix(selected.ID, "lab-") {
		return VerifiedArtifact{}, fmt.Errorf("lab installer accepts only explicit lab-* artifact IDs")
	}
	if err := downloader.Download(ctx, *selected, destination); err != nil {
		return VerifiedArtifact{}, fmt.Errorf("download lab artifact: %w", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		return VerifiedArtifact{}, fmt.Errorf("inspect lab artifact: %w", err)
	}
	return VerifiedArtifact{ID: selected.ID, Path: destination, SHA256: selected.SHA256, Size: info.Size()}, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
