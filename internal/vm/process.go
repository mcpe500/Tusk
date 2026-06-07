package vm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

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

func (m *Manager) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(os.Signal(nil)) == nil
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
