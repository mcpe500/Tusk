package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mcpe500/Tusk/internal/platform"
	"github.com/mcpe500/Tusk/internal/state"
)

type CandidateTargetFactory func(context.Context, Candidate) (QualificationTarget, error)

// RunTune attempts every applicable candidate. Missing dependencies and target
// construction failures are retained as rejected evidence instead of being
// silently omitted from the report.
func RunTune(ctx context.Context, device platform.Device, discoveries []Discovery, factory CandidateTargetFactory, options TuneOptions) (TuneReport, error) {
	if err := ValidateTuneOptions(options); err != nil {
		return TuneReport{}, err
	}
	if err := os.MkdirAll(options.EvidenceDir, 0o700); err != nil {
		return TuneReport{}, fmt.Errorf("create evidence directory: %w", err)
	}
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Minute
	}
	if options.StorageBudgetBytes == 0 {
		options.StorageBudgetBytes = DefaultStorageBudgetBytes
	}
	if options.DownloadBudgetBytes == 0 {
		options.DownloadBudgetBytes = DefaultDownloadBudgetBytes
	}
	if options.MinimumThermalRetention == 0 {
		options.MinimumThermalRetention = DefaultThermalRetention
	}
	storageBudgetRejection := ""
	if device.StorageAvailableBytes == 0 {
		storageBudgetRejection = "storage availability is unknown; tune was not attempted"
	} else if device.StorageAvailableBytes < options.StorageBudgetBytes {
		storageBudgetRejection = fmt.Sprintf("insufficient storage for tune: %d bytes available, %d required", device.StorageAvailableBytes, options.StorageBudgetBytes)
	}
	if options.RunID == "" {
		options.RunID = defaultTuneRunID(device, options)
	}
	if len(discoveries) == 0 {
		return TuneReport{}, errors.New("no candidates to tune")
	}
	ids := make([]string, 0, len(discoveries))
	for _, discovery := range discoveries {
		ids = append(ids, discovery.Candidate.ID)
	}
	journalPath := filepath.Join(options.EvidenceDir, "tune-journal.json")
	journal := NewTuneJournal(options.RunID, ids)
	if existing, err := LoadTuneJournal(journalPath); err == nil && existing.RunID == options.RunID && !options.Force {
		journal = existing
	}
	if err := SaveTuneJournal(journalPath, journal); err != nil {
		return TuneReport{}, err
	}

	report := TuneReport{SchemaVersion: runSchemaVersion, RunID: options.RunID, StartedAt: time.Now().UTC(), Mode: options.Mode, Policy: options.Policy, Device: device}
	var evidence []Evidence
	for _, discovery := range discoveries {
		cacheFingerprint, fingerprintErr := BuildEvidenceFingerprint(device, discovery.Candidate, options.ProbeImage, options.Policy)
		if fingerprintErr != nil {
			return report, fingerprintErr
		}
		if containsString(journal.Completed, discovery.Candidate.ID) && !options.Force {
			item, err := LoadEvidence(options.EvidenceDir, discovery.Candidate.ID)
			if err == nil && item.Fingerprint == cacheFingerprint {
				evidence = append(evidence, item)
				if err := ctx.Err(); err != nil {
					journal.Interrupted = true
					if journalErr := SaveTuneJournal(journalPath, journal); journalErr != nil {
						return report, fmt.Errorf("tune interrupted and journal save failed: %w (journal: %v)", err, journalErr)
					}
					report.Interrupted = true
					report.FinishedAt = time.Now().UTC()
					if reportErr := SaveTuneReport(filepath.Join(options.EvidenceDir, "tune-report.json"), report); reportErr != nil {
						return report, fmt.Errorf("tune interrupted and report save failed: %w (report: %v)", err, reportErr)
					}
					return report, err
				}
				continue
			}
		}
		item := Evidence{
			SchemaVersion:      EvidenceSchemaVersion,
			CandidateID:        discovery.Candidate.ID,
			Fingerprint:        cacheFingerprint,
			DeviceFingerprint:  device.Fingerprint,
			CollectedAt:        time.Now().UTC(),
			ConformanceVersion: ConformanceSuiteVersion,
			BenchmarkVersion:   BenchmarkSuiteVersion,
			PolicyName:         options.Policy.Name,
			PolicyVersion:      options.Policy.Version,
			PlatformCompatible: discovery.Applicable,
		}
		probeRejected := storageBudgetRejection != ""
		if storageBudgetRejection != "" {
			item.RejectionReasons = append(item.RejectionReasons, storageBudgetRejection)
		}
		if options.ProbeManifest != nil {
			architecture := discovery.Candidate.Architecture
			if architecture == "native" {
				architecture = device.HostArch
			}
			probeArtifact, err := VerifyProbeManifest(*options.ProbeManifest, options.ProbePublicKey, discovery.Candidate.ID, architecture)
			if err != nil {
				item.RejectionReasons = []string{err.Error()}
				probeRejected = true
			} else if uint64(probeArtifact.Size) > options.DownloadBudgetBytes {
				item.RejectionReasons = []string{fmt.Sprintf("probe download exceeds budget: %d bytes > %d", probeArtifact.Size, options.DownloadBudgetBytes)}
				probeRejected = true
			}
		}
		if !discovery.Applicable || !discovery.Available {
			item.RejectionReasons = append(item.RejectionReasons, discovery.Reasons...)
			if len(discovery.MissingCommands) > 0 {
				item.RejectionReasons = append(item.RejectionReasons, "missing commands: "+fmt.Sprint(discovery.MissingCommands))
			}
		} else if probeRejected {
			// A candidate without a verified compact probe must not execute.
		} else if factory != nil && options.ProbeManifest == nil {
			item.RejectionReasons = []string{"signed probe manifest is required before real qualification"}
		} else if factory == nil {
			item.RejectionReasons = []string{"no real backend adapter factory was provided"}
		} else {
			item = qualifyTarget(ctx, discovery, factory, item, options)
		}
		if err := SaveEvidence(options.EvidenceDir, item); err != nil {
			return report, err
		}
		evidence = append(evidence, item)
		if err := ctx.Err(); err != nil {
			journal.Interrupted = true
			if journalErr := SaveTuneJournal(journalPath, journal); journalErr != nil {
				return report, fmt.Errorf("tune interrupted and journal save failed: %w (journal: %v)", err, journalErr)
			}
			report.Interrupted = true
			report.FinishedAt = time.Now().UTC()
			if reportErr := SaveTuneReport(filepath.Join(options.EvidenceDir, "tune-report.json"), report); reportErr != nil {
				return report, fmt.Errorf("tune interrupted and report save failed: %w (report: %v)", err, reportErr)
			}
			return report, err
		}
		journal.Completed = appendUnique(journal.Completed, discovery.Candidate.ID)
		if err := SaveTuneJournal(journalPath, journal); err != nil {
			return report, err
		}
	}

	rankings, err := Rank(evidence, options.Policy)
	if err != nil {
		return report, err
	}
	report.Rankings = rankings
	byID := make(map[string]Ranking, len(rankings))
	for _, ranking := range rankings {
		byID[ranking.CandidateID] = ranking
	}
	for _, discovery := range discoveries {
		evidenceItem, _ := findEvidence(evidence, discovery.Candidate.ID)
		report.Candidates = append(report.Candidates, CandidateEvidence{Candidate: discovery.Candidate, Discovery: discovery, Evidence: evidenceItem, Ranking: byID[discovery.Candidate.ID]})
	}
	if selected, err := SelectQualified(rankings); err == nil {
		report.Selected = selected.CandidateID
		report.Applied = options.Apply
	}
	report.FinishedAt = time.Now().UTC()
	if err := SaveTuneReport(filepath.Join(options.EvidenceDir, "tune-report.json"), report); err != nil {
		return report, err
	}
	return report, nil
}

