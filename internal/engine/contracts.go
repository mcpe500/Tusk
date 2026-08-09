package engine

import (
	"errors"

	"github.com/mcpe500/Tusk/internal/platform"
)

const (
	EvidenceSchemaVersion   = 1
	ConformanceSuiteVersion = 1
	BenchmarkSuiteVersion   = 1
)

type ConformanceCheck struct {
	ID          string `json:"id"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type ConformanceContract struct {
	Version int                `json:"version"`
	Checks  []ConformanceCheck `json:"checks"`
}

type BenchmarkMetric struct {
	ID             string `json:"id"`
	Unit           string `json:"unit"`
	HigherIsBetter bool   `json:"higherIsBetter"`
	Description    string `json:"description"`
}

type BenchmarkContract struct {
	Version int               `json:"version"`
	Metrics []BenchmarkMetric `json:"metrics"`
}

type Explanation struct {
	Candidate     Candidate           `json:"candidate"`
	Preflight     Discovery           `json:"preflight"`
	Qualification string              `json:"qualification"`
	Conformance   ConformanceContract `json:"conformance"`
	Benchmark     BenchmarkContract   `json:"benchmark"`
	Policy        Policy              `json:"policy"`
}

func ConformanceContractV1() ConformanceContract {
	return ConformanceContract{
		Version: ConformanceSuiteVersion,
		Checks: []ConformanceCheck{
			{ID: "podman.info", Required: true, Description: "Podman reports a usable engine and storage backend"},
			{ID: "image.pull", Required: true, Description: "Pull a pinned OCI probe image"},
			{ID: "container.run", Required: true, Description: "Run a real container and verify its exit code and output"},
			{ID: "container.exec", Required: true, Description: "Execute a command in a running container"},
			{ID: "network.dns", Required: true, Description: "Resolve DNS from a real container"},
			{ID: "network.tcp", Required: true, Description: "Reach an outbound TCP endpoint from a real container"},
			{ID: "volume.persistence", Required: true, Description: "Persist data through container recreation"},
			{ID: "build.containerfile", Required: true, Description: "Build and run a minimal pinned Containerfile"},
			{ID: "lifecycle.restart", Required: true, Description: "Preserve Podman data across backend restart"},
			{ID: "cleanup", Required: true, Description: "Remove all probe resources without affecting user state"},
		},
	}
}

func BenchmarkContractV1() BenchmarkContract {
	return BenchmarkContract{
		Version: BenchmarkSuiteVersion,
		Metrics: []BenchmarkMetric{
			{ID: "reliability", Unit: "ratio", HigherIsBetter: true, Description: "Successful repeated lifecycle and workload runs"},
			{ID: "interactiveLatency", Unit: "milliseconds", Description: "Podman command round-trip latency"},
			{ID: "throughput", Unit: "operationsPerSecond", HigherIsBetter: true, Description: "Normalized pull, unpack, run, and build throughput"},
			{ID: "memory", Unit: "MiB", Description: "Peak host memory used by the backend"},
			{ID: "startupShutdown", Unit: "milliseconds", Description: "Cold start plus graceful shutdown duration"},
			{ID: "storage", Unit: "MiB", Description: "Persistent storage overhead after the probe workload"},
			{ID: "thermal", Unit: "ratio", HigherIsBetter: true, Description: "Performance retained during repeated workload execution"},
		},
	}
}

func ExplainCandidate(device platform.Device, id string) (Explanation, error) {
	for _, discovery := range DiscoverCandidates(device) {
		if discovery.Candidate.ID != id {
			continue
		}
		return Explanation{
			Candidate:     discovery.Candidate,
			Preflight:     discovery,
			Qualification: "not-run",
			Conformance:   ConformanceContractV1(),
			Benchmark:     BenchmarkContractV1(),
			Policy:        BalancedPolicy(),
		}, nil
	}
	return Explanation{}, errors.New("unknown backend candidate " + id)
}
