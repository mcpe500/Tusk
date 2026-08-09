package engine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type BenchmarkOptions struct {
	ProbeImage  string
	Mode        string
	Repetitions int
	Warmup      int
	Timeout     time.Duration
}

type BenchmarkSample struct {
	Iteration int           `json:"iteration"`
	Duration  time.Duration `json:"duration"`
	Passed    bool          `json:"passed"`
	Error     string        `json:"error,omitempty"`
}

type BenchmarkResult struct {
	Version     int               `json:"version"`
	Mode        string            `json:"mode"`
	Repetitions int               `json:"repetitions"`
	Warmup      int               `json:"warmup"`
	Samples     []BenchmarkSample `json:"samples"`
	Metrics     Metrics           `json:"metrics"`
}

// RunBenchmark measures actual Podman round trips and obtains resource metrics
// from the backend adapter. It refuses to invent memory, storage, or thermal
// measurements when an adapter cannot provide them.
func RunBenchmark(ctx context.Context, target QualificationTarget, contract BenchmarkContract, options BenchmarkOptions) (BenchmarkResult, error) {
	if target == nil {
		return BenchmarkResult{}, errors.New("benchmark target is required")
	}
	if !containsAtSHA256(options.ProbeImage) {
		return BenchmarkResult{}, errors.New("benchmark probe image must be pinned by digest")
	}
	if options.Mode != "quick" && options.Mode != "thorough" {
		return BenchmarkResult{}, fmt.Errorf("unsupported benchmark mode %q", options.Mode)
	}
	if options.Repetitions < 1 {
		return BenchmarkResult{}, errors.New("benchmark repetitions must be positive")
	}
	if options.Warmup < 0 {
		return BenchmarkResult{}, errors.New("benchmark warmup must not be negative")
	}

	for i := 0; i < options.Warmup; i++ {
		commandContext := ctx
		cancel := func() {}
		if options.Timeout > 0 {
			commandContext, cancel = context.WithTimeout(ctx, options.Timeout)
		}
		_, err := target.Execute(commandContext, ExecutionRequest{Args: []string{"podman", "run", "--rm", options.ProbeImage, "true"}})
		cancel()
		if err != nil {
			return BenchmarkResult{}, fmt.Errorf("benchmark warmup %d: %w", i+1, err)
		}
	}
	result := BenchmarkResult{Version: contract.Version, Mode: options.Mode, Repetitions: options.Repetitions, Warmup: options.Warmup, Samples: make([]BenchmarkSample, 0, options.Repetitions)}
	for i := 0; i < options.Repetitions; i++ {
		started := time.Now()
		commandContext := ctx
		cancel := func() {}
		if options.Timeout > 0 {
			commandContext, cancel = context.WithTimeout(ctx, options.Timeout)
		}
		value, err := target.Execute(commandContext, ExecutionRequest{Args: []string{"podman", "run", "--rm", options.ProbeImage, "true"}})
		cancel()
		sample := BenchmarkSample{Iteration: i + 1, Duration: time.Since(started), Passed: err == nil && value.ExitCode == 0}
		if err != nil {
			sample.Error = err.Error()
		} else if value.ExitCode != 0 {
			sample.Error = fmt.Sprintf("podman exited %d: %s", value.ExitCode, evidenceOutput(value.Stdout, value.Stderr))
		}
		result.Samples = append(result.Samples, sample)
	}
	metrics, err := target.Measure(ctx)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("collect benchmark metrics: %w", err)
	}
	if !validMetrics(metrics) {
		return BenchmarkResult{}, errors.New("adapter returned incomplete benchmark metrics")
	}
	result.Metrics = metrics
	return result, nil
}

func containsAtSHA256(image string) bool {
	index := -1
	for i := 0; i+8 <= len(image); i++ {
		if image[i:i+8] == "@sha256:" {
			index = i
		}
	}
	if index < 0 || len(image)-index-8 != 64 {
		return false
	}
	for _, char := range image[index+8:] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}
