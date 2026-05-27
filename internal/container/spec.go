package container

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tusk/tusk/internal/image"
	"github.com/tusk/tusk/pkg/types"
)

type SpecGenerator struct {
	store *image.Store
}

func NewSpecGenerator(imageStore *image.Store) *SpecGenerator {
	return &SpecGenerator{store: imageStore}
}

// FromImageConfig creates a RuntimeSpec from OCI image config
func (g *SpecGenerator) FromImageConfig(imageConfig *types.ImageConfig, containerID, containerName string) *types.RuntimeSpec {
	hostname := containerName
	if hostname == "" {
		hostname = fmt.Sprintf("tusk-%s", containerID[:12])
	}

	// Build environment from image config
	env := imageConfig.Config.Env
	if env == nil {
		env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	}

	// Build command
	cmd := imageConfig.Config.Cmd
	if cmd == nil {
		cmd = []string{"/bin/sh"}
	}

	spec := &types.RuntimeSpec{
		OCIVersion: "1.0.2",
		Hostname:   hostname,
		Process: types.ProcessSpec{
			Terminal: false,
			User: types.User{
				UID: 0,
				GID: 0,
			},
			Args: cmd,
			Cwd:  imageConfig.Config.WorkingDir,
			Env:  env,
		},
		Linux: types.LinuxSpec{
			Namespaces: []types.Namespace{
				{Type: "pid"},
				{Type: "network"},
				{Type: "mount"},
				{Type: "ipc"},
				{Type: "uts"},
			},
			Resources: &types.LinuxResources{
				Memory: &types.MemoryResources{},
			},
		},
	}

	// Set working dir default
	if spec.Process.Cwd == "" {
		spec.Process.Cwd = "/"
	}

	return spec
}

// WithResourceLimits sets CPU and memory limits
func (g *SpecGenerator) WithResourceLimits(spec *types.RuntimeSpec, memoryMB int64, cpuShares uint64) *types.RuntimeSpec {
	if spec.Linux.Resources == nil {
		spec.Linux.Resources = &types.LinuxResources{}
	}
	if spec.Linux.Resources.Memory == nil {
		spec.Linux.Resources.Memory = &types.MemoryResources{}
	}

	spec.Linux.Resources.Memory.Limit = &memoryMB

	if cpuShares > 0 {
		spec.Linux.Resources.CPU = &types.CPUResources{
			CPUShares: &cpuShares,
		}
	}

	return spec
}

// WithMounts adds volume mounts to the spec
func (g *SpecGenerator) WithMounts(spec *types.RuntimeSpec, mounts []types.Mount) *types.RuntimeSpec {
	spec.Mounts = append(spec.Mounts, mounts...)
	return spec
}

// SaveSpec saves the runtime spec to a bundle directory
func (g *SpecGenerator) SaveSpec(spec *types.RuntimeSpec, bundlePath string) error {
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("create bundle dir: %w", err)
	}

	configPath := filepath.Join(bundlePath, "config.json")
	data, err := jsonMarshal(spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func jsonMarshal(v interface{}) ([]byte, error) {
	// Simple JSON marshaler to avoid import cycle
	// In real implementation, use encoding/json
	return []byte{}, fmt.Errorf("use encoding/json in real implementation")
}