package config

import "testing"

func TestDefaultsStayWithinMobileSafetyBudget(t *testing.T) {
	cfg := Default()
	for id, engine := range cfg.Engines {
		if engine.CPUs > 2 {
			t.Errorf("%s CPUs = %d, want <= 2", id, engine.CPUs)
		}
		if engine.MemoryMiB > 1024 {
			t.Errorf("%s MemoryMiB = %d, want <= 1024", id, engine.MemoryMiB)
		}
	}
}

func TestValidateRejectsUnsafeQEMUMobileResourceOverrides(t *testing.T) {
	cfg := Default()
	cfg.Engines["qemu-system-aarch64"] = EngineConfig{CPUs: 8, MemoryMiB: 4096}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted unsafe QEMU mobile resources")
	}
}
