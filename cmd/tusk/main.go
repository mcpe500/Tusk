package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcpe500/Tusk/internal/artifact"
	"github.com/mcpe500/Tusk/internal/config"
	"github.com/mcpe500/Tusk/internal/engine"
	"github.com/mcpe500/Tusk/internal/platform"
	"github.com/mcpe500/Tusk/internal/state"
)

const (
	version       = engine.HostVersion
	specVersion   = "0.2.0"
	exUsage       = 64
	exUnavailable = 69
	exSoftware    = 70
)

func main() {
	invocation := filepath.Base(os.Args[0])
	if invocation == "podman" || invocation == "podman-compose" {
		os.Exit(runPodman(os.Args[1:], invocation == "podman-compose", os.Stdout, os.Stderr))
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exUsage
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "tusk %s (spec %s)\n", version, specVersion)
		return 0
	case "device":
		return runDevice(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "engine":
		return runEngine(args[1:], stdout, stderr)
	case "podman":
		return runPodman(args[1:], false, stdout, stderr)
	case "compose":
		if len(args) == 2 && args[1] == "features" {
			return writeJSON(stdout, stderr, engine.ComposeFeatureMatrix())
		}
		return runPodman(append([]string{"compose"}, args[1:]...), false, stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "env":
		return runEnv(args[1:], stdout, stderr)
	case "share":
		return runShare(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "backup":
		return runBackup(args[1:], stdout, stderr)
	case "uninstall":
		return runUninstall(args[1:], stdout, stderr)
	case "update":
		return runUpdate(args[1:], stdout, stderr)
	case "acceptance":
		return runAcceptance(args[1:], stdout, stderr)
	case "logs":
		return runLogs(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exUsage
	}
}

func runPodman(args []string, composeAlias bool, stdout, stderr io.Writer) int {
	helpRequested := len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || (composeAlias && args[0] == "help"))
	if composeAlias && !helpRequested {
		args = append([]string{"compose"}, args...)
	}
	isCompose := len(args) > 0 && args[0] == "compose"
	if isCompose && !helpRequested {
		validated, err := engine.ComposeArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exUsage
		}
		args = validated
	}
	if helpRequested {
		backend := "auto (qualification required)"
		if cfg, err := config.Load(filepath.Join(dataDir(), "config.json")); err == nil && cfg.SelectedEngine != "" {
			backend = cfg.SelectedEngine
		}
		engine.PodmanShimHelp(stdout, backend)
		return 0
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return exSoftware
	}
	selected := cfg.SelectedEngine
	if selected == "auto" {
		var report engine.TuneReport
		if err := state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report); err == nil {
			selected = report.Selected
		}
	}
	if selected == "" || selected == "auto" {
		fmt.Fprintf(stderr, "no qualified backend is selected; run %s\n", engine.RecoveryTuneCommand)
		return exUnavailable
	}
	var composeMetadata engine.ComposeMetadata
	if isCompose && !helpRequested {
		composeMetadata, err = engine.LoadComposeMetadata(filepath.Join(dataDir(), "engines", selected, "compose.json"))
		if err != nil {
			fmt.Fprintf(stderr, "load pinned Compose provider: %v\n", err)
			return exUnavailable
		}
	}
	var candidate engine.Candidate
	for _, item := range engine.Catalog() {
		if item.ID == selected {
			candidate = item
			break
		}
	}
	if candidate.ID == "" {
		fmt.Fprintf(stderr, "selected backend %q is unknown; run %s\n", selected, engine.RecoveryTuneCommand)
		return exUsage
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return exSoftware
	}
	if err := engine.ValidateWorkingDirectory(workingDirectory, filepath.Join(dataDir(), "workspaces"), args); err != nil {
		fmt.Fprintln(stderr, err)
		return exUsage
	}
	sharesPath := filepath.Join(dataDir(), "shares.json")
	shares, err := engine.LoadShares(sharesPath, filepath.Join(dataDir(), "workspaces"))
	if err != nil {
		fmt.Fprintf(stderr, "load shares: %v\n", err)
		return exSoftware
	}
	translatedArgs, guestWorkingDirectory, err := engine.TranslatePodmanArgs(args, workingDirectory, shares)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exUsage
	}
	resources := make(map[string]engine.BackendResources, len(cfg.Engines))
	for id, value := range cfg.Engines {
		resources[id] = engine.BackendResources{CPUs: value.CPUs, MemoryMiB: value.MemoryMiB}
	}
	factory := engine.NewLocalCandidateFactoryWithProbeAndResources(dataDir(), device, os.Getenv("TUSK_PROBE_IMAGE"), resources)
	target, err := factory(context.Background(), candidate)
	if err != nil {
		fmt.Fprintf(stderr, "initialize selected backend %s: %v\n", selected, err)
		return exUnavailable
	}
	adapterTarget, ok := target.(engine.AdapterTarget)
	if !ok || adapterTarget.Adapter == nil {
		fmt.Fprintln(stderr, "selected backend does not expose a Podman adapter")
		return exSoftware
	}
	var reconciler *engine.PortReconciler
	if forwarder, ok := adapterTarget.Adapter.(engine.PortForwarder); ok {
		reconciler = &engine.PortReconciler{Path: filepath.Join(dataDir(), "engines", selected, "ports.json"), BackendID: selected, Forwarder: forwarder}
	}
	tty := terminalFile(os.Stdin) && terminalWriter(stdout)
	versionOutput := len(translatedArgs) > 0 && translatedArgs[0] == "version" && !containsOutputFormat(translatedArgs[1:])
	output := stdout
	var captured bytes.Buffer
	if versionOutput {
		output = &captured
	}
	result, err := engine.ForwardPodman(context.Background(), adapterTarget.Adapter, translatedArgs, os.Stdin, output, stderr, tty, engine.ForwardOptions{AutoStart: os.Getenv("TUSK_NO_AUTO_START") != "1", WorkingDir: guestWorkingDirectory, Reconciler: reconciler})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exUnavailable
	}
	if versionOutput {
		fmt.Fprintf(stdout, "Tusk shim: %s\nSmart Engine policy: %s v%d\nSelected backend: %s\n", version, cfg.SmartEngine.Policy, cfg.SmartEngine.PolicyVersion, selected)
		if composeMetadata.Provider.Version != "" {
			fmt.Fprintf(stdout, "Compose provider: %s %s\n", composeMetadata.Provider.Name, composeMetadata.Provider.Version)
		}
		fmt.Fprintf(stdout, "Real Podman:\n%s", captured.String())
	}
	return result.ExitCode
}

