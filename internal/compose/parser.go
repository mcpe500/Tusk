package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tusk/tusk/internal/client"
	"github.com/tusk/tusk/pkg/protocol"
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
	spec        *types.ComposeSpec
	projectName string
	workDir     string
	cli         *client.Client
}

func NewOrchestrator(spec *types.ComposeSpec, workDir string) *Orchestrator {
	name := filepath.Base(workDir)
	return &Orchestrator{
		spec:        spec,
		projectName: name,
		workDir:     workDir,
	}
}

func (o *Orchestrator) Up() error {
	daemonPath := filepath.Join(os.Getenv("HOME"), ".tusk", "vm", "serial.sock")
	o.cli = client.New(daemonPath)
	if err := o.cli.Connect(); err != nil {
		return fmt.Errorf("connect tuskd: %w", err)
	}
	defer func() {
		o.cli.Close()
		o.cli = nil
	}()

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
	daemonPath := filepath.Join(os.Getenv("HOME"), ".tusk", "vm", "serial.sock")
	o.cli = client.New(daemonPath)
	if err := o.cli.Connect(); err != nil {
		return fmt.Errorf("connect tuskd: %w", err)
	}
	defer func() {
		o.cli.Close()
		o.cli = nil
	}()

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
		cmd := svc.ParseCommand()
		if len(cmd) > 0 {
			fmt.Printf("  command: %v\n", cmd)
		}
	}
	return nil
}

func (o *Orchestrator) Logs(service string) error {
	fmt.Printf("Logs for service: %s\n", service)
	return nil
}

func (o *Orchestrator) createNetwork(name string, net types.Network) error {
	driver := net.Driver
	if driver == "" {
		driver = "bridge"
	}
	fmt.Printf("Creating network: %s (driver: %s)\n", name, driver)
	if err := o.cli.NetworkCreate(name, driver); err != nil {
		return fmt.Errorf("create network %s: %w", name, err)
	}
	return nil
}

func (o *Orchestrator) removeNetwork(name string) {
	fmt.Printf("Removing network: %s\n", name)
}

func (o *Orchestrator) createVolume(name string, _ types.Volume) error {
	hostPath := filepath.Join(os.Getenv("HOME"), ".tusk", "volumes",
		o.projectName+"-"+name)
	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return fmt.Errorf("create volume dir %s: %w", name, err)
	}
	fmt.Printf("Created volume: %s\n", name)
	return nil
}

func (o *Orchestrator) startServices() error {
	order, err := o.resolveServiceOrder()
	if err != nil {
		return err
	}

	cli := o.cli

	for _, name := range order {
		svc := o.spec.Services[name]

		if svc.Image == "" {
			if svc.Build != nil {
				fmt.Fprintf(os.Stderr, "Warning: service %s: build not supported; specify an image instead\n", name)
				continue
			}
			fmt.Fprintf(os.Stderr, "Skipping service %s: no image configured\n", name)
			continue
		}

		fmt.Printf("Starting service: %s (image: %s)\n", name, svc.Image)

		// Auto-pull image if not already present (idempotent).
		if err := cli.ImagePull(svc.Image); err != nil {
			return fmt.Errorf("pull image %s for service %s: %w", svc.Image, name, err)
		}

		for _, dep := range svc.DependsOn {
			fmt.Printf("  depends on: %s\n", dep)
		}

		containerName := svc.ContainerName
		if containerName == "" {
			containerName = fmt.Sprintf("%s-%s", o.projectName, name)
		}

		cmd := svc.ParseCommand()
		if len(cmd) == 0 {
			fmt.Println("  command: default")
		} else {
			fmt.Printf("  command: %s\n", strings.Join(cmd, " "))
		}

		labels := map[string]string{
			"tusk.project": o.projectName,
			"tusk.service": name,
		}
		for k, v := range svc.Labels {
			labels[k] = v
		}
		mounts := parseMounts(svc.Volumes, o.projectName)

		params := &protocol.ContainerCreateParams{
			Image:   svc.Image,
			Name:    containerName,
			Command: cmd,
			Env:     mergeServiceEnv(svc.Environment, svc.EnvFile, o.workDir),
			Mounts:  mounts,
			Ports:   svc.Ports,
			Labels:  labels,
		}

		if len(svc.Networks) > 0 {
			params.Network = svc.Networks[0]
		}

		result, err := cli.ContainerCreate(params)
		if err != nil {
			return fmt.Errorf("failed to create service %s: %w", name, err)
		}

		if err := cli.ContainerStart(result.ID); err != nil {
			return fmt.Errorf("failed to start service %s: %w", name, err)
		}

		id := result.ID
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Printf("  started: %s\n\n", id)
	}

	return nil
}

