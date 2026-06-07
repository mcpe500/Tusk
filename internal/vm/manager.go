package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
