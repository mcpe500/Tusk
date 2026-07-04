package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/internal/build"
	"github.com/tusk/tusk/internal/image"
)

// runBuild implements `tusk build [options] <context>`.
// It builds a Dockerfile into a local OCI image.
func runBuild() {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	tag := fs.String("t", "", "Image tag (e.g. myapp:latest)")
	dockerfile := fs.String("f", "Dockerfile", "Dockerfile path (relative to context)")
	var buildArgsList []string
	fs.Func("build-arg", "Build-time variable (key=value)", func(s string) error {
		buildArgsList = append(buildArgsList, s)
		return nil
	})
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Println("Usage: tusk build [options] <context>")
		fmt.Println("Options:")
		fs.PrintDefaults()
		os.Exit(1)
	}

	contextDir := args[0]
	if !filepath.IsAbs(contextDir) {
		abs, _ := filepath.Abs(contextDir)
		contextDir = abs
	}

	if info, err := os.Stat(contextDir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: context directory not found: %s\n", contextDir)
		os.Exit(1)
	}

	imageRef := *tag
	if imageRef == "" {
		// Default tag based on context directory name.
		imageRef = "tusk-local/" + filepath.Base(contextDir) + ":latest"
	}

	// Parse build args.
	buildArgs := make(map[string]string)
	for _, arg := range buildArgsList {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			buildArgs[parts[0]] = parts[1]
		}
	}

	store := image.New(filepath.Join(tuskDir, "images"))
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: init image store: %v\n", err)
		os.Exit(1)
	}

	engine := build.NewEngine(store, tuskDir)
	engine.SetVerbose(true)

	fmt.Printf("Building %s from %s\n", imageRef, contextDir)
	manifest, err := engine.Build(context.Background(), build.BuildOptions{
		ContextDir: contextDir,
		Dockerfile: *dockerfile,
		ImageTag:   imageRef,
		BuildArgs:  buildArgs,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: build failed: %v\n", err)
		os.Exit(1)
	}

	totalSize := int64(0)
	for _, layer := range manifest.Layers {
		totalSize += layer.Size
	}
	fmt.Printf("Successfully built %s (%d layers, %s)\n", imageRef, len(manifest.Layers), humanSize(totalSize))
}
