package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestRunTuneAttemptsUnavailableCandidatesAndPersistsRejection(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	device := platform.Device{HostArch: "arm64", Fingerprint: "device-test", StorageAvailableBytes: 1 << 30, Commands: map[string]string{}}
	discoveries := DiscoverCandidates(device)
	report, err := RunTune(context.Background(), device, discoveries, nil, TuneOptions{
		Mode:        TuneQuick,
		Policy:      BalancedPolicy(),
		ProbeImage:  "image@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RunID:       "tune-test",
		EvidenceDir: directory,
	})
	if err != nil {
		t.Fatalf("RunTune() error = %v", err)
	}
	if report.Selected != "" || len(report.Candidates) != len(discoveries) {
		t.Fatalf("unexpected tune report: %#v", report)
	}
	if _, err := LoadEvidence(directory, "proot-native"); err != nil {
		t.Fatalf("LoadEvidence() error = %v", err)
	}
	if _, err := LoadTuneJournal(filepath.Join(directory, "tune-journal.json")); err != nil {
		t.Fatalf("LoadTuneJournal() error = %v", err)
	}
}
