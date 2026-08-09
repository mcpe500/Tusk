package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const RecoveryTuneCommand = "tusk engine tune --mode quick --apply"

var ErrNoQualifiedBackend = errors.New("no qualified backend is selected")

type ForwardOptions struct {
	AutoStart bool
}

// ForwardPodman performs no container logic. It only gates readiness and
// forwards the exact command vector to the selected real Podman adapter.
func ForwardPodman(ctx context.Context, adapter Adapter, args []string, stdin io.Reader, stdout, stderr io.Writer, tty bool, options ForwardOptions) (CommandResult, error) {
	if adapter == nil {
		return CommandResult{}, ErrNoQualifiedBackend
	}
	if len(args) == 0 {
		return CommandResult{}, errors.New("podman requires a command")
	}
	status, err := adapter.Status(ctx)
	if err != nil {
		return CommandResult{}, fmt.Errorf("inspect selected backend: %w", err)
	}
	if status.State != StateReady || !status.Healthy {
		if !options.AutoStart {
			return CommandResult{}, fmt.Errorf("selected backend is %s; run %s", status.State, RecoveryTuneCommand)
		}
		if err := adapter.Start(ctx); err != nil {
			return CommandResult{}, fmt.Errorf("auto-start selected backend: %w", err)
		}
	}
	return adapter.Execute(ctx, Command{Args: append([]string(nil), args...), Stdin: stdin, Stdout: stdout, Stderr: stderr, TTY: tty})
}

func PodmanShimHelp(writer io.Writer, backend string) {
	fmt.Fprintf(writer, "Tusk Podman shim\nBackend: %s\nEngine: real Podman inside the selected backend\nUsage: podman <native Podman arguments>\nIf no backend is qualified, run %s\n", backend, RecoveryTuneCommand)
}

func ValidateWorkingDirectory(workdir, sharedRoot string, args []string) error {
	if !pathSensitivePodmanCommand(args) {
		return nil
	}
	workdir, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	sharedRoot, err = filepath.Abs(sharedRoot)
	if err != nil {
		return fmt.Errorf("resolve shared root: %w", err)
	}
	relative, err := filepath.Rel(sharedRoot, workdir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("working directory %q is outside the registered shared root; run `tusk share add %s`", workdir, workdir)
	}
	return nil
}

func pathSensitivePodmanCommand(args []string) bool {
	for index, arg := range args {
		if arg == "build" || arg == "compose" || arg == "-v" || arg == "--volume" || arg == "--mount" || arg == "-f" || arg == "--file" {
			return true
		}
		if index > 0 && (args[index-1] == "-v" || args[index-1] == "--volume" || args[index-1] == "--mount" || args[index-1] == "-f" || args[index-1] == "--file") {
			return true
		}
	}
	return false
}
