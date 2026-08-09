package engine

import (
	"testing"

	"github.com/mcpe500/Tusk/internal/platform"
)

func TestCatalogHasStableUniqueIDs(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"proot-native":           true,
		"proot-qemu-user-x86_64": true,
		"qemu-system-aarch64":    true,
		"qemu-system-x86_64":     true,
		"qemu-system-i386":       true,
		"qemu-system-arm":        true,
	}
	for _, candidate := range Catalog() {
		if !want[candidate.ID] {
			t.Fatalf("unexpected or duplicate candidate %q", candidate.ID)
		}
		delete(want, candidate.ID)
	}
	if len(want) != 0 {
		t.Fatalf("catalog is missing candidates: %v", want)
	}
}

func TestDiscoverCandidatesDoesNotQualifyAvailability(t *testing.T) {
	t.Parallel()
	device := platform.Device{
		HostArch: "arm64",
		Commands: map[string]string{
			"proot":               "/bin/proot",
			"qemu-system-aarch64": "/bin/qemu-system-aarch64",
		},
	}
	got := DiscoverCandidates(device)
	byID := make(map[string]Discovery, len(got))
	for _, discovery := range got {
		byID[discovery.Candidate.ID] = discovery
	}
	if !byID["proot-native"].Available || !byID["qemu-system-aarch64"].Available {
		t.Fatalf("installed candidates not discovered: %#v", byID)
	}
	if byID["qemu-system-x86_64"].Available {
		t.Fatal("missing qemu-system-x86_64 reported available")
	}
}
