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
	blobsDir := filepath.Join(tuskDir, "images", "blobs", "sha256")
	entries, err := os.ReadDir(blobsDir)
	if err != nil {
		fmt.Println("No images found")
		return
	}

	blobCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			blobCount++
		}
	}
	if blobCount == 0 {
		fmt.Println("No images found")
		return
	}

	manifestsDir := filepath.Join(tuskDir, "images", "manifests")
	manifestEntries, _ := os.ReadDir(manifestsDir)
	indexDir := filepath.Join(tuskDir, "images", "index")

	fmt.Println("REPOSITORY   TAG      DIGEST                                   SIZE")
	for _, entry := range manifestEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			digest := strings.TrimSuffix(entry.Name(), ".json")
			tag := "latest"
			if indexDir != "" {
				_ = indexDir
			}
			shortDigest := digest
			if len(digest) > 16 {
				shortDigest = digest[:16]
			}
			fmt.Printf("%-12s %-8s %s...\n", "local", tag, shortDigest)
		}
	}

	fmt.Printf("\nTotal: %d blobs stored\n", blobCount)
}
