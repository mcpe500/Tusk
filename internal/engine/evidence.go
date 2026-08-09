package engine

import "time"

type Evidence struct {
	SchemaVersion           int               `json:"schemaVersion"`
	CandidateID             string            `json:"candidateId"`
	Fingerprint             string            `json:"fingerprint"`
	DeviceFingerprint       string            `json:"deviceFingerprint"`
	CollectedAt             time.Time         `json:"collectedAt"`
	ConformanceVersion      int               `json:"conformanceVersion"`
	BenchmarkVersion        int               `json:"benchmarkVersion"`
	PolicyName              string            `json:"policyName"`
	PolicyVersion           int               `json:"policyVersion"`
	RequiredConformancePass bool              `json:"requiredConformancePass"`
	PlatformCompatible      bool              `json:"platformCompatible"`
	Reliability             float64           `json:"reliability"`
	Metrics                 Metrics           `json:"metrics"`
	Conformance             ConformanceResult `json:"conformance"`
	Benchmark               BenchmarkResult   `json:"benchmark"`
	RejectionReasons        []string          `json:"rejectionReasons,omitempty"`
}

type Metrics struct {
	InteractiveLatencyMillis float64 `json:"interactiveLatencyMillis"`
	WorkloadThroughput       float64 `json:"workloadThroughput"`
	PeakMemoryMiB            float64 `json:"peakMemoryMiB"`
	StartupShutdownMillis    float64 `json:"startupShutdownMillis"`
	PersistentStorageMiB     float64 `json:"persistentStorageMiB"`
	ThermalRetention         float64 `json:"thermalRetention"`
}

type Ranking struct {
	CandidateID string             `json:"candidateId"`
	Qualified   bool               `json:"qualified"`
	Score       float64            `json:"score,omitempty"`
	Components  map[string]float64 `json:"components,omitempty"`
	Reasons     []string           `json:"reasons,omitempty"`
	Metrics     Metrics            `json:"metrics"`
}

type Policy struct {
	Name           string             `json:"name"`
	Version        int                `json:"version"`
	MinReliability float64            `json:"minReliability"`
	Weights        map[string]float64 `json:"weights"`
}

func BalancedPolicy() Policy {
	return Policy{
		Name:           "balanced",
		Version:        1,
		MinReliability: 1,
		Weights: map[string]float64{
			"reliability":        0.25,
			"interactiveLatency": 0.20,
			"throughput":         0.15,
			"memory":             0.15,
			"startupShutdown":    0.10,
			"storage":            0.10,
			"thermal":            0.05,
		},
	}
}
