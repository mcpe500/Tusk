package engine

import "testing"

func TestRankRejectsFailedConformance(t *testing.T) {
	t.Parallel()
	good := validEvidence("qemu-system-aarch64")
	bad := validEvidence("proot-native")
	bad.RequiredConformancePass = false

	ranked, err := Rank([]Evidence{bad, good}, BalancedPolicy())
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if ranked[0].CandidateID != good.CandidateID || !ranked[0].Qualified {
		t.Fatalf("first ranking = %#v", ranked[0])
	}
	if ranked[1].Qualified || len(ranked[1].Reasons) == 0 {
		t.Fatalf("failed candidate was not rejected: %#v", ranked[1])
	}
}

func TestRankUsesBalancedMeasurements(t *testing.T) {
	t.Parallel()
	fast := validEvidence("proot-native")
	fast.Metrics.InteractiveLatencyMillis = 10
	fast.Metrics.StartupShutdownMillis = 50
	fast.Metrics.PeakMemoryMiB = 100
	fast.Metrics.PersistentStorageMiB = 100

	slow := validEvidence("qemu-system-aarch64")
	slow.Metrics.InteractiveLatencyMillis = 100
	slow.Metrics.StartupShutdownMillis = 1000
	slow.Metrics.PeakMemoryMiB = 700
	slow.Metrics.PersistentStorageMiB = 900

	ranked, err := Rank([]Evidence{slow, fast}, BalancedPolicy())
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if ranked[0].CandidateID != "proot-native" {
		t.Fatalf("best candidate = %q, want proot-native; rankings=%#v", ranked[0].CandidateID, ranked)
	}
}

func TestRankTieBreaksByMemory(t *testing.T) {
	t.Parallel()
	lessMemory := validEvidence("candidate-b")
	lessMemory.Metrics.PeakMemoryMiB = 100
	moreMemory := validEvidence("candidate-a")
	moreMemory.Metrics.PeakMemoryMiB = 200

	ranked, err := Rank([]Evidence{moreMemory, lessMemory}, BalancedPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if ranked[0].CandidateID != "candidate-b" {
		t.Fatalf("tie-break selected %q", ranked[0].CandidateID)
	}
}

func validEvidence(id string) Evidence {
	return Evidence{
		CandidateID:             id,
		RequiredConformancePass: true,
		PlatformCompatible:      true,
		Reliability:             1,
		Metrics: Metrics{
			InteractiveLatencyMillis: 50,
			WorkloadThroughput:       100,
			PeakMemoryMiB:            500,
			StartupShutdownMillis:    500,
			PersistentStorageMiB:     500,
			ThermalRetention:         1,
		},
	}
}
