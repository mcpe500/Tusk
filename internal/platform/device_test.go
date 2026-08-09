package platform

import (
	"errors"
	"testing"
)

func TestDiscoverWith(t *testing.T) {
	t.Parallel()
	commands := map[string]string{
		"proot":               "/data/data/com.termux/files/usr/bin/proot",
		"qemu-system-aarch64": "/data/data/com.termux/files/usr/bin/qemu-system-aarch64",
	}
	probe := Probe{
		HostOS:   "android",
		HostArch: "aarch64",
		NumCPU:   func() int { return 8 },
		Getenv: func(key string) string {
			if key == "PREFIX" {
				return "/data/data/com.termux/files/usr"
			}
			return ""
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case "/proc/sys/kernel/osrelease":
				return []byte("6.1-android\n"), nil
			case "/proc/meminfo":
				return []byte("MemTotal:       2048 kB\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		LookPath: func(command string) (string, error) {
			if path, ok := commands[command]; ok {
				return path, nil
			}
			return "", errors.New("not found")
		},
		StatFS: func(string) (uint64, error) { return 4096, nil },
	}

	device, err := DiscoverWith(probe, "/tmp")
	if err != nil {
		t.Fatalf("DiscoverWith() error = %v", err)
	}
	if device.HostArch != "arm64" || !device.Termux {
		t.Fatalf("unexpected device identity: %#v", device)
	}
	if device.MemoryTotalBytes != 2048*1024 {
		t.Fatalf("MemoryTotalBytes = %d", device.MemoryTotalBytes)
	}
	if device.Commands["proot"] == "" || device.Fingerprint == "" {
		t.Fatalf("discovery missing command or fingerprint: %#v", device)
	}

	again, err := DiscoverWith(probe, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if again.Fingerprint != device.Fingerprint {
		t.Fatalf("fingerprint is not deterministic: %q != %q", again.Fingerprint, device.Fingerprint)
	}
}

func TestParseMemTotal(t *testing.T) {
	t.Parallel()
	if got := parseMemTotal([]byte("Other: 1 kB\n")); got != 0 {
		t.Fatalf("parseMemTotal() = %d, want 0", got)
	}
}
