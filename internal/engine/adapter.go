package engine

import (
	"context"
	"io"

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
	ExitCode int
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
