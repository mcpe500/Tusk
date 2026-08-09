package engine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ReliabilityOptions struct {
	ProbeImage  string
	Repetitions int
	Timeout     time.Duration
}

type ReliabilityAttempt struct {
	Number   int           `json:"number"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

type ReliabilityResult struct {
	Repetitions int                  `json:"repetitions"`
	Successes   int                  `json:"successes"`
	Ratio       float64              `json:"ratio"`
	Attempts    []ReliabilityAttempt `json:"attempts"`
}

func RunReliability(ctx context.Context, target QualificationTarget, options ReliabilityOptions) (ReliabilityResult, error) {
	if target == nil {
		return ReliabilityResult{}, errors.New("reliability target is required")
	}
	if !containsAtSHA256(options.ProbeImage) {
		return ReliabilityResult{}, errors.New("reliability probe image must be pinned by digest")
	}
	if options.Repetitions < 1 {
		return ReliabilityResult{}, errors.New("reliability repetitions must be positive")
	}
	result := ReliabilityResult{Repetitions: options.Repetitions, Attempts: make([]ReliabilityAttempt, 0, options.Repetitions)}
	for i := 0; i < options.Repetitions; i++ {
		started := time.Now()
		attempt := ReliabilityAttempt{Number: i + 1}
		attemptContext := ctx
		cancel := func() {}
		if options.Timeout > 0 {
			attemptContext, cancel = context.WithTimeout(ctx, options.Timeout)
		}
		if err := target.Restart(attemptContext); err != nil {
			attempt.Error = fmt.Sprintf("restart: %v", err)
		} else {
			value, err := target.Execute(attemptContext, ExecutionRequest{Args: []string{"podman", "run", "--rm", options.ProbeImage, "true"}})
			if err != nil {
				attempt.Error = err.Error()
			} else if value.ExitCode != 0 {
				attempt.Error = fmt.Sprintf("podman exited %d: %s", value.ExitCode, evidenceOutput(value.Stdout, value.Stderr))
			} else {
				attempt.Passed = true
				result.Successes++
			}
		}
		cancel()
		attempt.Duration = time.Since(started)
		result.Attempts = append(result.Attempts, attempt)
	}
	result.Ratio = float64(result.Successes) / float64(result.Repetitions)
	return result, nil
}
