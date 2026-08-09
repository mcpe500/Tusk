package engine

import (
	"bytes"
	"context"
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
	"syscall"
	"time"

	"github.com/mcpe500/Tusk/internal/platform"
	"github.com/mcpe500/Tusk/internal/state"
)

const prootStateSchemaVersion = 1

type ProotConfig struct {
	CandidateID       string
	ProotPath         string
	QEMUUserPath      string
	Rootfs            string
	DataDir           string
	SharedRoot        string
	RuntimeDir        string
	PodmanStoragePath string
	ProbeImage        string
}

type BackendResources struct {
	CPUs      int
	MemoryMiB int
}

type ProotAdapter struct {
	mu            sync.Mutex
	lifecycleMu   sync.Mutex
	config        ProotConfig
	statePath     string
	service       *exec.Cmd
	serviceCancel context.CancelFunc
	lastStart     time.Duration
	lastStop      time.Duration
}

type prootState struct {
	SchemaVersion int            `json:"schemaVersion"`
	CandidateID   string         `json:"candidateId"`
	State         LifecycleState `json:"state"`
	LastError     string         `json:"lastError,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func NewProotAdapter(config ProotConfig) (*ProotAdapter, error) {
	if config.CandidateID == "" {
		return nil, errors.New("proot candidate ID is required")
	}
	if config.CandidateID != "proot-native" && config.CandidateID != "proot-qemu-user-x86_64" {
		return nil, fmt.Errorf("unsupported proot candidate %q", config.CandidateID)
	}
	if config.ProotPath == "" {
		config.ProotPath = "proot"
	}
	if config.PodmanStoragePath == "" {
		config.PodmanStoragePath = "/var/lib/containers/storage"
	}
	if config.SharedRoot == "" {
		return nil, errors.New("proot shared root is required")
	}
	if config.RuntimeDir == "" {
		config.RuntimeDir = filepath.Join(config.DataDir, "run")
	}
	if err := validateProotPaths(config); err != nil {
		return nil, err
	}
	return &ProotAdapter{config: config, statePath: filepath.Join(config.DataDir, "state.json")}, nil
}

// NewLocalCandidateFactory wires the real proot adapters into the tuning
// pipeline. Missing rootfs or executables return an explicit rejection; the
// factory never returns a successful placeholder target.
func NewLocalCandidateFactory(dataRoot string, device platform.Device) CandidateTargetFactory {
	return NewLocalCandidateFactoryWithProbe(dataRoot, device, "")
}

func NewLocalCandidateFactoryWithProbe(dataRoot string, device platform.Device, probeImage string) CandidateTargetFactory {
	return NewLocalCandidateFactoryWithProbeAndResources(dataRoot, device, probeImage, nil)
}

func NewLocalCandidateFactoryWithProbeAndResources(dataRoot string, device platform.Device, probeImage string, resources map[string]BackendResources) CandidateTargetFactory {
	return func(ctx context.Context, candidate Candidate) (QualificationTarget, error) {
		if isQEMUCandidate(candidate.ID) {
			return newLocalQEMUTarget(ctx, dataRoot, device, candidate, probeImage, resources[candidate.ID])
		}
		if candidate.ID != "proot-native" && candidate.ID != "proot-qemu-user-x86_64" {
			return nil, fmt.Errorf("candidate %s is not a packaged adapter", candidate.ID)
		}
		engineRoot := filepath.Join(dataRoot, "engines", candidate.ID)
		config := ProotConfig{
			CandidateID: candidate.ID,
			ProotPath:   device.Commands["proot"],
			Rootfs:      filepath.Join(engineRoot, "rootfs"),
			DataDir:     filepath.Join(engineRoot, "data"),
			SharedRoot:  filepath.Join(dataRoot, "workspaces"),
			RuntimeDir:  filepath.Join(dataRoot, "run", candidate.ID),
			ProbeImage:  probeImage,
		}
		if candidate.ID == "proot-qemu-user-x86_64" {
			config.QEMUUserPath = device.Commands["qemu-x86_64"]
		}
		adapter, err := NewProotAdapter(config)
		if err != nil {
			return nil, err
		}
		preflight, err := adapter.Preflight(ctx, device)
		if err != nil {
			return nil, err
		}
		if !preflight.Applicable || !preflight.Available {
			return nil, fmt.Errorf("proot preflight rejected %s: %s", candidate.ID, strings.Join(preflight.Reasons, "; "))
		}
		if err := adapter.Initialize(ctx); err != nil {
			return nil, err
		}
		return AdapterTarget{Adapter: adapter}, nil
	}
}

func newLocalQEMUTarget(ctx context.Context, dataRoot string, device platform.Device, candidate Candidate, probeImage string, resources BackendResources) (QualificationTarget, error) {
	if resources.CPUs == 0 {
		resources.CPUs = 2
	}
	if resources.MemoryMiB == 0 {
		resources.MemoryMiB = 1024
	}
	engineRoot := filepath.Join(dataRoot, "engines", candidate.ID)
	command := candidate.RequiredCommands[0]
	config := QEMUConfig{
		CandidateID:   candidate.ID,
		QEMUPath:      device.Commands[command],
		SystemImage:   filepath.Join(engineRoot, "system.qcow2"),
		DataImage:     filepath.Join(engineRoot, "data.qcow2"),
		RuntimeDir:    filepath.Join(dataRoot, "run", candidate.ID),
		SharedRoot:    filepath.Join(dataRoot, "workspaces"),
		SSHPrivateKey: filepath.Join(dataRoot, "keys", "guest_ed25519"),
		KnownHosts:    filepath.Join(dataRoot, "keys", "known_hosts"),
		SSHPort:       0,
		CPUs:          resources.CPUs,
		MemoryMiB:     resources.MemoryMiB,
		ProbeImage:    probeImage,
	}
	adapter, err := NewQEMUAdapter(config)
	if err != nil {
		return nil, err
	}
	preflight, err := adapter.Preflight(ctx, device)
	if err != nil {
		return nil, err
	}
	if !preflight.Applicable || !preflight.Available {
		return nil, fmt.Errorf("QEMU preflight rejected %s: %s", candidate.ID, strings.Join(preflight.Reasons, "; "))
	}
	if err := adapter.Initialize(ctx); err != nil {
		return nil, err
	}
	return AdapterTarget{Adapter: adapter}, nil
}

func qemuSSHPort(candidateID string) int {
	for index, candidate := range Catalog() {
		if candidate.ID == candidateID {
			return 22000 + index
		}
	}
	return 22999
}

func (adapter *ProotAdapter) Candidate() Candidate {
	for _, candidate := range Catalog() {
		if candidate.ID == adapter.config.CandidateID {
			return candidate
		}
	}
	return Candidate{ID: adapter.config.CandidateID, Kind: KindProot}
}

func (adapter *ProotAdapter) Preflight(_ context.Context, device platform.Device) (Preflight, error) {
	preflight := Preflight{Applicable: true, Available: true}
	if adapter.config.CandidateID == "proot-qemu-user-x86_64" && device.HostArch == "amd64" {
		preflight.Applicable = false
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "qemu-user x86_64 is unnecessary on an amd64 host")
	}
	if info, err := os.Stat(adapter.config.Rootfs); err != nil {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "rootfs is unavailable: "+err.Error())
	} else if !info.IsDir() {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "rootfs is not a directory")
	}
	if _, err := os.Stat(adapter.config.DataDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "data directory cannot be inspected: "+err.Error())
	}
	if filepath.IsAbs(adapter.config.ProotPath) {
		if info, err := os.Stat(adapter.config.ProotPath); err != nil {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "proot is unavailable: "+err.Error())
		} else if info.Mode()&0o111 == 0 {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "proot is not executable")
		}
	} else if _, err := exec.LookPath(adapter.config.ProotPath); err != nil {
		preflight.Available = false
		preflight.Reasons = append(preflight.Reasons, "proot is unavailable: "+err.Error())
	}
	if adapter.config.CandidateID == "proot-qemu-user-x86_64" {
		if adapter.config.QEMUUserPath == "" {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "qemu-user executable is not configured")
		} else if filepath.IsAbs(adapter.config.QEMUUserPath) {
			if info, err := os.Stat(adapter.config.QEMUUserPath); err != nil {
				preflight.Available = false
				preflight.Reasons = append(preflight.Reasons, "qemu-user is unavailable: "+err.Error())
			} else if info.Mode()&0o111 == 0 {
				preflight.Available = false
				preflight.Reasons = append(preflight.Reasons, "qemu-user is not executable")
			}
		} else if _, err := exec.LookPath(adapter.config.QEMUUserPath); err != nil {
			preflight.Available = false
			preflight.Reasons = append(preflight.Reasons, "qemu-user is unavailable: "+err.Error())
		}
	}
	return preflight, nil
}

func (adapter *ProotAdapter) Initialize(_ context.Context) error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if err := validateProotPaths(adapter.config); err != nil {
		return err
	}
	if info, err := os.Stat(adapter.config.Rootfs); err != nil {
		_ = adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateUninitialized, LastError: err.Error()})
		return fmt.Errorf("initialize proot backend: rootfs is unavailable: %w", err)
	} else if !info.IsDir() {
		return errors.New("initialize proot backend: rootfs is not a directory")
	}
	for _, directory := range []string{adapter.config.DataDir, adapter.config.SharedRoot, adapter.config.RuntimeDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create proot directory %s: %w", directory, err)
		}
	}
	if err := os.Remove(filepath.Join(adapter.config.RuntimeDir, "podman.sock")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale proot API socket: %w", err)
	}
	return adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStopped})
}

func (adapter *ProotAdapter) Start(ctx context.Context) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	started := time.Now()
	adapter.mu.Lock()
	if err := adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStarting}); err != nil {
		adapter.mu.Unlock()
		return err
	}
	adapter.mu.Unlock()
	if _, err := adapter.execute(ctx, Command{Args: []string{"podman", "info", "--format", "{{.Version.Version}}"}}); err != nil {
		_ = adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateError, LastError: err.Error()})
		return fmt.Errorf("start proot Podman: %w", err)
	}
	if err := adapter.startAPI(ctx); err != nil {
		_ = adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error()})
		return err
	}
	if _, err := adapter.Health(ctx); err != nil {
		_ = adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: err.Error()})
		return fmt.Errorf("check proot readiness: %w", err)
	}
	if err := adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateReady}); err != nil {
		_ = adapter.stopLocked(context.Background(), true)
		return err
	}
	adapter.mu.Lock()
	adapter.lastStart = time.Since(started)
	adapter.mu.Unlock()
	return nil
}

func (adapter *ProotAdapter) Stop(ctx context.Context, force bool) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	return adapter.stopLocked(ctx, force)
}

func (adapter *ProotAdapter) stopLocked(ctx context.Context, force bool) error {
	started := time.Now()
	adapter.mu.Lock()
	cancel := adapter.serviceCancel
	service := adapter.service
	adapter.serviceCancel = nil
	adapter.service = nil
	adapter.mu.Unlock()
	if service != nil {
		wait := make(chan error, 1)
		go func() { wait <- service.Wait() }()
		if force && service.Process != nil {
			_ = service.Process.Kill()
		} else if service.Process != nil {
			_ = service.Process.Signal(syscall.SIGTERM)
		}
		stopped := false
		grace := time.NewTimer(5 * time.Second)
		select {
		case <-wait:
			stopped = true
		case <-ctx.Done():
			if service.Process != nil {
				_ = service.Process.Kill()
			}
			<-wait
			stopped = true
		case <-grace.C:
			if service.Process != nil {
				_ = service.Process.Kill()
			}
			select {
			case <-wait:
				stopped = true
			case <-time.After(5 * time.Second):
			}
		}
		grace.Stop()
		if !stopped {
			return fmt.Errorf("stop proot API service timed out (force=%t)", force)
		}
	}
	if cancel != nil {
		cancel()
	}
	if err := os.Remove(filepath.Join(adapter.config.RuntimeDir, "podman.sock")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove proot API socket: %w", err)
	}
	adapter.mu.Lock()
	adapter.lastStop = time.Since(started)
	adapter.mu.Unlock()
	return adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateStopped})
}

func (adapter *ProotAdapter) Status(_ context.Context) (RuntimeStatus, error) {
	stored, err := adapter.readState()
	if err != nil {
		return RuntimeStatus{State: StateError, Detail: err.Error()}, err
	}
	if stored.State == StateReady {
		if err := adapter.verifyStorage(); err != nil {
			return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
		}
		if err := adapter.verifyAPI(context.Background()); err != nil {
			return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
		}
	}
	return RuntimeStatus{State: stored.State, Healthy: stored.State == StateReady, Detail: stored.LastError}, nil
}

func (adapter *ProotAdapter) Execute(ctx context.Context, command Command) (CommandResult, error) {
	return adapter.execute(ctx, command)
}

func (adapter *ProotAdapter) APIEndpoint(_ context.Context) (string, error) {
	stored, err := adapter.readState()
	if err != nil {
		return "", err
	}
	if stored.State != StateReady {
		return "", fmt.Errorf("proot backend is %s, API socket is unavailable", stored.State)
	}
	if err := adapter.verifyAPI(context.Background()); err != nil {
		return "", err
	}
	return filepath.Join(adapter.config.RuntimeDir, "podman.sock"), nil
}

func (adapter *ProotAdapter) Health(ctx context.Context) (RuntimeStatus, error) {
	checks := [][]string{
		{"podman", "info", "--format", "{{.Host.Os}}"},
		{"podman", "network", "ls"},
		{"podman", "volume", "ls"},
		{"podman", "compose", "version"},
	}
	for _, args := range checks {
		result, err := adapter.execute(ctx, Command{Args: args})
		if err != nil || result.ExitCode != 0 {
			if err == nil {
				err = fmt.Errorf("%s exited %d: %s", args[1], result.ExitCode, strings.TrimSpace(result.Stderr))
			}
			return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
		}
	}
	if err := adapter.verifyStorage(); err != nil {
		return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
	}
	if err := adapter.verifyAPI(ctx); err != nil {
		return RuntimeStatus{State: StateDegraded, Detail: err.Error()}, err
	}
	return RuntimeStatus{State: StateReady, Healthy: true}, nil
}

func (adapter *ProotAdapter) Measure(ctx context.Context) (Metrics, error) {
	if !containsAtSHA256(adapter.config.ProbeImage) {
		return Metrics{}, errors.New("proot benchmark probe image is not configured with a digest")
	}
	infoResult, err := adapter.execute(ctx, Command{Args: []string{"podman", "info", "--format", "{{.Version.Version}}"}})
	if err != nil || infoResult.ExitCode != 0 {
		if err == nil {
			err = fmt.Errorf("podman info exited %d", infoResult.ExitCode)
		}
		return Metrics{}, err
	}
	var samples []CommandResult
	for i := 0; i < 3; i++ {
		result, err := adapter.execute(ctx, Command{Args: []string{"podman", "run", "--rm", adapter.config.ProbeImage, "true"}})
		if err != nil || result.ExitCode != 0 {
			if err == nil {
				err = fmt.Errorf("podman info exited %d", result.ExitCode)
			}
			return Metrics{}, err
		}
		if result.PeakMemoryMiB <= 0 {
			return Metrics{}, errors.New("proot measurement did not expose peak memory")
		}
		samples = append(samples, result)
	}
	storage, err := directoryBytes(adapter.config.DataDir)
	if err != nil {
		return Metrics{}, err
	}
	first := samples[0]
	last := samples[len(samples)-1]
	latency := infoResult.Duration.Seconds()
	if latency <= 0 {
		return Metrics{}, errors.New("proot measurement returned no latency")
	}
	adapter.mu.Lock()
	startupShutdown := adapter.lastStart + adapter.lastStop
	adapter.mu.Unlock()
	if startupShutdown <= 0 {
		return Metrics{}, errors.New("proot lifecycle startup/shutdown timing is unavailable")
	}
	return Metrics{
		InteractiveLatencyMillis: latency * 1000,
		WorkloadThroughput:       3 / (samples[0].Duration.Seconds() + samples[1].Duration.Seconds() + samples[2].Duration.Seconds()),
		PeakMemoryMiB:            maxRSS(samples),
		StartupShutdownMillis:    startupShutdown.Seconds() * 1000,
		PersistentStorageMiB:     float64(storage) / (1024 * 1024),
		ThermalRetention:         minRatio(first.Duration, last.Duration),
	}, nil
}

func minRatio(first, last time.Duration) float64 {
	if first <= 0 || last <= 0 {
		return 0
	}
	ratio := first.Seconds() / last.Seconds()
	if ratio > 1 {
		return 1
	}
	return ratio
}

func maxRSS(samples []CommandResult) float64 {
	var maximum float64
	for _, sample := range samples {
		if sample.PeakMemoryMiB > maximum {
			maximum = sample.PeakMemoryMiB
		}
	}
	return maximum
}

func (adapter *ProotAdapter) Cleanup(ctx context.Context) error {
	return adapter.Stop(ctx, true)
}

func (adapter *ProotAdapter) ForwardTCP(ctx context.Context, port PublishedPort) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(port.HostIP, strconv.Itoa(port.HostPort)))
	if err != nil {
		return fmt.Errorf("verify native proot TCP publish %s:%d: %w", port.HostIP, port.HostPort, err)
	}
	return connection.Close()
}

func (adapter *ProotAdapter) RemoveTCP(context.Context, PublishedPort) error { return nil }

// RecreateAPI repairs only the local bridge and preserves the backend data.
func (adapter *ProotAdapter) RecreateAPI(ctx context.Context) error {
	adapter.lifecycleMu.Lock()
	defer adapter.lifecycleMu.Unlock()
	_ = adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateDegraded, LastError: "recreating Podman API bridge"})
	adapter.mu.Lock()
	cancel := adapter.serviceCancel
	service := adapter.service
	adapter.serviceCancel = nil
	adapter.service = nil
	adapter.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if service != nil && service.Process != nil {
		_ = service.Process.Kill()
		_, _ = service.Process.Wait()
	}
	_ = os.Remove(filepath.Join(adapter.config.RuntimeDir, "podman.sock"))
	if err := adapter.startAPI(ctx); err != nil {
		return err
	}
	if _, err := adapter.Health(ctx); err != nil {
		return err
	}
	return adapter.writeState(prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateReady})
}

func (adapter *ProotAdapter) execute(ctx context.Context, command Command) (CommandResult, error) {
	if len(command.Args) == 0 {
		return CommandResult{}, errors.New("proot command requires arguments")
	}
	args := adapter.prootArgs(command.Args)
	process := exec.CommandContext(ctx, args[0], args[1:]...)
	process.Stdin = command.Stdin
	if process.Stdin == nil {
		process.Stdin = strings.NewReader("")
	}
	var stdout, stderr bytes.Buffer
	if command.TTY {
		return CommandResult{}, errors.New("proot adapter requires a PTY bridge for TTY commands")
	}
	process.Stdout = io.MultiWriter(&stdout, writerOrDiscard(command.Stdout))
	process.Stderr = io.MultiWriter(&stderr, writerOrDiscard(command.Stderr))
	if len(command.Env) > 0 {
		process.Env = append(os.Environ(), command.Env...)
	}
	if command.WorkingDir != "" {
		process.Dir = command.WorkingDir
	}
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

func (adapter *ProotAdapter) prootArgs(command []string) []string {
	args := []string{adapter.config.ProotPath, "-r", adapter.config.Rootfs, "-b", adapter.config.DataDir + ":" + adapter.config.PodmanStoragePath, "-b", adapter.config.SharedRoot + ":/mnt/tusk/workspaces", "-b", adapter.config.RuntimeDir + ":/run/tusk"}
	if adapter.config.CandidateID == "proot-qemu-user-x86_64" {
		args = append(args, "-q", adapter.config.QEMUUserPath)
	}
	return append(args, command...)
}

func (adapter *ProotAdapter) startAPI(ctx context.Context) error {
	adapter.mu.Lock()
	if adapter.service != nil {
		alive := adapter.service.ProcessState == nil
		adapter.mu.Unlock()
		if alive && adapter.verifyAPI(ctx) == nil {
			return nil
		}
		adapter.mu.Lock()
		adapter.service = nil
		adapter.serviceCancel = nil
		adapter.mu.Unlock()
	} else {
		adapter.mu.Unlock()
	}
	serviceContext, cancel := context.WithCancel(context.Background())
	args := adapter.prootArgs([]string{"podman", "system", "service", "--time=0", "unix:///run/tusk/podman.sock"})
	logPath := filepath.Join(adapter.config.RuntimeDir, "podman-service.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		cancel()
		return fmt.Errorf("open proot API log: %w", err)
	}
	service := exec.CommandContext(serviceContext, args[0], args[1:]...)
	service.Stdout = logFile
	service.Stderr = logFile
	if err := service.Start(); err != nil {
		_ = logFile.Close()
		cancel()
		return fmt.Errorf("start proot API service: %w", err)
	}
	adapter.mu.Lock()
	adapter.service = service
	adapter.serviceCancel = cancel
	adapter.mu.Unlock()
	cleanupService := func() {
		cancel()
		if service.Process != nil {
			_ = service.Process.Kill()
			_, _ = service.Process.Wait()
		}
		_ = logFile.Close()
		adapter.mu.Lock()
		adapter.service = nil
		adapter.serviceCancel = nil
		adapter.mu.Unlock()
		_ = os.Remove(filepath.Join(adapter.config.RuntimeDir, "podman.sock"))
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if adapter.verifyAPI(ctx) == nil {
			_ = logFile.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			cleanupService()
			return ctx.Err()
		case <-deadline.C:
			cleanupService()
			return errors.New("timed out waiting for proot Podman API socket")
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func (adapter *ProotAdapter) verifyAPI(ctx context.Context) error {
	socket := filepath.Join(adapter.config.RuntimeDir, "podman.sock")
	connection, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("connect proot Podman API socket: %w", err)
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(500 * time.Millisecond))
	}
	if _, err := io.WriteString(connection, "GET /_ping HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"); err != nil {
		return fmt.Errorf("query proot Podman API: %w", err)
	}
	buffer := make([]byte, 256)
	count, err := connection.Read(buffer)
	if err != nil {
		return fmt.Errorf("read proot Podman API response: %w", err)
	}
	if !strings.Contains(string(buffer[:count]), " 200 ") {
		return fmt.Errorf("proot Podman API did not return HTTP 200")
	}
	return nil
}

func (adapter *ProotAdapter) verifyStorage() error {
	if info, err := os.Stat(adapter.config.DataDir); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("data path is not a directory")
		}
		return fmt.Errorf("proot persistent storage is unavailable: %w", err)
	}
	temporary, err := os.CreateTemp(adapter.config.DataDir, ".ready-*")
	if err != nil {
		return fmt.Errorf("proot persistent storage is not writable: %w", err)
	}
	name := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func (adapter *ProotAdapter) writeState(value prootState) error {
	value.UpdatedAt = time.Now().UTC()
	return state.WriteJSON(adapter.statePath, value, 0o600)
}

func (adapter *ProotAdapter) readState() (prootState, error) {
	var value prootState
	if err := state.ReadJSON(adapter.statePath, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return prootState{SchemaVersion: prootStateSchemaVersion, CandidateID: adapter.config.CandidateID, State: StateUninitialized}, nil
		}
		return prootState{}, err
	}
	if value.SchemaVersion != prootStateSchemaVersion || value.CandidateID != adapter.config.CandidateID {
		return prootState{}, errors.New("invalid proot state schema or candidate")
	}
	return value, nil
}

func validateProotPaths(config ProotConfig) error {
	for name, path := range map[string]string{"rootfs": config.Rootfs, "data": config.DataDir, "shared": config.SharedRoot, "runtime": config.RuntimeDir} {
		if path == "" || !filepath.IsAbs(path) {
			return fmt.Errorf("proot %s path must be absolute", name)
		}
	}
	return nil
}

func writerOrDiscard(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}

func maxRSSMiB(processState *os.ProcessState) float64 {
	if processState == nil {
		return 0
	}
	usage, ok := processState.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss <= 0 {
		return 0
	}
	return float64(usage.Maxrss) / 1024
}

func directoryBytes(root string) (uint64, error) {
	var total uint64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += uint64(info.Size())
		return nil
	})
	return total, err
}