func (o *Orchestrator) resolveServiceOrder() ([]string, error) {
	state := make(map[string]int)
	order := make([]string, 0, len(o.spec.Services))

	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("dependency cycle at service %s", name)
		}
		if state[name] == 2 {
			return nil
		}

		svc, ok := o.spec.Services[name]
		if !ok {
			return fmt.Errorf("unknown dependency: %s", name)
		}

		state[name] = 1
		for _, dep := range svc.DependsOn {
			if _, ok := o.spec.Services[dep]; !ok {
				return fmt.Errorf("service %s depends on unknown service %s", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		order = append(order, name)
		return nil
	}

	names := make([]string, 0, len(o.spec.Services))
	for name := range o.spec.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

func mergeServiceEnv(base []string, envFiles []string, workDir string) []string {
	vars := map[string]string{}
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		value := ""
		if len(parts) > 1 {
			value = parts[1]
		}
		vars[key] = value
	}

	for _, envFile := range envFiles {
		path := envFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, envFile)
		}
		data, err := ParseEnvFile(path)
		if err != nil {
			continue
		}
		for key, value := range data {
			vars[key] = value
		}
	}

	result := make([]string, 0, len(vars))
	for key, value := range vars {
		result = append(result, key+"="+value)
	}

	return result
}

func (o *Orchestrator) stopService(name string) error {
	containerName := fmt.Sprintf("%s-%s", o.projectName, name)
	if svc, ok := o.spec.Services[name]; ok && svc.ContainerName != "" {
		containerName = svc.ContainerName
	}

	// Find matching containers by label or name prefix
	containers, err := o.cli.ContainerList(true)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	var found []string
	for _, c := range containers {
		if c.Name == containerName ||
			strings.TrimPrefix(c.Name, "/") == containerName {
			found = append(found, c.ID)
		}
	}

	if len(found) == 0 {
		fmt.Printf("No containers found for service: %s\n", name)
		return nil
	}

	for _, id := range found {
		fmt.Printf("Stopping service: %s (%s)\n", name, id[:min(12, len(id))])
		if err := o.cli.ContainerStop(id); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: stop %s: %v\n", id, err)
		}
		if err := o.cli.ContainerRemove(id, true); err != nil {
			return fmt.Errorf("remove container %s: %w", id, err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseMounts converts compose volume strings to MountParams.
// Formats: "host:container", "host:container:ro", "container" (anon), "volname:container" (named).
func parseMounts(volumes []string, projectName string) []protocol.MountParams {
	var mounts []protocol.MountParams
	for _, v := range volumes {
		parts := strings.SplitN(v, ":", 3)
		var src, dst string
		readOnly := false
		switch len(parts) {
		case 1:
			// anonymous: just a container path — skip, no host bind
			continue
		case 2:
			src = parts[0]
			dst = parts[1]
		case 3:
			src = parts[0]
			dst = parts[1]
			readOnly = parts[2] == "ro"
		}
		// named volume: no slash in source → map to ~/.tusk/volumes/<project>-<name>
		if !strings.Contains(src, "/") {
			src = filepath.Join(os.Getenv("HOME"), ".tusk", "volumes",
				projectName+"-"+src)
		}
		mounts = append(mounts, protocol.MountParams{
			Type:        "bind",
			Source:      src,
			Destination: dst,
			ReadOnly:    readOnly,
		})
	}
	return mounts
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
