package engine

import (
	"context"
	"testing"
)

type benchmarkTarget struct {
	recordingExecutor
}

func (benchmarkTarget) Restart(context.Context) error { return nil }

func (benchmarkTarget) Measure(context.Context) (Metrics, error) {
	return Metrics{
		InteractiveLatencyMillis: 10,
		WorkloadThroughput:       5,
		PeakMemoryMiB:            64,
		StartupShutdownMillis:    20,
		PersistentStorageMiB:     4,
		ThermalRetention:         1,
	}, nil
}

func TestRunBenchmarkRequiresAdapterMeasurements(t *testing.T) {
	t.Parallel()
	target := &benchmarkTarget{}
	result, err := RunBenchmark(context.Background(), target, BenchmarkContractV1(), BenchmarkOptions{
		ProbeImage:  "example/alpine@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Mode:        "quick",
		Repetitions: 2,
	})
	if err != nil {
		t.Fatalf("RunBenchmark() error = %v", err)
	}
	if len(result.Samples) != 2 || result.Metrics.PeakMemoryMiB != 64 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}

func TestContainsAtSHA256RejectsMalformedDigests(t *testing.T) {
	t.Parallel()
	if !containsAtSHA256("image@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
		t.Fatal("valid digest was rejected")
	}
	if containsAtSHA256("image@sha256:ABC") || containsAtSHA256("image:latest") {
		t.Fatal("malformed digest was accepted")
	}
}
