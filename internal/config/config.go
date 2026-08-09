package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/mcpe500/Tusk/internal/state"
)

const SchemaVersion = 1

type Config struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	SelectedEngine string                  `json:"selectedEngine"`
	AutoStart      bool                    `json:"autoStart"`
	SmartEngine    SmartEngineConfig       `json:"smartEngine"`
	Engines        map[string]EngineConfig `json:"engines,omitempty"`
	Network        NetworkConfig           `json:"network"`
}

type SmartEngineConfig struct {
	Mode             string `json:"mode"`
	Policy           string `json:"policy"`
	PolicyVersion    int    `json:"policyVersion"`
	AllowUnqualified bool   `json:"allowUnqualified"`
}

type EngineConfig struct {
	CPUs      int `json:"cpus,omitempty"`
	MemoryMiB int `json:"memoryMiB,omitempty"`
}

type NetworkConfig struct {
	DefaultBindAddress string `json:"defaultBindAddress"`
}

func Default() Config {
	return Config{
		SchemaVersion:  SchemaVersion,
		SelectedEngine: "auto",
		AutoStart:      true,
		SmartEngine: SmartEngineConfig{
			Mode:          "quick",
			Policy:        "balanced",
			PolicyVersion: 1,
		},
		Engines: map[string]EngineConfig{
			"proot-native":        {},
			"qemu-system-aarch64": {CPUs: 2, MemoryMiB: 1024},
			"qemu-system-x86_64":  {CPUs: 2, MemoryMiB: 1536},
		},
		Network: NetworkConfig{DefaultBindAddress: "127.0.0.1"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if err := state.ReadJSON(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return state.WriteJSON(path, cfg, 0o600)
}

func (c Config) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d (expected %d)", c.SchemaVersion, SchemaVersion)
	}
	if c.SelectedEngine == "" {
		return errors.New("selectedEngine is required")
	}
	if c.SmartEngine.Mode != "quick" && c.SmartEngine.Mode != "thorough" {
		return fmt.Errorf("smartEngine.mode must be quick or thorough, got %q", c.SmartEngine.Mode)
	}
	if !validPolicy(c.SmartEngine.Policy) {
		return fmt.Errorf("unsupported smartEngine.policy %q", c.SmartEngine.Policy)
	}
	if c.SmartEngine.PolicyVersion < 1 {
		return errors.New("smartEngine.policyVersion must be positive")
	}
	for id, engine := range c.Engines {
		if id == "" {
			return errors.New("engine ID must not be empty")
		}
		if engine.CPUs < 0 || engine.MemoryMiB < 0 {
			return fmt.Errorf("engine %q resources must not be negative", id)
		}
	}
	if net.ParseIP(c.Network.DefaultBindAddress) == nil {
		return fmt.Errorf("invalid network.defaultBindAddress %q", c.Network.DefaultBindAddress)
	}
	return nil
}

func validPolicy(policy string) bool {
	switch policy {
	case "balanced", "speed", "low-memory", "compatibility":
		return true
	default:
		return false
	}
}
