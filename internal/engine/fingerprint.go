package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mcpe500/Tusk/internal/platform"
)

type EvidenceFingerprint struct {
	TuskVersion        string   `json:"tuskVersion"`
	DeviceFingerprint  string   `json:"deviceFingerprint"`
	HostOS             string   `json:"hostOS"`
	HostArch           string   `json:"hostArch"`
	KernelRelease      string   `json:"kernelRelease"`
	TermuxPrefix       string   `json:"termuxPrefix"`
	CandidateID        string   `json:"candidateId"`
	BackendBinaries    []string `json:"backendBinaries"`
	ProbeImage         string   `json:"probeImage"`
	PolicyName         string   `json:"policyName"`
	PolicyVersion      int      `json:"policyVersion"`
	ConformanceVersion int      `json:"conformanceVersion"`
	BenchmarkVersion   int      `json:"benchmarkVersion"`
}

func BuildEvidenceFingerprint(device platform.Device, candidate Candidate, probeImage string, policy Policy) (string, error) {
	binaries := make([]string, 0, len(candidate.RequiredCommands))
	for _, command := range candidate.RequiredCommands {
		path := device.Commands[command]
		digest := "missing"
		if path != "" {
			digest = fileDigest(path)
		}
		binaries = append(binaries, command+"="+path+"#"+digest)
	}
	sort.Strings(binaries)
	value := EvidenceFingerprint{
		TuskVersion:        HostVersion,
		DeviceFingerprint:  device.Fingerprint,
		HostOS:             device.HostOS,
		HostArch:           device.HostArch,
		KernelRelease:      device.KernelRelease,
		TermuxPrefix:       device.TermuxPrefix,
		CandidateID:        candidate.ID,
		BackendBinaries:    binaries,
		ProbeImage:         probeImage,
		PolicyName:         policy.Name,
		PolicyVersion:      policy.Version,
		ConformanceVersion: ConformanceSuiteVersion,
		BenchmarkVersion:   BenchmarkSuiteVersion,
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode evidence fingerprint: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func fileDigest(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
