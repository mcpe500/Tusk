package engine

import (
	"context"
	"testing"
)

type flakyTarget struct {
	recordingExecutor
	restarts int
}

func (target *flakyTarget) Restart(context.Context) error {
	target.restarts++
	return nil
}

func (target *flakyTarget) Measure(context.Context) (Metrics, error) {
	return Metrics{InteractiveLatencyMillis: 1, WorkloadThroughput: 1, PeakMemoryMiB: 1, StartupShutdownMillis: 1, PersistentStorageMiB: 1, ThermalRetention: 1}, nil
}

func TestRunReliabilityRecordsFailuresAndRatio(t *testing.T) {
	t.Parallel()
	target := &flakyTarget{recordingExecutor: recordingExecutor{failOn: "podman run"}}
	result, err := RunReliability(context.Background(), target, ReliabilityOptions{ProbeImage: "image@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Repetitions: 2})
	if err != nil {
		t.Fatalf("RunReliability() error = %v", err)
	}
	if result.Ratio != 0 || len(result.Attempts) != 2 || target.restarts != 2 {
		t.Fatalf("unexpected reliability result: %#v", result)
	}
}
