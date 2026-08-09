package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mcpe500/Tusk/internal/platform"
	"github.com/mcpe500/Tusk/internal/state"
)

const qemuStateSchemaVersion = 1

type QEMUConfig struct {
	CandidateID   string
	QEMUPath      string
	SystemImage   string
	DataImage     string
	RuntimeDir    string
	SharedRoot    string
	SSHPrivateKey string
	KnownHosts    string
	SSHUser       string
	SSHPort       int
	CPUs          int
	MemoryMiB     int
	Machine       string
	ProbeImage    string
	UseKVM        bool
}

type QEMUAdapter struct {
	mu            sync.Mutex
	lifecycleMu   sync.Mutex
	config        QEMUConfig
	statePath     string
	process       *exec.Cmd
	processCancel context.CancelFunc
	apiService    *exec.Cmd
	apiBridge     *exec.Cmd
	apiCancel     context.CancelFunc
	lastStart     time.Duration
	lastStop      time.Duration
}

type qemuState struct {
	SchemaVersion int            `json:"schemaVersion"`
	CandidateID   string         `json:"candidateId"`
	State         LifecycleState `json:"state"`
	PID           int            `json:"pid,omitempty"`
	StartTicks    uint64         `json:"startTicks,omitempty"`
	SSHPort       int            `json:"sshPort"`
	LastError     string         `json:"lastError,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func NewQEMUAdapter(config QEMUConfig) (*QEMUAdapter, error) {
	if !isQEMUCandidate(config.CandidateID) {
		return nil, fmt.Errorf("unsupported QEMU candidate %q", config.CandidateID)
	}
	if config.QEMUPath == "" {
		return nil, errors.New("QEMU executable is required")
	}
	if config.RuntimeDir == "" || config.SharedRoot == "" {
		return nil, errors.New("QEMU runtime and shared roots are required")
	}
	if !filepath.IsAbs(config.SystemImage) || !filepath.IsAbs(config.DataImage) || !filepath.IsAbs(config.RuntimeDir) || !filepath.IsAbs(config.SharedRoot) || !filepath.IsAbs(config.SSHPrivateKey) || !filepath.IsAbs(config.KnownHosts) {
		return nil, errors.New("QEMU paths must be absolute")
	}
	if config.SSHUser == "" {
		config.SSHUser = "tusk"
	}
	if config.SSHPort < 0 || config.SSHPort > 65535 {
		return nil, errors.New("QEMU SSH port must be zero or between 1 and 65535")
	}
	if config.CPUs < 1 || config.MemoryMiB < 128 {
		return nil, errors.New("QEMU resources are below safe minimums")
	}
	if config.Machine == "" {
		config.Machine = qemuMachine(config.CandidateID)
	}
	return &QEMUAdapter{config: config, statePath: filepath.Join(config.RuntimeDir, "state.json")}, nil
}

func (adapter *QEMUAdapter) Candidate() Candidate {
	for _, candidate := range Catalog() {
		if candidate.ID == adapter.config.CandidateID {
			return candidate
		}
	}
	return Candidate{ID: adapter.config.CandidateID, Kind: KindQEMUSystem, Architecture: qemuArchitecture(adapter.config.CandidateID)}
}

func (adapter *QEMUAdapter) Preflight(_ context.Context, device platform.Device) (Preflight, error) {
	preflight := Preflight{Applicable: qemuApplicable(adapter.config.CandidateID, device.HostArch), Available: true}
	if !preflight.Applicable {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "QEMU architecture is not applicable to this host")
	}
	if device.CPUCount > 0 && adapter.config.CPUs > device.CPUCount {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "configured QEMU CPUs exceed host CPU capacity")
	}
	if device.MemoryTotalBytes > 0 && uint64(adapter.config.MemoryMiB)*1024*1024 > device.MemoryTotalBytes*3/4 {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "configured QEMU memory exceeds the mobile safety budget")
	}
	if filepath.IsAbs(adapter.config.QEMUPath) {
		info, err := os.Stat(adapter.config.QEMUPath)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "QEMU executable is missing or not executable")
		}
	} else if _, err := exec.LookPath(adapter.config.QEMUPath); err != nil {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "QEMU executable is unavailable: "+err.Error())
	}
	for name, path := range map[string]string{"system image": adapter.config.SystemImage, "data image": adapter.config.DataImage} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, name+" is unavailable")
		}
	}
	for name, path := range map[string]string{"SSH key": adapter.config.SSHPrivateKey, "known hosts": adapter.config.KnownHosts} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, name+" is unavailable")
		} else if name == "SSH key" && info.Mode().Perm()&0o077 != 0 {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "SSH private key is accessible by other users")
		}
	}
	return preflight, nil
}

func (adapter *QEMUAdapter) Initialize(_ context.Context) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	if _, err := os.Stat(adapter.config.SystemImage); err != nil {
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateUninitialized, LastError: err.Error()})
		return fmt.Errorf("initialize QEMU system image: %w", err)
	}
	if _, err := os.Stat(adapter.config.DataImage); err != nil {
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateUninitialized, LastError: err.Error()})
		return fmt.Errorf("initialize QEMU data image: %w", err)
	}
	if err := os.MkdirAll(adapter.config.RuntimeDir, 0o700); err != nil {
		return fmt.Errorf("create QEMU runtime directory: %w", err)
	}
	if err := os.Chmod(adapter.config.RuntimeDir, 0o700); err != nil {
		return err
	}
	existing, _ := adapter.readState()
	if existing.PID > 0 && processStartTicks(existing.PID) == existing.StartTicks {
		return errors.New("cannot initialize QEMU while its recorded process is still running")
	}
	if existing.SSHPort > 0 && qemuPortAvailable(existing.SSHPort) {
		adapter.config.SSHPort = existing.SSHPort
	}
	if adapter.config.SSHPort == 0 {
		port, err := reserveLoopbackPort()
		if err != nil {
			return err
		}
		adapter.config.SSHPort = port
	}
	if !qemuPortAvailable(adapter.config.SSHPort) {
		return fmt.Errorf("QEMU SSH port %d is already in use", adapter.config.SSHPort)
	}
	if err := verifyPrivateKey(adapter.config.SSHPrivateKey); err != nil {
		return err
	}
	for _, path := range []string{adapter.qmpPath(), adapter.pidPath()} {
		_ = os.Remove(path)
	}
	return adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStopped, SSHPort: adapter.config.SSHPort})
}

func (adapter *QEMUAdapter) Start(ctx context.Context) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	started := time.Now()
	if stored, err := adapter.readState(); err == nil && stored.SSHPort > 0 {
		adapter.config.SSHPort = stored.SSHPort
	}
	if stored, err := adapter.readState(); err == nil && stored.State == StateReady && stored.PID > 0 && processStartTicks(stored.PID) == stored.StartTicks {
		if _, err := adapter.Health(ctx); err == nil {
			return nil
		}
	}
	if adapter.process != nil {
		_ = adapter.stopLocked(context.Background(), true)
	}
	if err := adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStarting, SSHPort: adapter.config.SSHPort}); err != nil {
		return err
	}
	processContext, cancel := context.WithCancel(context.Background())
	logFile, err := os.OpenFile(adapter.serialPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		cancel()
		return fmt.Errorf("open QEMU serial log: %w", err)
	}
	args := adapter.qemuArgs()
	process := exec.CommandContext(processContext, args[0], args[1:]...)
	process.Stdout = logFile
	process.Stderr = logFile
	if err := process.Start(); err != nil {
		_ = logFile.Close()
		cancel()
		return fmt.Errorf("start QEMU (serial log %s): %w", adapter.serialPath(), err)
	}
	_ = logFile.Close()
	adapter.mu.Lock()
	adapter.process = process
	adapter.processCancel = cancel
	adapter.mu.Unlock()
	if err := adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStarting, PID: process.Process.Pid, StartTicks: processStartTicks(process.Process.Pid), SSHPort: adapter.config.SSHPort}); err != nil {
		_ = adapter.stopLocked(context.Background(), true)
		return fmt.Errorf("QEMU readiness failed (serial log %s, category %s): %w", adapter.serialPath(), qemuFailureCategory(err), err)
	}
	if err := adapter.waitReady(ctx); err != nil {
		_ = adapter.stopLocked(context.Background(), true)
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateError, LastError: err.Error()})
		return fmt.Errorf("QEMU guest health failed (serial log %s): %w", adapter.serialPath(), err)
	}
	if err := adapter.startAPIBridge(ctx); err != nil {
		_ = adapter.stopLocked(context.Background(), true)
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error(), SSHPort: adapter.config.SSHPort})
		return err
	}
	if _, err := adapter.Health(ctx); err != nil {
		_ = adapter.stopLocked(context.Background(), true)
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error()})
		return err
	}
	adapter.mu.Lock()
	adapter.lastStart = time.Since(started)
	adapter.mu.Unlock()
	return adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateReady, PID: process.Process.Pid, StartTicks: processStartTicks(process.Process.Pid), SSHPort: adapter.config.SSHPort})
}

func (adapter *QEMUAdapter) Stop(ctx context.Context, force bool) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	return adapter.stopLocked(ctx, force)
}

func (adapter *QEMUAdapter) stopLocked(ctx context.Context, force bool) error {
	started := time.Now()
	adapter.mu.Lock()
	process := adapter.process
	cancel := adapter.processCancel
	apiService := adapter.apiService
	apiBridge := adapter.apiBridge
	apiCancel := adapter.apiCancel
	adapter.process = nil
	adapter.processCancel = nil
	adapter.apiService = nil
	adapter.apiBridge = nil
	adapter.apiCancel = nil
	adapter.mu.Unlock()
	if apiCancel != nil {
		apiCancel()
	}
	for _, apiProcess := range []*exec.Cmd{apiService, apiBridge} {
		if apiProcess != nil && apiProcess.Process != nil {
			_ = apiProcess.Process.Kill()
			_, _ = apiProcess.Process.Wait()
		}
	}
	if process == nil {
		stored, _ := adapter.readState()
		if stored.PID > 0 && processStartTicks(stored.PID) == stored.StartTicks {
			return errors.New("QEMU process is owned by another Tusk process; retry after it stops")
		}
	}
	if process != nil && process.Process != nil {
		if !force {
			_ = adapter.qmpCommand(ctx, map[string]any{"execute": "system_powerdown"})
		}
		wait := make(chan error, 1)
		go func() { wait <- process.Wait() }()
		select {
		case <-wait:
		case <-ctx.Done():
			_ = process.Process.Kill()
			select {
			case <-wait:
			case <-time.After(5 * time.Second):
			}
		case <-time.After(10 * time.Second):
			_ = process.Process.Kill()
			select {
			case <-wait:
			case <-time.After(5 * time.Second):
			}
		}
	}
	if cancel != nil {
		cancel()
	}
	_ = os.Remove(adapter.qmpPath())
	_ = os.Remove(adapter.pidPath())
	adapter.mu.Lock()
	adapter.lastStop = time.Since(started)
	adapter.mu.Unlock()
	return adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStopped, SSHPort: adapter.config.SSHPort})
}

func (adapter *QEMUAdapter) Restart(ctx context.Context) error {
	if err := adapter.Stop(ctx, false); err != nil {
		return err
	}
	return adapter.Start(ctx)
}

func (adapter *QEMUAdapter) Status(_ context.Context) (RuntimeStatus, error) {
	stored, err := adapter.readState()
	if err != nil {
		return RuntimeStatus{State: StateError, Detail: err.Error()}, err
	}
	if stored.PID > 0 && processStartTicks(stored.PID) == stored.StartTicks && stored.State != StateReady {
		return RuntimeStatus{State: StateDegraded, Detail: "QEMU process is running while state is not ready"}, errors.New("QEMU process/state mismatch")
	}
	if stored.State == StateReady && (stored.PID <= 0 || processStartTicks(stored.PID) != stored.StartTicks) {
		return RuntimeStatus{State: StateDegraded, Detail: "QEMU process identity is stale"}, errors.New("QEMU process identity is stale")
	}
	if stored.State == StateReady {
		if err := adapter.verifyUnixAPI(context.Background()); err != nil {
			return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
		}
	}
	return RuntimeStatus{State: stored.State, Healthy: stored.State == StateReady, Detail: stored.LastError}, nil
}

func (adapter *QEMUAdapter) Execute(ctx context.Context, command Command) (CommandResult, error) {
	return adapter.executeSSH(ctx, command)
}

func (adapter *QEMUAdapter) APIEndpoint(_ context.Context) (string, error) {
	stored, err := adapter.readState()
	if err != nil || stored.State != StateReady {
		return "", errors.New("QEMU Podman API bridge is unavailable")
	}
	if err := adapter.verifyUnixAPI(context.Background()); err != nil {
		return "", err
	}
	return filepath.Join(adapter.config.RuntimeDir, "podman.sock"), nil
}

func (adapter *QEMUAdapter) RecreateAPI(ctx context.Context) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	adapter.mu.Lock()
	service, bridge, cancel := adapter.apiService, adapter.apiBridge, adapter.apiCancel
	adapter.apiService, adapter.apiBridge, adapter.apiCancel = nil, nil, nil
	adapter.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, process := range []*exec.Cmd{service, bridge} {
		if process != nil && process.Process != nil {
			_ = process.Process.Kill()
			_, _ = process.Process.Wait()
		}
	}
	_ = os.Remove(filepath.Join(adapter.config.RuntimeDir, "podman.sock"))
	if err := adapter.startAPIBridge(ctx); err != nil {
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error(), SSHPort: adapter.config.SSHPort})
		return err
	}
	if _, err := adapter.Health(ctx); err != nil {
		_ = adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error(), SSHPort: adapter.config.SSHPort})
		return err
	}
	stored, _ := adapter.readState()
	return adapter.writeState(qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateReady, PID: stored.PID, StartTicks: stored.StartTicks, SSHPort: adapter.config.SSHPort})
}

func (adapter *QEMUAdapter) Health(ctx context.Context) (RuntimeStatus, error) {
	if err := verifyImageFile(adapter.config.DataImage); err != nil {
		return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
	}
	if err := adapter.qmpStatus(ctx); err != nil {
		return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
	}
	for _, args := range [][]string{{"podman", "info", "--format", "{{.Host.Os}}"}, {"podman", "network", "ls"}, {"podman", "volume", "ls"}, {"podman", "compose", "version"}} {
		result, err := adapter.executeSSH(ctx, Command{Args: args})
		if err != nil || result.ExitCode != 0 {
			if err == nil {
				err = fmt.Errorf("%s exited %d: %s", args[1], result.ExitCode, strings.TrimSpace(result.Stderr))
			}
			return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
		}
	}
	return RuntimeStatus{State: StateReady, Healthy: true}, nil
}

func (adapter *QEMUAdapter) Measure(ctx context.Context) (Metrics, error) {
	if !containsAtSHA256(adapter.config.ProbeImage) {
		return Metrics{}, errors.New("QEMU benchmark probe image is not configured with a digest")
	}
	info, err := adapter.executeSSH(ctx, Command{Args: []string{"podman", "info", "--format", "{{.Version.Version}}"}})
	if err != nil || info.ExitCode != 0 {
		if err == nil {
			err = fmt.Errorf("podman info exited %d", info.ExitCode)
		}
		return Metrics{}, err
	}
	var samples []CommandResult
	for i := 0; i < 3; i++ {
		result, err := adapter.executeSSH(ctx, Command{Args: []string{"podman", "run", "--rm", adapter.config.ProbeImage, "true"}})
		if err != nil || result.ExitCode != 0 {
			return Metrics{}, fmt.Errorf("QEMU benchmark sample %d failed: %v", i+1, err)
		}
		samples = append(samples, result)
	}
	storage, err := directoryBytes(adapter.config.DataImage)
	if err != nil {
		return Metrics{}, err
	}
	adapter.mu.Lock()
	lifecycle := adapter.lastStart + adapter.lastStop
	adapter.mu.Unlock()
	if lifecycle <= 0 {
		return Metrics{}, errors.New("QEMU lifecycle timing is unavailable")
	}
	stored, _ := adapter.readState()
	peakMemory := qemuRSSMiB(stored.PID)
	if peakMemory <= 0 {
		return Metrics{}, errors.New("QEMU host process memory could not be measured")
	}
	return Metrics{InteractiveLatencyMillis: info.Duration.Seconds() * 1000, WorkloadThroughput: 3 / (samples[0].Duration.Seconds() + samples[1].Duration.Seconds() + samples[2].Duration.Seconds()), PeakMemoryMiB: peakMemory, StartupShutdownMillis: lifecycle.Seconds() * 1000, PersistentStorageMiB: float64(storage) / (1024 * 1024), ThermalRetention: minRatio(samples[0].Duration, samples[2].Duration)}, nil
}

func (adapter *QEMUAdapter) Cleanup(ctx context.Context) error {
	return adapter.Stop(ctx, true)
}

func (adapter *QEMUAdapter) ForwardTCP(ctx context.Context, port PublishedPort) error {
	if port.HostIP != "127.0.0.1" && port.HostIP != "::1" {
		return errors.New("QEMU host forwarding only supports loopback addresses")
	}
	command := fmt.Sprintf("hostfwd_add net0 tcp:%s:%d-:%d", port.HostIP, port.HostPort, port.ContainerPort)
	return adapter.qmpCommand(ctx, map[string]any{"execute": "human-monitor-command", "arguments": map[string]string{"command-line": command}})
}

func (adapter *QEMUAdapter) RemoveTCP(ctx context.Context, port PublishedPort) error {
	command := fmt.Sprintf("hostfwd_remove net0 tcp:%s:%d-:%d", port.HostIP, port.HostPort, port.ContainerPort)
	return adapter.qmpCommand(ctx, map[string]any{"execute": "human-monitor-command", "arguments": map[string]string{"command-line": command}})
}

func (adapter *QEMUAdapter) qemuArgs() []string {
	args := []string{adapter.config.QEMUPath, "-machine", adapter.config.Machine, "-m", strconv.Itoa(adapter.config.MemoryMiB), "-smp", strconv.Itoa(adapter.config.CPUs), "-drive", "file=" + adapter.config.SystemImage + ",if=virtio,format=qcow2", "-drive", "file=" + adapter.config.DataImage + ",if=virtio,format=qcow2", "-nographic", "-serial", "file:" + adapter.serialPath(), "-qmp", "unix:" + adapter.qmpPath() + ",server=on,wait=off", "-netdev", "user,id=net0,hostfwd=tcp:127.0.0.1:" + strconv.Itoa(adapter.config.SSHPort) + "-:22", "-device", "virtio-net-pci,netdev=net0", "-pidfile", adapter.pidPath()}
	if adapter.config.UseKVM {
		args = append(args, "-enable-kvm")
	} else {
		args = append(args, "-accel", "tcg,thread=multi")
	}
	return args
}

func (adapter *QEMUAdapter) executeSSH(ctx context.Context, command Command) (CommandResult, error) {
	if len(command.Args) == 0 {
		return CommandResult{}, errors.New("QEMU SSH command requires arguments")
	}
	if command.TTY {
		return CommandResult{}, errors.New("QEMU adapter requires a PTY bridge for TTY commands")
	}
	remote := make([]string, 0, len(command.Args)+1)
	if command.WorkingDir != "" {
		remote = append(remote, "cd", shellQuote(command.WorkingDir), "&&")
	}
	for _, arg := range command.Args {
		remote = append(remote, shellQuote(arg))
	}
	if len(command.Env) > 0 {
		prefix := make([]string, 0, len(command.Env))
		for _, value := range command.Env {
			prefix = append(prefix, shellQuote(value))
		}
		remote = append([]string{"env"}, append(prefix, remote...)...)
	}
	args := adapter.sshArgs(strings.Join(remote, " "))
	process := exec.CommandContext(ctx, "ssh", args...)
	process.Stdin = command.Stdin
	var stdout, stderr bytes.Buffer
	process.Stdout = io.MultiWriter(&stdout, writerOrDiscard(command.Stdout))
	process.Stderr = io.MultiWriter(&stderr, writerOrDiscard(command.Stderr))
	started := time.Now()
	err := process.Run()
	result := CommandResult{ExitCode: 0, Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started), PeakMemoryMiB: maxRSSMiB(process.ProcessState)}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		result.ExitCode = 75
		return result, ctx.Err()
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result, nil
	}
	return result, err
}

func (adapter *QEMUAdapter) sshBaseArgs() []string {
	return []string{"-i", adapter.config.SSHPrivateKey, "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + adapter.config.KnownHosts, "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "-p", strconv.Itoa(adapter.config.SSHPort), adapter.config.SSHUser + "@127.0.0.1"}
}

func (adapter *QEMUAdapter) sshArgs(remote string) []string {
	return append(adapter.sshBaseArgs(), "--", remote)
}

func (adapter *QEMUAdapter) startAPIBridge(ctx context.Context) error {
	if adapter.verifyUnixAPI(ctx) == nil {
		return nil
	}
	serviceContext, cancel := context.WithCancel(context.Background())
	serviceArgs := adapter.sshArgs("podman system service --time=0 unix:///run/tusk/podman.sock")
	service := exec.CommandContext(serviceContext, "ssh", serviceArgs...)
	logFile, err := os.OpenFile(adapter.serialPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		cancel()
		return fmt.Errorf("open QEMU API log: %w", err)
	}
	service.Stdout, service.Stderr = logFile, logFile
	if err := service.Start(); err != nil {
		_ = logFile.Close()
		cancel()
		return fmt.Errorf("start guest Podman API service: %w", err)
	}
	bridgeArgs := append(adapter.sshBaseArgs(), "-N", "-T", "-o", "ExitOnForwardFailure=yes", "-L", filepath.Join(adapter.config.RuntimeDir, "podman.sock")+":/run/tusk/podman.sock")
	bridge := exec.CommandContext(serviceContext, "ssh", bridgeArgs...)
	bridge.Stdout, bridge.Stderr = logFile, logFile
	if err := bridge.Start(); err != nil {
		_ = service.Process.Kill()
		_, _ = service.Process.Wait()
		_ = logFile.Close()
		cancel()
		return fmt.Errorf("start QEMU API bridge: %w", err)
	}
	_ = logFile.Close()
	adapter.mu.Lock()
	adapter.apiService, adapter.apiBridge, adapter.apiCancel = service, bridge, cancel
	adapter.mu.Unlock()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		if adapter.verifyUnixAPI(ctx) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out waiting for QEMU Podman API bridge")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (adapter *QEMUAdapter) verifyUnixAPI(ctx context.Context) error {
	connection, err := net.DialTimeout("unix", filepath.Join(adapter.config.RuntimeDir, "podman.sock"), 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(500 * time.Millisecond))
	}
	if _, err := io.WriteString(connection, "GET /_ping HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"); err != nil {
		return err
	}
	response := make([]byte, 256)
	count, err := connection.Read(response)
	if err != nil {
		return err
	}
	if !strings.Contains(string(response[:count]), " 200 ") {
		return errors.New("QEMU Podman API did not return HTTP 200")
	}
	return nil
}

func (adapter *QEMUAdapter) waitReady(ctx context.Context) error {
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()
	for {
		if adapter.qmpStatus(ctx) == nil {
			if result, err := adapter.executeSSH(ctx, Command{Args: []string{"true"}}); err == nil && result.ExitCode == 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("QEMU readiness timeout waiting for QMP and SSH")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (adapter *QEMUAdapter) qmpStatus(ctx context.Context) error {
	return adapter.qmpCommand(ctx, map[string]any{"execute": "query-status"})
}

func (adapter *QEMUAdapter) qmpCommand(ctx context.Context, command map[string]any) error {
	connection, err := net.DialTimeout("unix", adapter.qmpPath(), 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(500 * time.Millisecond))
	}
	greeting := make([]byte, 1024)
	_, _ = connection.Read(greeting)
	data, err := json.Marshal(command)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := connection.Write(data); err != nil {
		return err
	}
	response := make([]byte, 2048)
	count, err := connection.Read(response)
	if err != nil {
		return err
	}
	if !bytes.Contains(response[:count], []byte(`"return"`)) {
		return errors.New("QMP command returned an error")
	}
	return nil
}

func (adapter *QEMUAdapter) qmpPath() string {
	return filepath.Join(adapter.config.RuntimeDir, "qmp.sock")
}
func (adapter *QEMUAdapter) pidPath() string {
	return filepath.Join(adapter.config.RuntimeDir, "qemu.pid")
}
func (adapter *QEMUAdapter) serialPath() string {
	return filepath.Join(adapter.config.RuntimeDir, "serial.log")
}

func (adapter *QEMUAdapter) writeState(value qemuState) error {
	value.UpdatedAt = time.Now().UTC()
	return state.WriteJSON(adapter.statePath, value, 0o600)
}

func (adapter *QEMUAdapter) readState() (qemuState, error) {
	var value qemuState
	if err := state.ReadJSON(adapter.statePath, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return qemuState{SchemaVersion: qemuStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateUninitialized}, nil
		}
		return qemuState{}, err
	}
	if value.SchemaVersion != qemuStateSchemaVersion || value.CandidateID != adapter.config.CandidateID {
		return qemuState{}, errors.New("invalid QEMU state schema or candidate")
	}
	return value, nil
}

func processStartTicks(pid int) uint64 {
	if pid <= 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) <= 21 {
		return 0
	}
	ticks, _ := strconv.ParseUint(fields[21], 10, 64)
	return ticks
}

func isQEMUCandidate(id string) bool {
	return strings.HasPrefix(id, "qemu-system-")
}

func qemuArchitecture(id string) string {
	return strings.TrimPrefix(id, "qemu-system-")
}

func qemuMachine(id string) string {
	if id == "qemu-system-aarch64" || id == "qemu-system-arm" {
		return "virt"
	}
	return "q35"
}

func qemuApplicable(id, hostArch string) bool {
	if id == "qemu-system-aarch64" {
		return hostArch == "arm64" || hostArch == "amd64"
	}
	if id == "qemu-system-x86_64" || id == "qemu-system-i386" {
		return hostArch == "amd64" || hostArch == "arm64"
	}
	return id == "qemu-system-arm" && (hostArch == "arm64" || hostArch == "arm")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func reserveLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate QEMU SSH port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func qemuPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	return listener.Close() == nil
}

func verifyPrivateKey(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("SSH private key unavailable: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("SSH private key must be mode 0600 or stricter")
	}
	return nil
}

func verifyImageFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("QEMU persistent data image unavailable: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return errors.New("QEMU persistent data image is empty or a directory")
	}
	return nil
}

func qemuFailureCategory(err error) string {
	if errors.Is(err, os.ErrPermission) {
		return "permission"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "dependency"
	}
	return "launch"
}

func qemuRSSMiB(pid int) float64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "VmRSS:" && fields[2] == "kB" {
			value, _ := strconv.ParseFloat(fields[1], 64)
			return value / 1024
		}
	}
	return 0
}