func runShare(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: tusk share list|add|remove")
		return exUsage
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "usage: tusk share list|add|remove")
		return 0
	}
	path := filepath.Join(dataDir(), "shares.json")
	registry, err := engine.LoadShares(path, filepath.Join(dataDir(), "workspaces"))
	if err != nil {
		fmt.Fprintf(stderr, "load shares: %v\n", err)
		return exSoftware
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: tusk share list")
			return exUsage
		}
		return writeJSON(stdout, stderr, registry.Shares)
	case "add":
		flags := flag.NewFlagSet("share add", flag.ContinueOnError)
		flags.SetOutput(stderr)
		name := flags.String("name", "", "stable share name")
		readOnly := flags.Bool("read-only", false, "export the share read-only")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 1 {
			fmt.Fprintln(stderr, "usage: tusk share add <host-path> [--name name] [--read-only]")
			return exUsage
		}
		registry, err = engine.AddShare(registry, flags.Arg(0), *name, *readOnly)
		if err != nil {
			fmt.Fprintf(stderr, "add share: %v\n", err)
			return exUsage
		}
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: tusk share remove <name>")
			return exUsage
		}
		registry, err = engine.RemoveShare(registry, args[1])
		if err != nil {
			fmt.Fprintf(stderr, "remove share: %v\n", err)
			return exUsage
		}
	default:
		fmt.Fprintf(stderr, "unknown share command %q\n", args[0])
		return exUsage
	}
	if err := engine.SaveShares(path, registry); err != nil {
		fmt.Fprintf(stderr, "save shares: %v\n", err)
		return exSoftware
	}
	fmt.Fprintln(stdout, "share configuration updated")
	return 0
}

