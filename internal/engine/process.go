package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// ProcessExecutor is the production bridge from an adapter to a real guest
// command. It never invokes a shell and preserves stdout, stderr, and the
// remote process exit code separately.
type ProcessExecutor struct {
	Env        []string
	WorkingDir string
}

func (executor ProcessExecutor) Execute(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	if len(request.Args) == 0 || request.Args[0] == "" {
		return ExecutionResult{}, errors.New("execution requires a command vector")
	}
	started := time.Now()
	command := exec.CommandContext(ctx, request.Args[0], request.Args[1:]...)
	command.Stdin = request.Stdin
	command.Dir = request.WorkingDir
	command.Env = append(os.Environ(), executor.Env...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := ExecutionResult{ExitCode: 0, Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started)}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		result.ExitCode = 75
		result.Stderr = fmt.Sprintf("command context ended: %v", ctx.Err())
		return result, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result, nil
	}
	return result, err
}
