package engine

import (
	"context"
	"strings"
	"testing"
)

type recordingExecutor struct {
	requests []ExecutionRequest
	failOn   string
}

func (e *recordingExecutor) Execute(_ context.Context, request ExecutionRequest) (ExecutionResult, error) {
	e.requests = append(e.requests, request)
	joined := strings.Join(request.Args, " ")
	if e.failOn != "" && strings.Contains(joined, e.failOn) {
		return ExecutionResult{ExitCode: 1, Stderr: "induced failure"}, nil
	}
	output := ""
	if strings.Contains(joined, "printf tusk-probe") {
		output = "tusk-probe"
	}
	return ExecutionResult{ExitCode: 0, Stdout: output}, nil
}

type recordingLifecycle struct{ restarts int }

func (l *recordingLifecycle) Restart(context.Context) error {
	l.restarts++
	return nil
}

func TestRunConformanceRequiresDigestPinnedImage(t *testing.T) {
	t.Parallel()
	_, err := RunConformance(context.Background(), &recordingExecutor{}, &recordingLifecycle{}, ConformanceContractV1(), ConformanceOptions{CandidateID: "test", ProbeImage: "alpine:latest", RunID: "run"})
	if err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("RunConformance() error = %v, want digest validation", err)
	}
}

func TestRunConformanceRunsEveryRequiredCheckAndCleansResources(t *testing.T) {
	t.Parallel()
	executor := &recordingExecutor{}
	lifecycle := &recordingLifecycle{}
	result, err := RunConformance(context.Background(), executor, lifecycle, ConformanceContractV1(), ConformanceOptions{
		CandidateID: "test",
		ProbeImage:  "example/alpine@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RunID:       "unit-test",
	})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("conformance unexpectedly failed: %#v", result)
	}
	if lifecycle.restarts != 1 {
		t.Fatalf("restarts = %d, want 1", lifecycle.restarts)
	}
	for _, check := range ConformanceContractV1().Checks {
		found := false
		for _, actual := range result.Checks {
			if actual.ID == check.ID && actual.Passed {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("required check %q was not passed: %#v", check.ID, result.Checks)
		}
	}
	if len(executor.requests) < 10 {
		t.Fatalf("executed %d commands, want complete suite", len(executor.requests))
	}
}

func TestRunConformanceNeverMarksFailedCommandQualified(t *testing.T) {
	t.Parallel()
	executor := &recordingExecutor{failOn: "podman pull"}
	result, err := RunConformance(context.Background(), executor, &recordingLifecycle{}, ConformanceContractV1(), ConformanceOptions{
		CandidateID: "test",
		ProbeImage:  "example/alpine@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RunID:       "unit-test",
	})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if result.Passed {
		t.Fatal("failed pull was marked as passed")
	}
}
