package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxEvidenceOutput = 4096

// ExecutionRequest is the adapter-neutral form of one guest command.
// Args are kept as a vector so callers never need to construct a shell string.
type ExecutionRequest struct {
	Args       []string
	Stdin      io.Reader
	WorkingDir string
}

type ExecutionResult struct {
	ExitCode int           `json:"exitCode"`
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	Duration time.Duration `json:"duration"`
}

// Executor is implemented by every real backend adapter. A successful return
// is not qualification; the command's exit code and output are recorded first.
type Executor interface {
	Execute(context.Context, ExecutionRequest) (ExecutionResult, error)
}

type Lifecycle interface {
	Restart(context.Context) error
}

type MetricsProvider interface {
	Measure(context.Context) (Metrics, error)
}

type QualificationTarget interface {
	Executor
	Lifecycle
	MetricsProvider
}

type ConformanceOptions struct {
	CandidateID string
	ProbeImage  string
	RunID       string
	Timeout     time.Duration
}

type CheckResult struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Passed      bool          `json:"passed"`
	ExitCode    int           `json:"exitCode"`
	Output      string        `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

type ConformanceResult struct {
	Version       int           `json:"version"`
	StartedAt     time.Time     `json:"startedAt"`
	FinishedAt    time.Time     `json:"finishedAt"`
	Checks        []CheckResult `json:"checks"`
	Passed        bool          `json:"passed"`
	CleanupErrors []string      `json:"cleanupErrors,omitempty"`
}

// RunConformance executes the complete mandatory suite. It deliberately uses
// real Podman commands and fails closed when the probe image is not digest
// pinned. The caller can persist the returned evidence for later inspection.
func RunConformance(ctx context.Context, executor Executor, lifecycle Lifecycle, contract ConformanceContract, options ConformanceOptions) (ConformanceResult, error) {
	if executor == nil {
		return ConformanceResult{}, errors.New("conformance executor is required")
	}
	if !containsAtSHA256(options.ProbeImage) {
		return ConformanceResult{}, errors.New("probe image must be pinned by digest (image@sha256:...)")
	}
	if options.RunID == "" {
		return ConformanceResult{}, errors.New("conformance run ID is required")
	}
	if len(contract.Checks) == 0 {
		return ConformanceResult{}, errors.New("conformance contract has no checks")
	}
	if err := validateConformanceContract(contract); err != nil {
		return ConformanceResult{}, err
	}
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Minute
	}

	started := time.Now().UTC()
	result := ConformanceResult{Version: contract.Version, StartedAt: started}
	resources := probeResources(options.RunID)
	podman := func(args ...string) []string { return append([]string{"podman"}, args...) }
	run := func(request ExecutionRequest) ExecutionResult {
		commandContext := ctx
		cancel := func() {}
		if options.Timeout > 0 {
			commandContext, cancel = context.WithTimeout(ctx, options.Timeout)
		}
		defer cancel()
		value, err := executor.Execute(commandContext, request)
		if err != nil {
			if commandContext.Err() != nil {
				return ExecutionResult{ExitCode: 75, Stderr: fmt.Sprintf("command timeout after %s: %v", options.Timeout, commandContext.Err())}
			}
			return ExecutionResult{ExitCode: 70, Stderr: err.Error()}
		}
		return value
	}
	contractByID := make(map[string]ConformanceCheck, len(contract.Checks))
	for _, contractCheck := range contract.Checks {
		contractByID[contractCheck.ID] = contractCheck
	}
	getCheck := func(id string) (ConformanceCheck, error) {
		contractCheck, ok := contractByID[id]
		if !ok {
			return ConformanceCheck{}, fmt.Errorf("conformance contract is missing %q", id)
		}
		return contractCheck, nil
	}
	check := func(id string, requests []ExecutionRequest, predicates []func(ExecutionResult) error) error {
		contractCheck, err := getCheck(id)
		if err != nil {
			return err
		}
		startedCheck := time.Now()
		checkResult := CheckResult{
			ID:          contractCheck.ID,
			Description: contractCheck.Description,
			Required:    contractCheck.Required,
			Duration:    time.Since(startedCheck),
		}
		var outputs []string
		checkResult.Passed = true
		for index, request := range requests {
			value := run(request)
			checkResult.ExitCode = value.ExitCode
			if output := evidenceOutput(value.Stdout, value.Stderr); output != "" {
				outputs = append(outputs, output)
			}
			if value.ExitCode != 0 {
				checkResult.Passed = false
				if checkResult.Error == "" {
					checkResult.Error = fmt.Sprintf("command exited %d", value.ExitCode)
				}
				continue
			}
			if index < len(predicates) && predicates[index] != nil {
				if predicateError := predicates[index](value); predicateError != nil {
					checkResult.Passed = false
					checkResult.Error = predicateError.Error()
				}
			}
		}
		checkResult.Output = strings.Join(outputs, "\n")
		checkResult.Duration = time.Since(startedCheck)
		result.Checks = append(result.Checks, checkResult)
		return nil
	}

	if err := check("podman.info", []ExecutionRequest{{Args: podman("info", "--format", "{{.Version.Version}}")}}, nil); err != nil {
		return result, err
	}
	if err := check("image.pull", []ExecutionRequest{{Args: podman("pull", options.ProbeImage)}}, nil); err != nil {
		return result, err
	}
	if err := check("container.run", []ExecutionRequest{{Args: podman("run", "--rm", options.ProbeImage, "sh", "-c", "printf tusk-probe")}}, []func(ExecutionResult) error{func(value ExecutionResult) error {
		if !strings.Contains(value.Stdout, "tusk-probe") {
			return errors.New("probe output did not contain tusk-probe")
		}
		return nil
	}}); err != nil {
		return result, err
	}
	if err := check("container.exec", []ExecutionRequest{{Args: podman("run", "-d", "--name", resources.container, options.ProbeImage, "sh", "-c", "sleep 30")}, {Args: podman("exec", resources.container, "true")}}, nil); err != nil {
		return result, err
	}
	if err := check("network.dns", []ExecutionRequest{{Args: podman("run", "--rm", options.ProbeImage, "sh", "-c", "getent hosts example.com")}}, nil); err != nil {
		return result, err
	}
	if err := check("network.tcp", []ExecutionRequest{{Args: podman("run", "--rm", options.ProbeImage, "sh", "-c", "wget -q -O - https://example.com >/dev/null")}}, nil); err != nil {
		return result, err
	}
	if err := check("volume.persistence", []ExecutionRequest{{Args: podman("volume", "create", resources.volume)}, {Args: podman("run", "--rm", "-v", resources.volume+":/data", options.ProbeImage, "sh", "-c", "printf persisted >/data/value")}, {Args: podman("run", "--rm", "-v", resources.volume+":/data", options.ProbeImage, "sh", "-c", "test \"$(cat /data/value)\" = persisted")}}, nil); err != nil {
		return result, err
	}
	containerfile := []byte("FROM " + options.ProbeImage + "\nRUN printf built >/tusk-build\nCMD [\"cat\", \"/tusk-build\"]\n")
	if err := check("build.containerfile", []ExecutionRequest{{Args: podman("build", "-t", resources.image, "-"), Stdin: bytes.NewReader(containerfile)}}, nil); err != nil {
		return result, err
	}
	restartCheck, err := getCheck("lifecycle.restart")
	if err != nil {
		return result, err
	}
	if lifecycle == nil {
		result.Checks = append(result.Checks, CheckResult{ID: restartCheck.ID, Description: restartCheck.Description, Required: restartCheck.Required, ExitCode: 70, Error: "backend lifecycle restart is unavailable"})
	} else {
		startedRestart := time.Now()
		restartContext := ctx
		cancelRestart := func() {}
		if options.Timeout > 0 {
			restartContext, cancelRestart = context.WithTimeout(ctx, options.Timeout)
		}
		err := lifecycle.Restart(restartContext)
		cancelRestart()
		checkResult := CheckResult{ID: restartCheck.ID, Description: restartCheck.Description, Required: restartCheck.Required, Duration: time.Since(startedRestart), Passed: err == nil}
		if err != nil {
			checkResult.Error = err.Error()
		}
		result.Checks = append(result.Checks, checkResult)
	}
	cleanupErrors := runCleanup(ctx, executor, podman, resources, options.Timeout)
	cleanupCheck, err := getCheck("cleanup")
	if err != nil {
		return result, err
	}
	cleanupResult := CheckResult{ID: cleanupCheck.ID, Description: cleanupCheck.Description, Required: cleanupCheck.Required, Passed: len(cleanupErrors) == 0}
	if len(cleanupErrors) > 0 {
		cleanupResult.Error = strings.Join(cleanupErrors, "; ")
	}
	result.Checks = append(result.Checks, cleanupResult)
	result.CleanupErrors = cleanupErrors
	result.FinishedAt = time.Now().UTC()
	result.Passed = conformancePassed(result, contract)
	return result, nil
}

type probeResourceNames struct {
	container string
	volume    string
	image     string
}

func probeResources(runID string) probeResourceNames {
	clean := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(runID)
	return probeResourceNames{
		container: "tusk-probe-" + clean,
		volume:    "tusk-probe-" + clean,
		image:     "tusk-probe-build-" + clean,
	}
}

func runCleanup(ctx context.Context, executor Executor, podman func(...string) []string, resources probeResourceNames, timeout time.Duration) []string {
	var failures []string
	cleanupParent := context.WithoutCancel(ctx)
	for _, resource := range [][]string{
		podman("rm", "-f", resources.container),
		podman("rm", "-f", resources.image),
		podman("rmi", "-f", resources.image),
		podman("volume", "rm", resources.volume),
	} {
		commandContext := cleanupParent
		cancel := func() {}
		if timeout > 0 {
			commandContext, cancel = context.WithTimeout(cleanupParent, timeout)
		}
		value, err := executor.Execute(commandContext, ExecutionRequest{Args: resource})
		cancel()
		if err != nil {
			failures = append(failures, err.Error())
		} else if value.ExitCode != 0 && !strings.Contains(strings.ToLower(value.Stderr), "no such") {
			failures = append(failures, fmt.Sprintf("%s exited %d: %s", resource[1], value.ExitCode, evidenceOutput(value.Stdout, value.Stderr)))
		}
	}
	return failures
}

func conformancePassed(result ConformanceResult, contract ConformanceContract) bool {
	counts := make(map[string]int, len(result.Checks))
	passed := make(map[string]bool, len(result.Checks))
	for _, check := range result.Checks {
		counts[check.ID]++
		passed[check.ID] = passed[check.ID] || check.Passed
	}
	for _, check := range contract.Checks {
		if check.Required && (counts[check.ID] == 0 || !passed[check.ID]) {
			return false
		}
	}
	return len(result.CleanupErrors) == 0
}

func validateConformanceContract(contract ConformanceContract) error {
	if contract.Version < 1 {
		return errors.New("conformance contract version must be positive")
	}
	seen := make(map[string]struct{}, len(contract.Checks))
	for _, check := range contract.Checks {
		if check.ID == "" || check.Description == "" {
			return errors.New("conformance checks require an ID and description")
		}
		if _, exists := seen[check.ID]; exists {
			return fmt.Errorf("duplicate conformance check %q", check.ID)
		}
		seen[check.ID] = struct{}{}
	}
	if len(contract.Checks) < 10 {
		return fmt.Errorf("conformance contract has %d checks; version 1 requires at least 10", len(contract.Checks))
	}
	return nil
}

func evidenceOutput(stdout, stderr string) string {
	output := strings.TrimSpace(stdout)
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(stderr)
	}
	if len(output) > maxEvidenceOutput {
		return output[:maxEvidenceOutput] + "..."
	}
	return output
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
