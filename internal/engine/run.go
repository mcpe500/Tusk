package engine

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mcpe500/Tusk/internal/artifact"
	"github.com/mcpe500/Tusk/internal/platform"
	"github.com/mcpe500/Tusk/internal/state"
)

type TuneMode string

const (
	TuneQuick                  TuneMode = "quick"
	TuneThorough               TuneMode = "thorough"
	HostVersion                         = "0.1.0-dev"
	DefaultStorageBudgetBytes  uint64   = 512 << 20
	DefaultDownloadBudgetBytes uint64   = 256 << 20
	DefaultThermalRetention    float64  = 0.80
)

type TuneOptions struct {
	Mode                    TuneMode
	Policy                  Policy
	ProbeImage              string
	Force                   bool
	Apply                   bool
	RunID                   string
	EvidenceDir             string
	Timeout                 time.Duration
	StorageBudgetBytes      uint64
	DownloadBudgetBytes     uint64
	MinimumThermalRetention float64
	ProbeManifest           *artifact.Manifest
	ProbePublicKey          ed25519.PublicKey
}

type CandidateEvidence struct {
	Candidate Candidate `json:"candidate"`
	Discovery Discovery `json:"discovery"`
	Evidence  Evidence  `json:"evidence"`
	Ranking   Ranking   `json:"ranking"`
	Error     string    `json:"error,omitempty"`
}

type TuneReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	RunID         string              `json:"runId"`
	StartedAt     time.Time           `json:"startedAt"`
	FinishedAt    time.Time           `json:"finishedAt"`
	Mode          TuneMode            `json:"mode"`
	Policy        Policy              `json:"policy"`
	Device        platform.Device     `json:"device"`
	Candidates    []CandidateEvidence `json:"candidates"`
	Rankings      []Ranking           `json:"rankings"`
	Selected      string              `json:"selected,omitempty"`
	Applied       bool                `json:"applied"`
	Interrupted   bool                `json:"interrupted,omitempty"`
}

type TuneJournal struct {
	SchemaVersion int       `json:"schemaVersion"`
	RunID         string    `json:"runId"`
	CandidateIDs  []string  `json:"candidateIds"`
	Completed     []string  `json:"completed"`
	StartedAt     time.Time `json:"startedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Interrupted   bool      `json:"interrupted"`
}

const runSchemaVersion = 1

func ValidateTuneOptions(options TuneOptions) error {
	if options.Mode != TuneQuick && options.Mode != TuneThorough {
		return fmt.Errorf("unsupported tune mode %q", options.Mode)
	}
	if options.Policy.Name == "" {
		return errors.New("tune policy is required")
	}
	if options.ProbeImage == "" {
		return errors.New("a digest-pinned probe image is required")
	}
	if !containsAtSHA256(options.ProbeImage) {
		return errors.New("probe image must be pinned by digest")
	}
	if options.ProbeManifest != nil && len(options.ProbePublicKey) != ed25519.PublicKeySize {
		return errors.New("signed probe manifest requires a valid Ed25519 public key")
	}
	if options.EvidenceDir == "" {
		return errors.New("evidence directory is required")
	}
	return nil
}

func NewTuneJournal(runID string, candidateIDs []string) TuneJournal {
	return TuneJournal{SchemaVersion: runSchemaVersion, RunID: runID, CandidateIDs: append([]string(nil), candidateIDs...), StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}

func SaveTuneJournal(path string, journal TuneJournal) error {
	if journal.SchemaVersion != runSchemaVersion || journal.RunID == "" {
		return errors.New("invalid tune journal")
	}
	journal.UpdatedAt = time.Now().UTC()
	return state.WriteJSON(path, journal, 0o600)
}

func LoadTuneJournal(path string) (TuneJournal, error) {
	var journal TuneJournal
	if err := state.ReadJSON(path, &journal); err != nil {
		return TuneJournal{}, err
	}
	if journal.SchemaVersion != runSchemaVersion || journal.RunID == "" {
		return TuneJournal{}, errors.New("unsupported or invalid tune journal")
	}
	return journal, nil
}

func EvidencePath(directory, candidateID string) string {
	return filepath.Join(directory, candidateID+".json")
}

func SaveEvidence(directory string, evidence Evidence) error {
	if evidence.SchemaVersion != EvidenceSchemaVersion || evidence.CandidateID == "" {
		return errors.New("invalid evidence")
	}
	return state.WriteJSON(EvidencePath(directory, evidence.CandidateID), evidence, 0o600)
}

func LoadEvidence(directory, candidateID string) (Evidence, error) {
	var evidence Evidence
	if err := state.ReadJSON(EvidencePath(directory, candidateID), &evidence); err != nil {
		return Evidence{}, err
	}
	if evidence.SchemaVersion != EvidenceSchemaVersion || evidence.CandidateID != candidateID {
		return Evidence{}, errors.New("evidence schema or candidate mismatch")
	}
	return evidence, nil
}

func SelectQualified(rankings []Ranking) (Ranking, error) {
	for _, ranking := range rankings {
		if ranking.Qualified {
			return ranking, nil
		}
	}
	return Ranking{}, errors.New("no qualified backend is available")
}
