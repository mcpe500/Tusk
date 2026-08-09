package engine

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProcessExecutorPreservesStreamsAndExitCode(t *testing.T) {
	t.Parallel()
	result, err := (ProcessExecutor{}).Execute(context.Background(), ExecutionRequest{Args: []string{"sh", "-c", "printf out; printf err >&2; exit 3"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 3 || result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessExecutorTimesOutWithoutReportingSuccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result, err := (ProcessExecutor{}).Execute(ctx, ExecutionRequest{Args: []string{"sh", "-c", "sleep 1"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode == 0 || !strings.Contains(result.Stderr, "deadline") {
		t.Fatalf("timeout result = %#v", result)
	}
}
