package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var candidateCommands = []string{
	"proot",
	"qemu-aarch64",
	"qemu-arm",
	"qemu-i386",
	"qemu-x86_64",
	"qemu-system-aarch64",
	"qemu-system-arm",
	"qemu-system-i386",
	"qemu-system-x86_64",
}

type Device struct {
	HostOS                string            `json:"hostOS"`
	HostArch              string            `json:"hostArch"`
	KernelRelease         string            `json:"kernelRelease,omitempty"`
	Termux                bool              `json:"termux"`
	TermuxPrefix          string            `json:"termuxPrefix,omitempty"`
	CPUCount              int               `json:"cpuCount"`
	MemoryTotalBytes      uint64            `json:"memoryTotalBytes,omitempty"`
	StorageAvailableBytes uint64            `json:"storageAvailableBytes,omitempty"`
	Commands              map[string]string `json:"commands"`
	Fingerprint           string            `json:"fingerprint"`
}

type Probe struct {
	HostOS   string
	HostArch string
	NumCPU   func() int
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	LookPath func(string) (string, error)
	StatFS   func(string) (uint64, error)
}

func DefaultProbe() Probe {
	return Probe{
		HostOS:   runtime.GOOS,
		HostArch: runtime.GOARCH,
		NumCPU:   runtime.NumCPU,
		Getenv:   os.Getenv,
		ReadFile: os.ReadFile,
		LookPath: exec.LookPath,
		StatFS:   availableBytes,
	}
}

func Discover(storagePath string) (Device, error) {
	return DiscoverWith(DefaultProbe(), storagePath)
}

func DiscoverWith(probe Probe, storagePath string) (Device, error) {
	if probe.NumCPU == nil || probe.Getenv == nil || probe.ReadFile == nil || probe.LookPath == nil || probe.StatFS == nil {
		return Device{}, fmt.Errorf("platform probe is incomplete")
	}

	prefix := probe.Getenv("PREFIX")
	device := Device{
		HostOS:       probe.HostOS,
		HostArch:     normalizeArch(probe.HostArch),
		TermuxPrefix: prefix,
		Termux:       strings.Contains(prefix, "com.termux") || probe.HostOS == "android",
		CPUCount:     probe.NumCPU(),
		Commands:     make(map[string]string),
	}
	if device.CPUCount < 1 {
		device.CPUCount = 1
	}

	if data, err := probe.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		device.KernelRelease = strings.TrimSpace(string(data))
	}
	if data, err := probe.ReadFile("/proc/meminfo"); err == nil {
		device.MemoryTotalBytes = parseMemTotal(data)
	}
	if available, err := probe.StatFS(storagePath); err == nil {
		device.StorageAvailableBytes = available
	}
	for _, command := range candidateCommands {
		if path, err := probe.LookPath(command); err == nil {
			device.Commands[command] = path
		}
	}

	fingerprint, err := fingerprint(device)
	if err != nil {
		return Device{}, err
	}
	device.Fingerprint = fingerprint
	return device, nil
}

func normalizeArch(arch string) string {
	switch arch {
	case "arm64", "aarch64":
		return "arm64"
	case "amd64", "x86_64":
		return "amd64"
	case "386", "i386", "i686":
		return "386"
	default:
		return arch
	}
}

func parseMemTotal(data []byte) uint64 {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return value * 1024
	}
	return 0
}

func availableBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

func fingerprint(device Device) (string, error) {
	commands := make([]string, 0, len(device.Commands))
	for command := range device.Commands {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	payload := struct {
		HostOS        string   `json:"hostOS"`
		HostArch      string   `json:"hostArch"`
		KernelRelease string   `json:"kernelRelease"`
		Termux        bool     `json:"termux"`
		TermuxPrefix  string   `json:"termuxPrefix"`
		CPUCount      int      `json:"cpuCount"`
		MemoryBytes   uint64   `json:"memoryBytes"`
		Commands      []string `json:"commands"`
	}{
		HostOS:        device.HostOS,
		HostArch:      device.HostArch,
		KernelRelease: device.KernelRelease,
		Termux:        device.Termux,
		TermuxPrefix:  device.TermuxPrefix,
		CPUCount:      device.CPUCount,
		MemoryBytes:   device.MemoryTotalBytes,
		Commands:      commands,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode device fingerprint: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
