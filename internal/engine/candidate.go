package engine

import (
	"sort"

	"github.com/mcpe500/Tusk/internal/platform"
)

type Kind string

const (
	KindProot      Kind = "proot"
	KindProotQEMU  Kind = "proot-qemu-user"
	KindQEMUSystem Kind = "qemu-system"
)

type Candidate struct {
	ID               string   `json:"id"`
	Kind             Kind     `json:"kind"`
	Architecture     string   `json:"architecture"`
	RequiredCommands []string `json:"requiredCommands"`
	InstallHint      string   `json:"installHint"`
	Experimental     bool     `json:"experimental"`
	Description      string   `json:"description"`
}

type Discovery struct {
	Candidate       Candidate `json:"candidate"`
	Applicable      bool      `json:"applicable"`
	Available       bool      `json:"available"`
	MissingCommands []string  `json:"missingCommands,omitempty"`
	Reasons         []string  `json:"reasons,omitempty"`
}

func Catalog() []Candidate {
	return []Candidate{
		{
			ID:               "proot-native",
			Kind:             KindProot,
			Architecture:     "native",
			RequiredCommands: []string{"proot"},
			InstallHint:      "pkg install proot",
			Experimental:     true,
			Description:      "Podman in a host-architecture proot environment",
		},
		{
			ID:               "proot-qemu-user-x86_64",
			Kind:             KindProotQEMU,
			Architecture:     "amd64",
			RequiredCommands: []string{"proot", "qemu-x86_64"},
			InstallHint:      "pkg install proot qemu-user-x86-64",
			Experimental:     true,
			Description:      "Podman in proot with qemu-user x86_64 translation",
		},
		{
			ID:               "qemu-system-aarch64",
			Kind:             KindQEMUSystem,
			Architecture:     "arm64",
			RequiredCommands: []string{"qemu-system-aarch64"},
			InstallHint:      "install the Termux package providing qemu-system-aarch64",
			Description:      "Full ARM64 Linux system under QEMU",
		},
		{
			ID:               "qemu-system-x86_64",
			Kind:             KindQEMUSystem,
			Architecture:     "amd64",
			RequiredCommands: []string{"qemu-system-x86_64"},
			InstallHint:      "install the Termux package providing qemu-system-x86_64",
			Description:      "Full AMD64 Linux system under QEMU",
		},
		{
			ID:               "qemu-system-i386",
			Kind:             KindQEMUSystem,
			Architecture:     "386",
			RequiredCommands: []string{"qemu-system-i386"},
			InstallHint:      "install the Termux package providing qemu-system-i386",
			Experimental:     true,
			Description:      "Full 32-bit x86 Linux system under QEMU",
		},
		{
			ID:               "qemu-system-arm",
			Kind:             KindQEMUSystem,
			Architecture:     "arm",
			RequiredCommands: []string{"qemu-system-arm"},
			InstallHint:      "install the Termux package providing qemu-system-arm",
			Experimental:     true,
			Description:      "Full ARM32 Linux system under QEMU",
		},
	}
}

func DiscoverCandidates(device platform.Device) []Discovery {
	catalog := Catalog()
	result := make([]Discovery, 0, len(catalog))
	for _, candidate := range catalog {
		discovery := Discovery{Candidate: candidate, Applicable: applicable(candidate, device)}
		for _, command := range candidate.RequiredCommands {
			if _, ok := device.Commands[command]; !ok {
				discovery.MissingCommands = append(discovery.MissingCommands, command)
			}
		}
		discovery.Available = discovery.Applicable && len(discovery.MissingCommands) == 0
		if !discovery.Applicable {
			discovery.Reasons = append(discovery.Reasons, "candidate does not match this host architecture")
		}
		if len(discovery.MissingCommands) > 0 {
			discovery.Reasons = append(discovery.Reasons, "required commands are not installed")
		}
		result = append(result, discovery)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Candidate.ID < result[j].Candidate.ID
	})
	return result
}

func applicable(candidate Candidate, device platform.Device) bool {
	switch candidate.ID {
	case "proot-qemu-user-x86_64":
		return device.HostArch != "amd64"
	default:
		return true
	}
}
