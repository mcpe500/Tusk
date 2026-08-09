package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mcpe500/Tusk/internal/config"
	"github.com/mcpe500/Tusk/internal/engine"
	"github.com/mcpe500/Tusk/internal/platform"
)

const (
	version     = "0.1.0-dev"
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
	default:
		fmt.Fprintf(stderr, "unknown engine command %q\n", args[0])
		printEngineUsage(stderr)
		return exUsage
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
}

func printEngineUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: tusk engine <command>")
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "commands:")
	fmt.Fprintln(writer, "  candidates [--json]")
	fmt.Fprintln(writer, "  explain [--json] <candidate>")
}
