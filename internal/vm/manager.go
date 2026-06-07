package vm

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Manager struct {
	baseDir    string
	vmDir      string
	sockDir    string
	qmpSock    string
	serialSock string
	pidFile    string
	cmd        *exec.Cmd
}

type Config struct {
	Memory     int
	CPUs       int
	DiskPath   string
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
		pidFile:    filepath.Join(vmDir, "qemu.pid"),
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
	args = append(args, "-device", "virtio-serial-pci")
	args = append(args, "-device", "virtserialport,chardev=ch0,name=tusk0")
	args = append(args, "-chardev", fmt.Sprintf("socket,id=ch0,path=%s,server,nowait", m.serialSock))

	// Dedicated serial port for console/logs
	consoleSock := filepath.Join(m.vmDir, "console.sock")
	args = append(args, "-serial", fmt.Sprintf("unix:%s,server,nowait", consoleSock))

	// Clean up stale sockets before starting a new VM process.
	_ = os.Remove(m.qmpSock)
	_ = os.Remove(m.serialSock)
	_ = os.Remove(consoleSock)

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
	if err := m.writePID(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	return nil
}

func (m *Manager) Stop() error {
	var stopErr error

	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(os.Kill); err != nil && !isProcessNotRunning(err) {
			stopErr = err
		}
	} else if pid, err := m.readPID(); err == nil {
		if err := m.killPID(pid); err != nil && !isProcessNotRunning(err) {
			stopErr = err
		}
	}

	_ = m.clearPID()
	_ = os.Remove(m.qmpSock)
	_ = os.Remove(m.serialSock)

	if stopErr != nil {
		return stopErr
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
	if m.isRunning() {
		return StatusRunning
	}

	if m.QMPSocketExists() {
		return StatusStopped
	}
	return StatusError
}

func (m *Manager) isRunning() bool {
	if m.cmd != nil && m.cmd.Process != nil {
		if m.isProcessAlive(m.cmd.Process.Pid) {
			return true
		}
	}

	pid, err := m.readPID()
	if err == nil {
		if m.isProcessAlive(pid) {
			return true
		}

		_ = m.clearPID()
	}

	if m.isQMPListening() {
		return true
	}

	return false
}

func (m *Manager) isQMPListening() bool {
	if !m.QMPSocketExists() {
		return false
	}

	qmp, err := NewQMPClient(m.qmpSock)
	if err != nil {
		return false
	}

	if err := qmp.Connect(); err == nil {
		_ = qmp.Close()
		return true
	}

	return false
}

func (m *Manager) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := proc.Signal(os.Signal(nil)); err == nil {
		return true
	}
	return false
}

func (m *Manager) killPID(pid int) error {
	if pid <= 0 {
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Signal(os.Signal(nil)); err != nil {
		return err
	}

	return proc.Kill()
}

func (m *Manager) writePID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	return os.WriteFile(m.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func (m *Manager) readPID() (int, error) {
	data, err := os.ReadFile(m.pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func (m *Manager) clearPID() error {
	return os.Remove(m.pidFile)
}

func isProcessNotRunning(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.Contains(msg, "no such process") ||
		strings.Contains(msg, "process already finished") ||
		strings.Contains(msg, "not found")
}

func (m *Manager) QMPSocket() string {
	return m.qmpSock
}

func (m *Manager) SerialSocket() string {
	return m.serialSock
}

func (m *Manager) ConsoleSocket() string {
	return filepath.Join(m.vmDir, "console.sock")
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
