package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
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
	version     = engine.HostVersion
	specVersion = "0.2.0"
	exUsage     = 64
	exSoftware  = 70
)

func main() {
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
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exUsage
	}
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
	report, err := engine.RunTune(context.Background(), device, engine.DiscoverCandidates(device), nil, engine.TuneOptions{
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
