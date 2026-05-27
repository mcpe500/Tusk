package vm

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manager struct {
	baseDir  string
	vmDir    string
	sockDir  string
	qmpSock  string
	serialSock string
	cmd      *exec.Cmd
}

type Config struct {
	Memory     int
	CPUs      int
	DiskPath  string
	KernelPath string
	InitrdPath string
}

type VMStatus string

const (
	StatusRunning VMStatus = "running"
	StatusStopped VMStatus = "stopped"
	StatusError   VMStatus = "error"
)

func New(tuskDir string) *Manager {
	vmDir := filepath.Join(tuskDir, "vm")
	return &Manager{
		baseDir:    tuskDir,
		vmDir:      vmDir,
		sockDir:    filepath.Join(vmDir, "sockets"),
		qmpSock:    filepath.Join(vmDir, "qmp.sock"),
		serialSock: filepath.Join(vmDir, "serial.sock"),
	}
}

func (m *Manager) Init() error {
	dirs := []string{m.vmDir, m.sockDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, cfg *Config) error {
	if m.isRunning() {
		return nil
	}

	if err := m.Init(); err != nil {
		return err
	}

	// Default config
	if cfg.Memory == 0 {
		cfg.Memory = 512
	}
	if cfg.CPUs == 0 {
		cfg.CPUs = 2
	}

	args := []string{
		"qemu-system-x86_64",
		"-M", "pc-i440fx-9.2",
		"-m", fmt.Sprintf("%d", cfg.Memory),
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nographic",
	}

	// QMP socket for VM control
	args = append(args, "-qmp", fmt.Sprintf("unix:%s,server,nowait", m.qmpSock))

	// virtio-serial for CLI communication
	args = append(args, "-serial", fmt.Sprintf("unix:%s", m.serialSock))

	// Network: user-mode NAT
	args = append(args, "-netdev", "user,id=net0")
	args = append(args, "-device", "virtio-net-pci,netdev=net0")

	// virtfs for shared storage (tusk directory)
	args = append(args, "-virtfs",
		fmt.Sprintf("local,path=%s,mount_tag=tusk-data,security_model=mapped,id=tusk", m.baseDir))

	// Optional disk
	if cfg.DiskPath != "" {
		args = append(args, "-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", cfg.DiskPath))
	}

	// Optional kernel boot (for custom VM images)
	if cfg.KernelPath != "" {
		args = append(args, "-kernel", cfg.KernelPath)
	}
	if cfg.InitrdPath != "" {
		args = append(args, "-initrd", cfg.InitrdPath)
		args = append(args, "-append", "console=ttyS0 root=/dev/vda")
	}

	// ISO boot option (Alpine)
	if cfg.KernelPath == "" && cfg.DiskPath == "" {
		args = append(args, "-cdrom", filepath.Join(os.Getenv("HOME"), "alpine-virt-3.19.1-x86_64.iso"))
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	m.cmd = cmd
	return nil
}

func (m *Manager) Stop() error {
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Kill()
	}
	return nil
}

func (m *Manager) Wait() error {
	if m.cmd != nil {
		return m.cmd.Wait()
	}
	return nil
}

func (m *Manager) Status() VMStatus {
	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(os.Signal(nil)); err == nil {
			return StatusRunning
		}
	}
	if _, err := os.Stat(m.qmpSock); err == nil {
		return StatusStopped
	}
	return StatusError
}

func (m *Manager) isRunning() bool {
	return m.Status() == StatusRunning
}

func (m *Manager) QMPSocket() string {
	return m.qmpSock
}

func (m *Manager) SerialSocket() string {
	return m.serialSock
}

func (m *Manager) QMPSocketExists() bool {
	_, err := os.Stat(m.qmpSock)
	return err == nil
}

func (m *Manager) WaitForQMP(timeout time.Duration) (*QMPClient, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.QMPSocketExists() {
			c, err := NewQMPClient(m.qmpSock)
			if err == nil {
				if err := c.Connect(); err == nil {
					return c, nil
				}
				c.Close()
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("qmp socket not available after %v", timeout)
}

func (m *Manager) WaitForSerial(timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", m.serialSock, 100*time.Millisecond)
		if err == nil {
			return conn, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("serial socket not available after %v", timeout)
}