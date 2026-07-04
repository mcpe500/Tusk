package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/internal/image"
)

func runPull(ref string) {
	fmt.Printf("Pulling %s...\n", ref)

	store := image.New(filepath.Join(tuskDir, "images"))
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init image store: %v\n", err)
		os.Exit(1)
	}

	puller := image.NewPuller(store)
	ctx := context.Background()
	if err := puller.Pull(ctx, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Pull failed: %v\n", err)
		os.Exit(1)
	}
}

func runImages() {
	store := image.New(filepath.Join(tuskDir, "images"))
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init image store: %v\n", err)
		os.Exit(1)
	}

	manifestsDir := filepath.Join(tuskDir, "images", "manifests")
	manifestEntries, _ := os.ReadDir(manifestsDir)
	if len(manifestEntries) == 0 {
		fmt.Println("No images found")
		return
	}

	// Load refs registry (digest → []ref).
	refs, _ := store.ListRefs()

	// Count blobs.
	blobsDir := filepath.Join(tuskDir, "images", "blobs", "sha256")
	blobEntries, _ := os.ReadDir(blobsDir)
	blobCount := 0
	for _, entry := range blobEntries {
		if !entry.IsDir() {
			blobCount++
		}
	}

	fmt.Printf("%-30s %-10s %-20s %s\n", "REPOSITORY", "TAG", "DIGEST", "SIZE")
	for _, entry := range manifestEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		digest := strings.TrimSuffix(entry.Name(), ".json")

		// Get size from manifest file.
		info, _ := entry.Info()
		size := info.Size()

		// Sum layer blob sizes for total image size.
		if m, err := store.GetManifest(digest); err == nil {
			for _, layer := range m.Layers {
				if blobInfo, err := os.Stat(store.BlobPath(layer.Digest)); err == nil {
					size += blobInfo.Size()
				}
			}
		}

		shortDigest := digest
		if i := strings.Index(digest, ":"); i >= 0 {
			shortDigest = digest[i+1:]
		}
		if len(shortDigest) > 19 {
			shortDigest = shortDigest[:19]
		}

		// Find ref names.
		refNames := refs[digest]
		if len(refNames) == 0 {
			fmt.Printf("%-30s %-10s %s... %s\n", "<none>", "<none>", shortDigest, humanSize(size))
			continue
		}
		for _, ref := range refNames {
			repo, tag := splitRef(ref)
			fmt.Printf("%-30s %-10s %s... %s\n", repo, tag, shortDigest, humanSize(size))
		}
	}

	fmt.Printf("\nTotal: %d blobs stored\n", blobCount)
}

func splitRef(ref string) (repo, tag string) {
	repo = ref
	tag = "latest"
	if idx := strings.LastIndex(ref, ":"); idx >= 0 {
		// Make sure the colon isn't part of a port (e.g. registry:5000/foo).
		if slash := strings.LastIndex(ref, "/"); slash < idx {
			repo = ref[:idx]
			tag = ref[idx+1:]
		}
	}
	return
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
