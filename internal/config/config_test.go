package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default().Validate() error = %v", err)
	}
	if cfg.SelectedEngine != "auto" || cfg.SmartEngine.Policy != "balanced" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	want := Default()
	want.SelectedEngine = "qemu-system-aarch64"

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SelectedEngine != want.SelectedEngine {
		t.Fatalf("SelectedEngine = %q, want %q", got.SelectedEngine, want.SelectedEngine)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	t.Parallel()
	got, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
}

func TestValidateRejectsInvalidPolicy(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.SmartEngine.Policy = "fastest-at-any-cost"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted an unsupported policy")
	}
}