func runEnv(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: tusk env")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return exSoftware
	}
	selected := cfg.SelectedEngine
	if selected == "auto" {
		var report engine.TuneReport
		_ = state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report)
		selected = report.Selected
	}
	if selected == "" || selected == "auto" {
		fmt.Fprintf(stderr, "no qualified backend is selected; run %s\n", engine.RecoveryTuneCommand)
		return exUnavailable
	}
	fmt.Fprintf(stdout, "DOCKER_HOST=unix://%s\n", filepath.Join(dataDir(), "run", selected, "podman.sock"))
	return 0
}

func containsOutputFormat(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--format") || arg == "--json" {
			return true
		}
	}
	return false
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: tusk status")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	var report engine.TuneReport
	if err := state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report); err != nil {
		fmt.Fprintf(stdout, "Selected backend: none\nQualification report: unavailable\nRecovery: %s\n", engine.RecoveryTuneCommand)
		return 0
	}
	fmt.Fprintf(stdout, "Selected backend: %s\nTune mode: %s\nPolicy: %s v%d\n", report.Selected, report.Mode, report.Policy.Name, report.Policy.Version)
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	bundle := flags.String("bundle", "", "write a redacted diagnostic bundle")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk doctor [--json]")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	report := struct {
		SchemaVersion int                `json:"schemaVersion"`
		Device        platform.Device    `json:"device"`
		Candidates    []engine.Discovery `json:"candidates"`
		Recovery      string             `json:"recovery"`
	}{SchemaVersion: 1, Device: device, Candidates: engine.DiscoverCandidates(device), Recovery: engine.RecoveryTuneCommand}
	if *bundle != "" {
		files := []string{"config.json", "shares.json", "acceptance.json", "update-plan.json"}
		fmt.Fprintf(stdout, "Diagnostic bundle files: %v\n", files)
		if err := engine.CreateDiagnosticBundle(*bundle, dataDir(), files); err != nil {
			fmt.Fprintf(stderr, "create diagnostic bundle: %v\n", err)
			return exSoftware
		}
		fmt.Fprintf(stdout, "Diagnostic bundle: %s\n", *bundle)
	}
	if *jsonOutput {
		return writeJSON(stdout, stderr, report)
	}
	fmt.Fprintf(stdout, "Host: %s/%s\nCandidates: %d\nRecovery: %s\n", device.HostOS, device.HostArch, len(report.Candidates), report.Recovery)
	return 0
}

func runLogs(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: tusk logs engine|backend|benchmark|install|update")
		return exUsage
	}
	paths := map[string]string{"engine": "engine.log", "backend": "backend.log", "benchmark": "benchmark.log", "install": "install.log", "update": "update.log"}
	relative, ok := paths[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown log category %q\n", args[0])
		return exUsage
	}
	path := filepath.Join(dataDir(), "logs", relative)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", path, err)
		return exSoftware
	}
	const maxLogBytes = 64 * 1024
	if len(data) > maxLogBytes {
		data = data[len(data)-maxLogBytes:]
	}
	_, _ = stdout.Write(data)
	return 0
}

func runBackup(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "", "backend data directory")
	destination := flags.String("output", "", "backup archive path")
	backend := flags.String("backend", "", "backend ID")
	stopped := flags.Bool("stopped", false, "assert the backend is stopped before backup")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk backup --backend <id> --source <data-dir> --output <archive.tar.gz> --stopped")
		return exUsage
	}
	if *source == "" && *backend != "" {
		*source = filepath.Join(dataDir(), "engines", *backend, "data")
	}
	if *source == "" || *destination == "" || *backend == "" || !*stopped {
		fmt.Fprintln(stderr, "backup requires --backend, --source, --output, and --stopped")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	metadata, err := engine.CreateBackup(context.Background(), *source, *destination, engine.BackupMetadata{BackendID: *backend, Architecture: device.HostArch, TuskVersion: version, RuntimeVersion: version, DataSchema: 1, WorkspaceOutside: true, Stopped: true})
	if err != nil {
		fmt.Fprintf(stderr, "backup: %v\n", err)
		return exSoftware
	}
	return writeJSON(stdout, stderr, metadata)
}

func runUninstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(stderr)
	purge := flags.Bool("purge-data", false, "also delete backend data; required explicitly")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk uninstall [--purge-data]")
		return exUsage
	}
	adapter, _, resolveErr := selectedAdapterForOperations()
	if resolveErr != nil && !strings.Contains(resolveErr.Error(), "no qualified backend") {
		fmt.Fprintf(stderr, "inspect active backend: %v\n", resolveErr)
		return exUnavailable
	}
	stop := func() error { return nil }
	if adapter != nil {
		stop = func() error { return adapter.Stop(context.Background(), false) }
	}
	if err := engine.Uninstall(engine.UninstallOptions{Root: dataDir(), PurgeData: *purge, Stop: stop}); err != nil {
		fmt.Fprintf(stderr, "uninstall: %v\n", err)
		return exSoftware
	}
	fmt.Fprintln(stdout, "Tusk configuration removed; workspaces and registered shares were preserved")
	return 0
}

func selectedAdapterForOperations() (engine.Adapter, string, error) {
	device, err := platform.Discover(dataDir())
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		return nil, "", err
	}
	selected := cfg.SelectedEngine
	if selected == "auto" {
		var report engine.TuneReport
		if err := state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report); err == nil {
			selected = report.Selected
		}
	}
	if selected == "" || selected == "auto" {
		return nil, "", engine.ErrNoQualifiedBackend
	}
	var candidate engine.Candidate
	for _, item := range engine.Catalog() {
		if item.ID == selected {
			candidate = item
			break
		}
	}
	if candidate.ID == "" {
		return nil, selected, fmt.Errorf("selected backend %q is unknown", selected)
	}
	resources := make(map[string]engine.BackendResources, len(cfg.Engines))
	for id, value := range cfg.Engines {
		resources[id] = engine.BackendResources{CPUs: value.CPUs, MemoryMiB: value.MemoryMiB}
	}
	factory := engine.NewLocalCandidateFactoryWithProbeAndResources(dataDir(), device, os.Getenv("TUSK_PROBE_IMAGE"), resources)
	target, err := factory(context.Background(), candidate)
	if err != nil {
		return nil, selected, err
	}
	adapterTarget, ok := target.(engine.AdapterTarget)
	if !ok {
		return nil, selected, errors.New("selected backend adapter is unavailable")
	}
	return adapterTarget.Adapter, selected, nil
}

func runUpdate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: tusk update check")
		return exUsage
	}
	var plan engine.UpdatePlan
	if err := state.ReadJSON(filepath.Join(dataDir(), "update-plan.json"), &plan); err != nil {
		fmt.Fprintf(stderr, "update check: %v\n", err)
		return exSoftware
	}
	return writeJSON(stdout, stderr, plan)
}

func runAcceptance(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "status" {
		fmt.Fprintln(stderr, "usage: tusk acceptance status")
		return exUsage
	}
	matrix, err := engine.LoadAcceptanceMatrix(filepath.Join(dataDir(), "acceptance.json"))
	if err != nil {
		fmt.Fprintf(stdout, "MVP acceptance: incomplete (%v)\n", err)
		return 0
	}
	if err := matrix.ValidateMVP(); err != nil {
		fmt.Fprintf(stdout, "MVP acceptance: incomplete (%v)\n", err)
		return 0
	}
	fmt.Fprintln(stdout, "MVP acceptance: complete")
	return 0
}

func terminalFile(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func terminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && terminalFile(file)
}

