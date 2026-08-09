package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcpe500/Tusk/internal/state"
)

const ComposeMetadataSchemaVersion = 1

type ComposeProvider struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ComposeMetadata struct {
	SchemaVersion int             `json:"schemaVersion"`
	Provider      ComposeProvider `json:"provider"`
}

var composeCommands = map[string]bool{
	"config": true, "build": true, "pull": true, "up": true, "down": true,
	"ps": true, "logs": true, "exec": true, "run": true, "stop": true,
	"start": true, "restart": true, "rm": true,
}

func ValidateComposeProvider(provider ComposeProvider) error {
	if provider.Name == "" || provider.Version == "" {
		return errors.New("Compose provider name and pinned version are required")
	}
	if provider.Name != "podman-compose" {
		return fmt.Errorf("unsupported Compose provider %q", provider.Name)
	}
	return nil
}

func SaveComposeMetadata(path string, metadata ComposeMetadata) error {
	if metadata.SchemaVersion != ComposeMetadataSchemaVersion {
		return errors.New("unsupported Compose metadata schema")
	}
	if err := ValidateComposeProvider(metadata.Provider); err != nil {
		return err
	}
	return state.WriteJSON(path, metadata, 0o600)
}

func LoadComposeMetadata(path string) (ComposeMetadata, error) {
	var metadata ComposeMetadata
	if err := state.ReadJSON(path, &metadata); err != nil {
		return ComposeMetadata{}, err
	}
	if metadata.SchemaVersion != ComposeMetadataSchemaVersion {
		return ComposeMetadata{}, errors.New("unsupported Compose metadata schema")
	}
	if err := ValidateComposeProvider(metadata.Provider); err != nil {
		return ComposeMetadata{}, err
	}
	return metadata, nil
}

func ComposeArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, errors.New("podman compose requires a subcommand")
	}
	command := args[0]
	if command == "help" || command == "--help" || command == "-h" {
		return append([]string{"compose"}, args...), nil
	}
	if !composeCommands[command] {
		return nil, fmt.Errorf("unsupported Compose subcommand %q; provider warnings/errors are not suppressed", command)
	}
	return append([]string{"compose"}, args...), nil
}

func ComposeFeatureMatrix() map[string]string {
	return map[string]string{
		"config":  "tested provider passthrough",
		"build":   "tested provider passthrough",
		"pull":    "tested provider passthrough",
		"up":      "tested provider passthrough plus TCP reconciliation",
		"down":    "tested provider passthrough plus project cleanup",
		"ps":      "tested provider passthrough",
		"logs":    "tested provider passthrough",
		"exec":    "tested provider passthrough",
		"run":     "tested provider passthrough",
		"stop":    "tested provider passthrough",
		"start":   "tested provider passthrough",
		"restart": "tested provider passthrough",
		"rm":      "tested provider passthrough",
	}
}

func ComposeProjectDirectory(workingDirectory string) (string, error) {
	absolute, err := filepath.Abs(workingDirectory)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return "", fmt.Errorf("Compose project directory unavailable: %w", err)
	}
	return absolute, nil
}

func RunCompose(ctx context.Context, adapter Adapter, args []string, workingDirectory string, shares ShareRegistry, reconciler *PortReconciler, stdin io.Reader, stdout, stderr io.Writer, autoStart bool) (CommandResult, error) {
	translated, err := ComposeArgs(args)
	if err != nil {
		return CommandResult{}, err
	}
	projectDirectory, err := ComposeProjectDirectory(workingDirectory)
	if err != nil {
		return CommandResult{}, err
	}
	translated, guestDirectory, err := TranslatePodmanArgs(translated, projectDirectory, shares)
	if err != nil {
		return CommandResult{}, err
	}
	return ForwardPodman(ctx, adapter, translated, stdin, stdout, stderr, false, ForwardOptions{AutoStart: autoStart, WorkingDir: guestDirectory, Reconciler: reconciler})
}

func ComposeFeatureSupported(command string) bool {
	return composeCommands[strings.TrimSpace(command)]
}
