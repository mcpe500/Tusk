package engine

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/mcpe500/Tusk/internal/artifact"
)

const MaxProbeArtifactBytes int64 = 256 << 20

type ProbeDescriptor struct {
	ID           string `json:"id"`
	CandidateID  string `json:"candidateId"`
	Architecture string `json:"architecture"`
	ArtifactID   string `json:"artifactId"`
	Version      string `json:"version"`
}

// VerifyProbeManifest verifies the release signature before selecting a probe.
// A valid checksum without a valid signature is never sufficient.
func VerifyProbeManifest(manifest artifact.Manifest, publicKey ed25519.PublicKey, candidateID, architecture string) (artifact.Artifact, error) {
	if err := manifest.Verify(publicKey); err != nil {
		return artifact.Artifact{}, fmt.Errorf("verify probe manifest: %w", err)
	}
	for _, item := range manifest.Artifacts {
		if item.Backend == candidateID && item.Architecture == architecture {
			if item.Size > MaxProbeArtifactBytes {
				return artifact.Artifact{}, fmt.Errorf("probe artifact %q exceeds compact size limit", item.ID)
			}
			return item, nil
		}
	}
	return artifact.Artifact{}, fmt.Errorf("no signed probe artifact for %s/%s", candidateID, architecture)
}

func ValidateProbeDescriptor(descriptor ProbeDescriptor) error {
	if descriptor.ID == "" || descriptor.CandidateID == "" || descriptor.Architecture == "" || descriptor.ArtifactID == "" || descriptor.Version == "" {
		return errors.New("probe descriptor requires id, candidateId, architecture, artifactId, and version")
	}
	return nil
}