func runDevice(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("device", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return exUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk device [--json]")
		return exUsage
	}

	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	if *jsonOutput {
		return writeJSON(stdout, stderr, device)
	}
	fmt.Fprintf(stdout, "Host:        %s/%s\n", device.HostOS, device.HostArch)
	fmt.Fprintf(stdout, "Termux:      %t\n", device.Termux)
	fmt.Fprintf(stdout, "CPU count:   %d\n", device.CPUCount)
	fmt.Fprintf(stdout, "Memory:      %d bytes\n", device.MemoryTotalBytes)
	fmt.Fprintf(stdout, "Storage:     %d bytes available\n", device.StorageAvailableBytes)
	fmt.Fprintf(stdout, "Fingerprint: %s\n", device.Fingerprint)
	return 0
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "show" {
		fmt.Fprintln(stderr, "usage: tusk config show")
		return exUsage
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return exSoftware
	}
	return writeJSON(stdout, stderr, cfg)
}

func runEngine(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printEngineUsage(stderr)
		return exUsage
	}
	switch args[0] {
	case "candidates":
		return runEngineCandidates(args[1:], stdout, stderr)
	case "explain":
		return runEngineExplain(args[1:], stdout, stderr)
	case "tune":
		return runEngineTune(args[1:], stdout, stderr)
	case "list":
		return runEngineList(args[1:], stdout, stderr)
	case "current":
		return runEngineCurrent(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown engine command %q\n", args[0])
		printEngineUsage(stderr)
		return exUsage
	}
}

func runEngineTune(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("engine tune", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mode := flags.String("mode", "quick", "qualification mode: quick or thorough")
	policyName := flags.String("policy", "balanced", "ranking policy")
	probeImage := flags.String("probe-image", os.Getenv("TUSK_PROBE_IMAGE"), "digest-pinned OCI probe image")
	manifestPath := flags.String("probe-manifest", os.Getenv("TUSK_PROBE_MANIFEST"), "signed compact probe manifest JSON")
	publicKeyHex := flags.String("probe-public-key", os.Getenv("TUSK_PROBE_PUBLIC_KEY"), "trusted Ed25519 public key in hex")
	force := flags.Bool("force", false, "ignore resumable tune evidence")
	apply := flags.Bool("apply", false, "apply the highest-ranked qualified backend")
	jsonOutput := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return exUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk engine tune [--mode quick|thorough] [--policy balanced|speed|low-memory|compatibility] [--probe-image image@sha256:...] [--force] [--apply] [--json]")
		return exUsage
	}
	policy, err := engine.PolicyForName(*policyName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	cfg, err := config.Load(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return exSoftware
	}
	resources := make(map[string]engine.BackendResources, len(cfg.Engines))
	for id, value := range cfg.Engines {
		resources[id] = engine.BackendResources{CPUs: value.CPUs, MemoryMiB: value.MemoryMiB}
	}
	modeValue := engine.TuneMode(*mode)
	if modeValue != engine.TuneQuick && modeValue != engine.TuneThorough {
		fmt.Fprintf(stderr, "unsupported tune mode %q\n", *mode)
		return exUsage
	}
	if *manifestPath == "" || *publicKeyHex == "" {
		fmt.Fprintln(stderr, "tune engine: production qualification requires --probe-manifest and --probe-public-key")
		return exUsage
	}
	var manifest artifact.Manifest
	if err := state.ReadJSON(*manifestPath, &manifest); err != nil {
		fmt.Fprintf(stderr, "read probe manifest: %v\n", err)
		return exUsage
	}
	publicKey, err := hex.DecodeString(*publicKeyHex)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		fmt.Fprintln(stderr, "probe public key must be 32-byte Ed25519 key encoded as hex")
		return exUsage
	}
	report, err := engine.RunTune(context.Background(), device, engine.DiscoverCandidates(device), engine.NewLocalCandidateFactoryWithProbeAndResources(dataDir(), device, *probeImage, resources), engine.TuneOptions{
		Mode:           modeValue,
		Policy:         policy,
		ProbeImage:     *probeImage,
		Force:          *force,
		Apply:          *apply,
		EvidenceDir:    filepath.Join(dataDir(), "benchmarks", device.Fingerprint),
		ProbeManifest:  &manifest,
		ProbePublicKey: ed25519.PublicKey(publicKey),
	})
	if err != nil {
		fmt.Fprintf(stderr, "tune engine: %v\n", err)
		return exSoftware
	}
	if *jsonOutput {
		return writeJSON(stdout, stderr, report)
	}
	printTuneReport(stdout, report)
	return 0
}

