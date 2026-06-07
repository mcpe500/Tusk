package vm

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

	return m.isQMPListening()
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
