package vm

import (
	"fmt"
	"net"
	"time"
)

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