func runEngineList(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("engine list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	ranked := flags.Bool("ranked", false, "show the latest ranked tune report")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk engine list [--ranked]")
		return exUsage
	}
	if !*ranked {
		return runEngineCandidates(nil, stdout, stderr)
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	var report engine.TuneReport
	if err := state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report); err != nil {
		fmt.Fprintf(stderr, "load ranked report: %v\n", err)
		return exSoftware
	}
	return writeJSON(stdout, stderr, report.Rankings)
}

func runEngineCurrent(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: tusk engine current")
		return exUsage
	}
	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	var report engine.TuneReport
	if err := state.ReadJSON(filepath.Join(dataDir(), "benchmarks", device.Fingerprint, "tune-report.json"), &report); err != nil {
		fmt.Fprintf(stderr, "load current engine: %v\n", err)
		return exSoftware
	}
	fmt.Fprintf(stdout, "Selected backend: %s\n", report.Selected)
	return 0
}

func printTuneReport(stdout io.Writer, report engine.TuneReport) {
	fmt.Fprintf(stdout, "Tune %s (%s, policy %s v%d)\n", report.RunID, report.Mode, report.Policy.Name, report.Policy.Version)
	for _, candidate := range report.Candidates {
		ranking := candidate.Ranking
		status := "rejected"
		if ranking.Qualified {
			status = fmt.Sprintf("qualified score %.2f", ranking.Score)
		}
		fmt.Fprintf(stdout, "%-28s %s\n", ranking.CandidateID, status)
		metrics := candidate.Evidence.Metrics
		if ranking.Qualified {
			metrics = ranking.Metrics
		}
		fmt.Fprintf(stdout, "  raw metrics: latency=%.2fms throughput=%.2f/s memory=%.2fMiB startupShutdown=%.2fms storage=%.2fMiB thermal=%.3f reliability=%.3f\n", metrics.InteractiveLatencyMillis, metrics.WorkloadThroughput, metrics.PeakMemoryMiB, metrics.StartupShutdownMillis, metrics.PersistentStorageMiB, metrics.ThermalRetention, candidate.Evidence.Reliability)
		if ranking.Qualified {
			fmt.Fprintf(stdout, "  normalized components: %v\n", ranking.Components)
		}
		if len(candidate.Evidence.RejectionReasons) > 0 {
			fmt.Fprintf(stdout, "  rejection: %s\n", strings.Join(candidate.Evidence.RejectionReasons, "; "))
		}
	}
	if report.Selected == "" {
		fmt.Fprintln(stdout, "No qualified backend selected. No Podman operation was simulated.")
	} else {
		fmt.Fprintf(stdout, "Selected backend: %s\n", report.Selected)
	}
}

func runEngineCandidates(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("engine candidates", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return exUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tusk engine candidates [--json]")
		return exUsage
	}

	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	candidates := engine.DiscoverCandidates(device)
	if *jsonOutput {
		return writeJSON(stdout, stderr, candidates)
	}
	fmt.Fprintln(stdout, "CANDIDATE                    AVAILABLE  STATUS")
	for _, candidate := range candidates {
		status := "unqualified"
		if !candidate.Applicable {
			status = "not applicable"
		} else if !candidate.Available {
			status = "missing dependencies"
		}
		fmt.Fprintf(stdout, "%-28s %-10t %s\n", candidate.Candidate.ID, candidate.Available, status)
	}
	fmt.Fprintln(stdout, "\nAvailability is discovery only. No backend is qualified yet.")
	return 0
}