func defaultTuneRunID(device platform.Device, options TuneOptions) string {
	value := device.Fingerprint + "|" + string(options.Mode) + "|" + options.Policy.Name + "|" + options.ProbeImage
	digest := sha256.Sum256([]byte(value))
	return "tune-" + hex.EncodeToString(digest[:8])
}

func qualifyTarget(ctx context.Context, discovery Discovery, factory CandidateTargetFactory, evidence Evidence, options TuneOptions) Evidence {
	target, err := factory(ctx, discovery.Candidate)
	if err != nil {
		evidence.RejectionReasons = []string{fmt.Sprintf("create adapter: %v", err)}
		return evidence
	}
	conformance, err := RunConformance(ctx, target, target, ConformanceContractV1(), ConformanceOptions{CandidateID: discovery.Candidate.ID, ProbeImage: options.ProbeImage, RunID: options.RunID + "-" + discovery.Candidate.ID, Timeout: options.Timeout})
	if err != nil {
		evidence.RejectionReasons = []string{fmt.Sprintf("conformance setup: %v", err)}
		return evidence
	}
	evidence.Conformance = conformance
	evidence.RequiredConformancePass = conformance.Passed
	if !conformance.Passed {
		evidence.RejectionReasons = []string{"required Podman conformance failed"}
		return evidence
	}
	repetitions := 1
	if options.Mode == TuneThorough {
		repetitions = 3
	}
	reliability, err := RunReliability(ctx, target, ReliabilityOptions{ProbeImage: options.ProbeImage, Repetitions: repetitions, Timeout: options.Timeout})
	if err != nil {
		evidence.RejectionReasons = []string{fmt.Sprintf("reliability: %v", err)}
		return evidence
	}
	evidence.Reliability = reliability.Ratio
	benchmark, err := RunBenchmark(ctx, target, BenchmarkContractV1(), BenchmarkOptions{ProbeImage: options.ProbeImage, Mode: string(options.Mode), Repetitions: repetitions, Warmup: 1, Timeout: options.Timeout})
	if err != nil {
		evidence.RejectionReasons = []string{fmt.Sprintf("benchmark: %v", err)}
		return evidence
	}
	evidence.Benchmark = benchmark
	evidence.Metrics = benchmark.Metrics
	if benchmark.Metrics.ThermalRetention < options.MinimumThermalRetention {
		evidence.RejectionReasons = []string{fmt.Sprintf("thermal retention %.3f is below budget %.3f", benchmark.Metrics.ThermalRetention, options.MinimumThermalRetention)}
	}
	return evidence
}

func SaveTuneReport(path string, report TuneReport) error {
	if err := ValidateTuneReport(report); err != nil {
		return err
	}
	return state.WriteJSON(path, report, 0o600)
}

func ValidateTuneReport(report TuneReport) error {
	if report.SchemaVersion != runSchemaVersion || report.RunID == "" {
		return errors.New("invalid tune report")
	}
	if report.FinishedAt.IsZero() {
		return errors.New("tune report is unfinished")
	}
	return nil
}

func findEvidence(items []Evidence, candidateID string) (Evidence, bool) {
	for _, item := range items {
		if item.CandidateID == candidateID {
			return item, true
		}
	}
	return Evidence{}, false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}
