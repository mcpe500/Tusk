package types

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
	Command    []string          `json:"command,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	DependsOn  []ServiceDep     `json:"depends_on,omitempty"`
	Ports      []PortSpec       `json:"ports,omitempty"`
	Volumes    []VolumeSpec     `json:"volumes,omitempty"`
	Environment []string         `json:"environment,omitempty"`
	EnvFile    []string          `json:"env_file,omitempty"`
	Networks   []string          `json:"networks,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	CommandString string         `json:"command_string,omitempty"`
	ContainerName string        `json:"container_name,omitempty"`
	Restart    string            `json:"restart,omitempty"`
	HealthCheck *HealthCheck     `json:"healthcheck,omitempty"`
	Deploy     *DeploySpec       `json:"deploy,omitempty"`
	User       string            `json:"user,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Links      []string          `json:"links,omitempty"`
	ExternalLinks []string       `json:"external_links,omitempty"`
	Pid        string            `json:"pid,omitempty"`
	NetworkMode string           `json:"network_mode,omitempty"`
	DNS        []string          `json:"dns,omitempty"`
	DNSOpts    []string          `json:"dns_opt,omitempty"`
	DNSSearch  []string          `json:"dns_search,omitempty"`
	ExtraHosts []string          `json:"extra_hosts,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	IPC        string            `json:"ipc,omitempty"`
	MacAddress string            `json:"mac_address,omitempty"`
	Privileged bool              `json:"privileged,omitempty"`
	ReadOnly   bool              `json:"read_only,omitempty"`
	ShmSize    interface{}       `json:"shm_size,omitempty"`
	StdinOpen  bool              `json:"stdin_open,omitempty"`
	Tty        bool              `json:"tty,omitempty"`
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

type PortSpec struct {
	Target    int    `json:"target"`
	Published int    `json:"published,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
}

type VolumeSpec struct {
	Type        string `json:"type"`
	Source      string `json:"source,omitempty"`
	Target      string `json:"target,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	BindOptions *BindOptions `json:"bind_options,omitempty"`
	VolumeOptions *VolumeOptions `json:"volume_options,omitempty"`
}

type BindOptions struct {
	Propagation string `json:"propagation,omitempty"`
}

type VolumeOptions struct {
	NoCopy bool `json:"nocopy,omitempty"`
}

type HealthCheck struct {
	Test        []string `json:"test,omitempty"`
	Interval    string   `json:"interval,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	Retries     int      `json:"retries,omitempty"`
	StartPeriod string   `json:"start_period,omitempty"`
}

type DeploySpec struct {
	Replicas  int               `json:"replicas,omitempty"`
	Resources *DeployResources  `json:"resources,omitempty"`
	RestartPolicy *RestartPolicy `json:"restart_policy,omitempty"`
}

type DeployResources struct {
	Limits       *ResourceSpec `json:"limits,omitempty"`
	Reservations *ResourceSpec `json:"reservations,omitempty"`
}

type ResourceSpec struct {
	MemoryBytes   int64 `json:"memory_bytes,omitempty"`
	NanoCPUs      int64 `json:"nano_cpus,omitempty"`
}

type RestartPolicy struct {
	Condition string `json:"condition,omitempty"`
	Delay     string `json:"delay,omitempty"`
	MaxAttempts int  `json:"max_attempts,omitempty"`
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