func runEngineExplain(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("engine explain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return exUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: tusk engine explain [--json] <candidate>")
		return exUsage
	}

	device, err := platform.Discover(dataDir())
	if err != nil {
		fmt.Fprintf(stderr, "discover device: %v\n", err)
		return exSoftware
	}
	explanation, err := engine.ExplainCandidate(device, flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exUsage
	}
	if *jsonOutput {
		return writeJSON(stdout, stderr, explanation)
	}
	fmt.Fprintf(stdout, "Candidate:              %s\n", explanation.Candidate.ID)
	fmt.Fprintf(stdout, "Kind/architecture:      %s/%s\n", explanation.Candidate.Kind, explanation.Candidate.Architecture)
	fmt.Fprintf(stdout, "Applicable:             %t\n", explanation.Preflight.Applicable)
	fmt.Fprintf(stdout, "Dependencies available: %t\n", explanation.Preflight.Available)
	fmt.Fprintf(stdout, "Qualification:          %s\n", explanation.Qualification)
	fmt.Fprintf(stdout, "Conformance contract:   v%d (%d checks)\n", explanation.Conformance.Version, len(explanation.Conformance.Checks))
	fmt.Fprintf(stdout, "Benchmark contract:     v%d (%d metrics)\n", explanation.Benchmark.Version, len(explanation.Benchmark.Metrics))
	fmt.Fprintf(stdout, "Scoring policy:         %s v%d\n", explanation.Policy.Name, explanation.Policy.Version)
	if len(explanation.Preflight.MissingCommands) > 0 {
		fmt.Fprintf(stdout, "Missing commands:       %v\n", explanation.Preflight.MissingCommands)
		fmt.Fprintf(stdout, "Install guidance:       %s\n", explanation.Candidate.InstallHint)
	}
	fmt.Fprintln(stdout, "\nPreflight does not qualify a backend. Qualification starts in M2.")
	return 0
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "encode output: %v\n", err)
		return exSoftware
	}
	return 0
}

func dataDir() string {
	if value := os.Getenv("TUSK_HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".tusk"
	}
	return filepath.Join(home, ".tusk")
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: tusk <command>")
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "commands:")
	fmt.Fprintln(writer, "  version                    print host and spec versions")
	fmt.Fprintln(writer, "  device [--json]            inspect the current device")
	fmt.Fprintln(writer, "  config show                print effective configuration")
	fmt.Fprintln(writer, "  engine candidates [--json] discover backend candidates")
	fmt.Fprintln(writer, "  engine explain <candidate> explain preflight and contracts")
	fmt.Fprintln(writer, "  engine tune [options]      run real qualification and benchmark")
	fmt.Fprintln(writer, "  engine list [--ranked]     list candidates or ranked evidence")
	fmt.Fprintln(writer, "  engine current             show selected backend evidence")
	fmt.Fprintln(writer, "  podman <args...>           forward to real Podman in selected backend")
	fmt.Fprintln(writer, "  compose <args...>          explicit Compose forwarding fallback")
	fmt.Fprintln(writer, "  compose features           show the tested provider feature matrix")
	fmt.Fprintln(writer, "  status                     show selected backend and qualification")
	fmt.Fprintln(writer, "  env                       print the protected Podman API endpoint")
	fmt.Fprintln(writer, "  share list|add|remove     manage explicit host shares")
	fmt.Fprintln(writer, "  doctor [--json]           run non-destructive diagnostics")
	fmt.Fprintln(writer, "  backup --backend ...      create a verified backend backup")
	fmt.Fprintln(writer, "  uninstall [--purge-data] remove Tusk safely")
	fmt.Fprintln(writer, "  update check              show staged update plan")
	fmt.Fprintln(writer, "  acceptance status         show release acceptance evidence")
}

func printEngineUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: tusk engine <command>")
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "commands:")
	fmt.Fprintln(writer, "  candidates [--json]")
	fmt.Fprintln(writer, "  explain [--json] <candidate>")
	fmt.Fprintln(writer, "  tune [options]")
	fmt.Fprintln(writer, "  list [--ranked]")
	fmt.Fprintln(writer, "  current")
}
