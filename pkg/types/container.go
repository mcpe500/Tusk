package types

import (
	"time"
)

type ContainerState struct {
	OCIVersion string           `json:"ociVersion"`
	ID         string           `json:"id"`
	Status     ContainerStatus  `json:"status"`
	Pid        int              `json:"pid"`
	Bundle     string           `json:"bundle"`
	Created    time.Time        `json:"created"`
	Started    *time.Time      `json:"started,omitempty"`
	Finished   *time.Time      `json:"finished,omitempty"`
	ExitCode   int              `json:"exitCode,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ContainerStatus string

const (
	StatusCreated ContainerStatus = "created"
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusPaused  ContainerStatus = "paused"
)

type RuntimeSpec struct {
	OCIVersion string       `json:"ociVersion"`
	Hostname   string      `json:"hostname,omitempty"`
	Mounts     []Mount      `json:"mounts,omitempty"`
	Process    ProcessSpec `json:"process"`
	Linux      LinuxSpec   `json:"linux,omitempty"`
	Hooks      *Hooks      `json:"hooks,omitempty"`
}

type ProcessSpec struct {
	Terminal bool     `json:"terminal,omitempty"`
	User     User     `json:"user"`
	Args     []string `json:"args"`
	Cwd      string   `json:"cwd"`
	Env      []string `json:"env,omitempty"`
	Capabilities *LinuxCapabilities `json:"capabilities,omitempty"`
}

type User struct {
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
	AdditionalGids []uint32 `json:"additionalGids,omitempty"`
}

type Mount struct {
	Source      string   `json:"source,omitempty"`
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
}

type LinuxSpec struct {
	Namespaces  []Namespace      `json:"namespaces,omitempty"`
	Resources   *LinuxResources  `json:"resources,omitempty"`
	CgroupsPath string           `json:"cgroupsPath,omitempty"`
	Seccomp     *Seccomp         `json:"seccomp,omitempty"`
	MaskedPaths []string         `json:"maskedPaths,omitempty"`
	ReadonlyPaths []string       `json:"readonlyPaths,omitempty"`
}

type Namespace struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type LinuxResources struct {
	Memory       *MemoryResources    `json:"memory,omitempty"`
	CPU          *CPUResources        `json:"cpu,omitempty"`
	BlockIO      *BlockIOResources    `json:"blockIO,omitempty"`
	Pids         *PidsResources       `json:"pids,omitempty"`
}

type MemoryResources struct {
	Limit   *int64 `json:"limit,omitempty"`
	Reservation *int64 `json:"reservation,omitempty"`
	Swap    *int64 `json:"swap,omitempty"`
	Kernel  *int64 `json:"kernel,omitempty"`
	KernelTCP *int64 `json:"kernelTCP,omitempty"`
	Swappiness *uint64 `json:"swappiness,omitempty"`
}

type CPUResources struct {
	Realtime   *CPURealtime   `json:"realtime,omitempty"`
	CPUShares  *uint64        `json:"shares,omitempty"`
	CpusetCpus string          `json:"cpuset,omitempty"`
	CpusetMems string          `json:"cpusetMems,omitempty"`
}

type CPURealtime struct {
	Period *uint64 `json:"period,omitempty"`
	Runtime *int64 `json:"runtime,omitempty"`
}

type BlockIOResources struct {
	Weight *uint16 `json:"weight,omitempty"`
}

type PidsResources struct {
	Limit *int64 `json:"limit,omitempty"`
}

type LinuxCapabilities struct {
	Bounding  []string `json:"bounding,omitempty"`
	Effective []string `json:"effective,omitempty"`
	Inheritable []string `json:"inheritable,omitempty"`
	Permitted []string `json:"permitted,omitempty"`
	Ambient   []string `json:"ambient,omitempty"`
}

type Seccomp struct {
	DefaultAction string      `json:"defaultAction"`
	Architectures []string   `json:"architectures,omitempty"`
	Syscalls      []SyscallRule `json:"syscalls,omitempty"`
}

type SyscallRule struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
}

type Hooks struct {
	Prestart        []Hook        `json:"prestart,omitempty"`
	Poststart       []Hook        `json:"poststart,omitempty"`
	Poststop        []Hook        `json:"poststop,omitempty"`
}

type Hook struct {
	Path    string   `json:"path"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type ContainerInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Created   time.Time        `json:"created"`
	State     ContainerStatus  `json:"state"`
	Pid       int              `json:"pid,omitempty"`
	IPAddress string           `json:"ipAddress,omitempty"`
	Ports     []PortMapping    `json:"ports,omitempty"`
	Mounts    []Mount          `json:"mounts,omitempty"`
	Env       []string         `json:"env,omitempty"`
	Cmd       []string         `json:"cmd,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type PortMapping struct {
	Protocol  string `json:"protocol"`
	HostPort   int    `json:"hostPort"`
	TargetPort int    `json:"targetPort"`
}

type NetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}