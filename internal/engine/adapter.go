package engine

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/mcpe500/Tusk/internal/platform"
)

type LifecycleState string

const (
	StateUninitialized LifecycleState = "uninitialized"
	StateStopped       LifecycleState = "stopped"
	StateStarting      LifecycleState = "starting"
	StateProvisioning  LifecycleState = "provisioning"
	StateReady         LifecycleState = "ready"
	StateDegraded      LifecycleState = "degraded"
	StateStopping      LifecycleState = "stopping"
	StateError         LifecycleState = "error"
)

type Preflight struct {
	Applicable bool     `json:"applicable"`
	Available  bool     `json:"available"`
	Reasons    []string `json:"reasons,omitempty"`
}

type RuntimeStatus struct {
	State   LifecycleState `json:"state"`
	Healthy bool           `json:"healthy"`
	Detail  string         `json:"detail,omitempty"`
}

type Command struct {
	Args       []string
	Env        []string
	WorkingDir string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	TTY        bool
}

type CommandResult struct {
	ExitCode      int
	Stdout        string
	Stderr        string
	Duration      time.Duration
	PeakMemoryMiB float64
}

// AdapterTarget exposes an Adapter through the qualification harness while
// preserving streams and measurements. It is the only bridge used by tuning.
type AdapterTarget struct {
	Adapter Adapter
}

func (target AdapterTarget) Execute(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	if target.Adapter == nil {
		return ExecutionResult{}, errors.New("adapter target is not configured")
	}
	var stdout, stderr strings.Builder
	result, err := target.Adapter.Execute(ctx, Command{Args: request.Args, Stdin: request.Stdin, Stdout: &stdout, Stderr: &stderr, WorkingDir: request.WorkingDir})
	if err != nil {
		return ExecutionResult{}, err
	}
	return ExecutionResult{ExitCode: result.ExitCode, Stdout: stdout.String(), Stderr: stderr.String(), Duration: result.Duration}, nil
}

func (target AdapterTarget) Restart(ctx context.Context) error {
	if target.Adapter == nil {
		return errors.New("adapter target is not configured")
	}
	if err := target.Adapter.Stop(ctx, false); err != nil {
		return err
	}
	return target.Adapter.Start(ctx)
}

func (target AdapterTarget) Measure(ctx context.Context) (Metrics, error) {
	if target.Adapter == nil {
		return Metrics{}, errors.New("adapter target is not configured")
	}
	return target.Adapter.Measure(ctx)
}

// Adapter is the evidence and lifecycle boundary every backend must implement.
// Implementations are added only when their real conformance work begins.
type Adapter interface {
	Candidate() Candidate
	Preflight(context.Context, platform.Device) (Preflight, error)
	Initialize(context.Context) error
	Start(context.Context) error
	Stop(context.Context, bool) error
	Status(context.Context) (RuntimeStatus, error)
	Execute(context.Context, Command) (CommandResult, error)
	APIEndpoint(context.Context) (string, error)
	Health(context.Context) (RuntimeStatus, error)
	Measure(context.Context) (Metrics, error)
	Cleanup(context.Context) error
}
