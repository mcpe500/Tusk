package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcpe500/Tusk/internal/artifact"
	"github.com/mcpe500/Tusk/internal/config"
	"github.com/mcpe500/Tusk/internal/engine"
	"github.com/mcpe500/Tusk/internal/platform"
)

func runLabInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("engine lab-install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", os.Getenv("TUSK_LAB_MANIFEST"), "signed lab artifact manifest JSON")
	publicKeyHex := flags.String("public-key", os.Getenv("TUSK_LAB_PUBLIC_KEY"), "trusted Ed25519 public key in hex")
	candidate := flags.String("candidate", "qemu-system-aarch64", "QEMU candidate ID")
	systemArtifact := flags.String("system-artifact", "", "signed lab-* system image artifact ID")
	dataArtifact := flags.String("data-artifact", "", "signed lab-* data image artifact ID")
	if err := flags.Parse(args); err != nil {
		return exUsage
	}
	if flags.NArg() != 0 || *manifestPath == "" || *publicKeyHex == "" || *systemArtifact == "" || *dataArtifact == "" {
		fmt.Fprintln(stderr, "usage: tusk engine lab-install --manifest <file> --public-key <hex> --candidate qemu-system-aarch64 --system-artifact lab-system --data-artifact lab-data")
		return exUsage
	}
	if !strings.HasPrefix(*systemArtifact, "lab-") || !strings.HasPrefix(*dataArtifact, "lab-") {
		fmt.Fprintln(stderr, "lab installer accepts only lab-* artifact IDs")
		return exUsage
	}
	publicKey, err := hex.DecodeString(*publicKeyHex)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		fmt.Fprintln(stderr, "public key must be a 32-byte Ed25519 key encoded as hex")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return exSoftware
	}
	resources := cfg.Engines[*candidate]
	system := filepath.Join(dataDir(), "engines", *candidate, "system.qcow2")
	data := filepath.Join(dataDir(), "engines", *candidate, "data.qcow2")
	if _, err := os.Stat(system); err == nil {
		fmt.Fprintln(stderr, "existing system image must not be overwritten")
		return exUsage
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "inspect system image: %v\n", err)
		return exSoftware
	}
	if _, err := os.Stat(data); err == nil {
		fmt.Fprintln(stderr, "existing data image must not be overwritten")
		return exUsage
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "inspect data image: %v\n", err)
		return exSoftware
	}
	downloader := artifact.Downloader{}
	if _, err := artifact.DownloadVerifiedLabArtifact(context.Background(), downloader, *manifestPath, publicKey, *systemArtifact, system); err != nil {
		fmt.Fprintf(stderr, "install system image: %v\n", err)
		return exSoftware
	}
	if _, err := artifact.DownloadVerifiedLabArtifact(context.Background(), downloader, *manifestPath, publicKey, *dataArtifact, data); err != nil {
		fmt.Fprintf(stderr, "install data image: %v\n", err)
		return exSoftware
	}
	target, err := engine.NewLocalCandidateFactoryWithProbeAndResources(dataDir(), device, "", map[string]engine.BackendResources{
		*candidate: {CPUs: resources.CPUs, MemoryMiB: resources.MemoryMiB},
	})(context.Background(), engine.Candidate{ID: *candidate, Kind: engine.KindQEMUSystem})
	if err != nil {
		fmt.Fprintf(stderr, "initialize backend: %v\n", err)
		return exSoftware
	}
	adapterTarget, ok := target.(engine.AdapterTarget)
	if !ok || adapterTarget.Adapter == nil {
		fmt.Fprintln(stderr, "backend target does not expose an initializer")
		return exSoftware
	}
	if err := adapterTarget.Adapter.Initialize(context.Background()); err != nil {
		fmt.Fprintf(stderr, "initialize backend state: %v\n", err)
		return exSoftware
	}
	fmt.Fprintf(stdout, "Installed verified lab images for %s\nSystem: %s\nData:   %s\nQualification remains required before Podman use.\n", *candidate, system, data)
	return 0
}
