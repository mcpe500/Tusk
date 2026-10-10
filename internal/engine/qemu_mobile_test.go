package engine

import (
	"strings"
	"testing"
)

func TestQEMUArgsUseLowPowerMobileDefaults(t *testing.T) {
	adapter := &QEMUAdapter{config: QEMUConfig{
		QEMUPath:    "qemu-system-aarch64",
		Machine:     "virt",
		CandidateID: "qemu-system-aarch64",
		CPUs:        2,
		MemoryMiB:   768,
		SSHPort:     2222,
	}}
	args := strings.Join(adapter.qemuArgs(), " ")
	for _, required := range []string{"-no-reboot", "-cpu cortex-a76", "-device virtio-rng-pci", "-device virtio-balloon"} {
		if !strings.Contains(args, required) {
			t.Fatalf("QEMU args omit %s: %q", required, args)
		}
	}
}
