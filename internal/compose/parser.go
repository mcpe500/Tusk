package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tusk/tusk/pkg/types"
	"gopkg.in/yaml.v3"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(path string) (*types.ComposeSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var spec types.ComposeSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	return &spec, nil
}

type Orchestrator struct {
	spec     *types.ComposeSpec
	projectName string
	workDir  string
}

func NewOrchestrator(spec *types.ComposeSpec, workDir string) *Orchestrator {
	name := filepath.Base(workDir)
	return &Orchestrator{
		spec:         spec,
		projectName:  name,
		workDir:      workDir,
	}
}

func (o *Orchestrator) Up() error {
	// Create networks first
	for name, net := range o.spec.Networks {
		if err := o.createNetwork(name, net); err != nil {
			return err
		}
	}

	// Create volumes
	for name, vol := range o.spec.Volumes {
		if err := o.createVolume(name, vol); err != nil {
			return err
		}
	}

	// Start services in dependency order
	return o.startServices()
}

func (o *Orchestrator) Down() error {
	// Stop and remove services
	for name := range o.spec.Services {
		if err := o.stopService(name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	// Remove networks
	for name := range o.spec.Networks {
		o.removeNetwork(name)
	}

	return nil
}

func (o *Orchestrator) PS() error {
	for name, svc := range o.spec.Services {
		fmt.Printf("%s: image=%s\n", name, svc.Image)
		if len(svc.Command) > 0 {
			fmt.Printf("  command: %v\n", svc.Command)
		}
	}
	return nil
}

func (o *Orchestrator) Logs(service string) error {
	fmt.Printf("Logs for service: %s\n", service)
	return nil
}

func (o *Orchestrator) createNetwork(name string, net types.Network) error {
	fmt.Printf("Creating network: %s (driver: %s)\n", name, net.Driver)
	return nil
}

func (o *Orchestrator) removeNetwork(name string) {
	fmt.Printf("Removing network: %s\n", name)
}

func (o *Orchestrator) createVolume(name string, vol types.Volume) error {
	fmt.Printf("Creating volume: %s\n", name)
	return nil
}

func (o *Orchestrator) startServices() error {
	// Simple: start all services
	for name, svc := range o.spec.Services {
		fmt.Printf("Starting service: %s (image: %s)\n", name, svc.Image)

		// Check dependencies
		for _, dep := range svc.DependsOn {
			var depName string
			switch v := dep.(type) {
			case string:
				depName = v
			default:
				if m, ok := dep.(map[string]interface{}); ok {
					depName = ""
					for k := range m {
						depName = k
						break
					}
				}
			}
			if depName != "" {
				fmt.Printf("  depends on: %s\n", depName)
			}
		}

		// TODO: Actually create and start containers
	}
	return nil
}

func (o *Orchestrator) stopService(name string) error {
	fmt.Printf("Stopping service: %s\n", name)
	return nil
}

func ParseEnvFile(path string) (map[string]string, error) {
	result := make(map[string]string)

	data, err := os.ReadFile(path)
	if err != nil {
		return result, nil
	}

	lines := string(data)
	for _, line := range splitLines(lines) {
		line = trimComment(line)
		if line == "" {
			continue
		}
		parts := splitEnv(line)
		if len(parts) >= 1 {
			key := parts[0]
			value := ""
			if len(parts) >= 2 {
				value = parts[1]
			}
			result[key] = value
		}
	}

	return result, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimComment(line string) string {
	inQuote := false
	for i, c := range line {
		if c == '"' || c == '\'' {
			inQuote = !inQuote
		}
		if c == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func splitEnv(line string) []string {
	var parts []string
	var current []byte
	inQuote := false

	for _, c := range []byte(line) {
		if c == '"' || c == '\'' {
			inQuote = !inQuote
			continue
		}
		if c == '=' && !inQuote {
			parts = append(parts, string(current))
			current = nil
			continue
		}
		current = append(current, c)
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}

	return parts
}