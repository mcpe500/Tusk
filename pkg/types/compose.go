package types

import "gopkg.in/yaml.v3"

type ComposeSpec struct {
	Version  string                 `json:"version"`
	Services map[string]Service     `json:"services"`
	Networks map[string]Network    `json:"networks,omitempty"`
	Volumes  map[string]Volume     `json:"volumes,omitempty"`
	Secrets  map[string]Secret     `json:"secrets,omitempty"`
	Configs  map[string]ConfigFile `json:"configs,omitempty"`
}

type Service struct {
	Image      string            `json:"image"`
	Build      *BuildSpec        `json:"build,omitempty"`
	Command    any               `json:"command,omitempty"`
	Entrypoint any               `json:"entrypoint,omitempty"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	Ports      []string          `json:"ports,omitempty"`
	Volumes    []string          `json:"volumes,omitempty"`
	Environment []string         `json:"environment,omitempty"`
	EnvFile    []string          `json:"env_file,omitempty"`
	Networks   []string          `json:"networks,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	ContainerName string          `json:"container_name,omitempty"`
	Restart    string            `json:"restart,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Links      []string          `json:"links,omitempty"`
	Pid        string            `json:"pid,omitempty"`
	NetworkMode string           `json:"network_mode,omitempty"`
	DNS        []string          `json:"dns,omitempty"`
	ExtraHosts []string          `json:"extra_hosts,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	StdinOpen  bool              `json:"stdin_open,omitempty"`
	Tty        bool              `json:"tty,omitempty"`
}

// ParseCommand parses command which can be either a string or []string
func (s *Service) ParseCommand() []string {
	if s.Command == nil {
		return nil
	}
	switch v := s.Command.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	}
	return nil
}

// MarshalYAML fix for command parsing
func (s *Service) UnmarshalYAML(value *yaml.Node) error {
	type rawService Service
	var raw rawService
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*s = Service(raw)
	return nil
}

type BuildSpec struct {
	Context    string            `json:"context,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	CacheFrom  []string          `json:"cache_from,omitempty"`
}

type ServiceDep struct {
	Condition string `json:"condition,omitempty"`
}

type Network struct {
	Driver    string            `json:"driver,omitempty"`
	DriverOpts map[string]string `json:"driver_opts,omitempty"`
	IPAM      *IPAM             `json:"ipam,omitempty"`
	External  bool              `json:"external,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type IPAM struct {
	Driver  string       `json:"driver,omitempty"`
	Config  []IPAMConfig `json:"config,omitempty"`
}

type IPAMConfig struct {
	Subnet     string `json:"subnet,omitempty"`
	IPRange    string `json:"ip_range,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
}

type Volume struct {
	Driver     string            `json:"driver,omitempty"`
	DriverOpts map[string]string `json:"driver_opts,omitempty"`
	External   bool              `json:"external,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type Secret struct {
	File     string `json:"file,omitempty"`
	External bool   `json:"external,omitempty"`
}

type ConfigFile struct {
	File     string `json:"file,omitempty"`
	External bool   `json:"external,omitempty"`
}