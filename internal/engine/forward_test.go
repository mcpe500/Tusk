package engine

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

type forwardAdapter struct {
	state  RuntimeStatus
	starts int
	args   []string
	exit   int
}

func (adapter *forwardAdapter) Candidate() Candidate { return Candidate{ID: "test"} }
func (adapter *forwardAdapter) Preflight(context.Context, platform.Device) (Preflight, error) {
	return Preflight{}, nil
}
func (adapter *forwardAdapter) Initialize(context.Context) error { return nil }
func (adapter *forwardAdapter) Start(context.Context) error {
	adapter.starts++
	adapter.state = RuntimeStatus{State: StateReady, Healthy: true}
	return nil
}
func (adapter *forwardAdapter) Stop(context.Context, bool) error {
	adapter.state.State = StateStopped
	return nil
}
func (adapter *forwardAdapter) Status(context.Context) (RuntimeStatus, error) {
	return adapter.state, nil
}
func (adapter *forwardAdapter) Execute(_ context.Context, command Command) (CommandResult, error) {
	adapter.args = command.Args
	if command.Stdout != nil {
		_, _ = io.WriteString(command.Stdout, "forwarded")
	}
	return CommandResult{ExitCode: adapter.exit}, nil
}
func (adapter *forwardAdapter) APIEndpoint(context.Context) (string, error) {
	return "unix:///tmp/podman.sock", nil
}
func (adapter *forwardAdapter) Health(context.Context) (RuntimeStatus, error) {
	return adapter.state, nil
}
func (adapter *forwardAdapter) Measure(context.Context) (Metrics, error) {
	return Metrics{}, errors.New("not measured")
}
func (adapter *forwardAdapter) Cleanup(context.Context) error { return nil }

func TestForwardPodmanAutoStartsAndPreservesExitCode(t *testing.T) {
	t.Parallel()
	adapter := &forwardAdapter{state: RuntimeStatus{State: StateStopped}}
	var stdout bytes.Buffer
	result, err := ForwardPodman(context.Background(), adapter, []string{"run", "--rm", "image"}, nil, &stdout, &bytes.Buffer{}, false, ForwardOptions{AutoStart: true})
	if err != nil || result.ExitCode != 0 || adapter.starts != 1 || stdout.String() != "forwarded" {
		t.Fatalf("ForwardPodman() = %#v, %v, starts=%d stdout=%q", result, err, adapter.starts, stdout.String())
	}
	if len(adapter.args) != 3 || adapter.args[0] != "run" {
		t.Fatalf("forwarded args = %#v", adapter.args)
	}
}

func TestForwardPodmanReturnsRecoveryWithoutAutoStart(t *testing.T) {
	t.Parallel()
	_, err := ForwardPodman(context.Background(), &forwardAdapter{state: RuntimeStatus{State: StateStopped}}, []string{"ps"}, nil, io.Discard, io.Discard, false, ForwardOptions{})
	if err == nil || !errors.Is(err, ErrNoQualifiedBackend) && !bytes.Contains([]byte(err.Error()), []byte(RecoveryTuneCommand)) {
		t.Fatalf("ForwardPodman() error = %v", err)
	}
}

func TestValidateWorkingDirectoryRejectsUnsharedBuildContext(t *testing.T) {
	t.Parallel()
	if err := ValidateWorkingDirectory("/tmp/project", "/tmp/workspaces", []string{"build", "."}); err == nil {
		t.Fatal("unshared build context was accepted")
	}
	if err := ValidateWorkingDirectory("/tmp/workspaces/project", "/tmp/workspaces", []string{"build", "."}); err != nil {
		t.Fatalf("shared build context rejected: %v", err)
	}
}
