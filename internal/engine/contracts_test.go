package engine

import (
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestConformanceContractV1HasUniqueRequiredChecks(t *testing.T) {
	t.Parallel()
	contract := ConformanceContractV1()
	if contract.Version != ConformanceSuiteVersion {
		t.Fatalf("Version = %d, want %d", contract.Version, ConformanceSuiteVersion)
	}
	seen := make(map[string]struct{}, len(contract.Checks))
	for _, check := range contract.Checks {
		if check.ID == "" || !check.Required || check.Description == "" {
			t.Fatalf("invalid conformance check: %#v", check)
		}
		if _, ok := seen[check.ID]; ok {
			t.Fatalf("duplicate conformance check %q", check.ID)
		}
		seen[check.ID] = struct{}{}
	}
	if len(seen) != 10 {
		t.Fatalf("check count = %d, want 10", len(seen))
	}
}

func TestBenchmarkContractMatchesScoringComponents(t *testing.T) {
	t.Parallel()
	contract := BenchmarkContractV1()
	if contract.Version != BenchmarkSuiteVersion {
		t.Fatalf("Version = %d, want %d", contract.Version, BenchmarkSuiteVersion)
	}
	seen := make(map[string]struct{}, len(contract.Metrics))
	for _, metric := range contract.Metrics {
		seen[metric.ID] = struct{}{}
	}
	for _, component := range scoreComponents {
		if _, ok := seen[component]; !ok {
			t.Fatalf("benchmark contract is missing scoring component %q", component)
		}
	}
}

func TestExplainCandidateIsPreflightOnly(t *testing.T) {
	t.Parallel()
	device := platform.Device{
		HostArch: "arm64",
		Commands: map[string]string{"proot": "/bin/proot"},
	}
	explanation, err := ExplainCandidate(device, "proot-native")
	if err != nil {
		t.Fatalf("ExplainCandidate() error = %v", err)
	}
	if !explanation.Preflight.Available || explanation.Qualification != "not-run" {
		t.Fatalf("unexpected explanation: %#v", explanation)
	}
	if explanation.Policy.Version != 1 || explanation.Conformance.Version != 1 || explanation.Benchmark.Version != 1 {
		t.Fatalf("contracts are not versioned: %#v", explanation)
	}
}
