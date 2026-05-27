package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type QMPClient struct {
	sockPath string
	conn     net.Conn
	decoder  *json.Decoder
}

type QMPMessage struct {
	Type    string          `json:"QMP"`
	Event   string          `json:"event,omitempty"`
	Command string          `json:"command,omitempty"`
	Return  json.RawMessage `json:"return,omitempty"`
	Error   *QMPError       `json:"error,omitempty"`
	Id      interface{}     `json:"id,omitempty"`
}

type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

func NewQMPClient(sockPath string) (*QMPClient, error) {
	return &QMPClient{sockPath: sockPath}, nil
}

func (c *QMPClient) Connect() error {
	conn, err := net.DialTimeout("unix", c.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial qmp: %w", err)
	}
	c.conn = conn
	c.decoder = json.NewDecoder(conn)
	return nil
}

func (c *QMPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *QMPClient) Read() (*QMPMessage, error) {
	var msg QMPMessage
	if err := c.decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("decode qmp: %w", err)
	}
	return &msg, nil
}

func (c *QMPClient) Execute(cmd string, args map[string]interface{}) (json.RawMessage, error) {
	id := time.Now().UnixNano()

	req := map[string]interface{}{
		"execute": cmd,
		"arguments": args,
		"id": id,
	}

	enc := json.NewEncoder(c.conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("send qmp: %w", err)
	}

	// Read response
	for {
		msg, err := c.Read()
		if err != nil {
			return nil, err
		}
		if msg.Id == id {
			if msg.Error != nil {
				return nil, fmt.Errorf("qmp error: %s - %s", msg.Error.Class, msg.Error.Desc)
			}
			return msg.Return, nil
		}
	}
}

func (c *QMPClient) Greeting() (*QMPMessage, error) {
	msg, err := c.Read()
	if err != nil {
		return nil, err
	}
	if msg.Type != "QMP" {
		return nil, fmt.Errorf("expected QMP greeting, got %s", msg.Type)
	}
	return msg, nil
}

func (c *QMPClient) Handshake() error {
	// Read greeting
	_, err := c.Greeting()
	if err != nil {
		return fmt.Errorf("qmp greeting: %w", err)
	}

	// Send qmp_capabilities
	_, err = c.Execute("qmp_capabilities", map[string]interface{}{
		"enable": []string{"oob"},
	})
	return err
}

func (c *QMPClient) Stop() error {
	_, err := c.Execute("stop", nil)
	return err
}

func (c *QMPClient) Cont() error {
	_, err := c.Execute("cont", nil)
	return err
}

func (c *QMPClient) SystemReset() error {
	_, err := c.Execute("system_reset", nil)
	return err
}

func (c *QMPClient) SystemPowerdown() error {
	_, err := c.Execute("system_powerdown", nil)
	return err
}

func (c *QMPClient) Quit() error {
	_, err := c.Execute("quit", nil)
	return err
}

func (c *QMPClient) QueryStatus() (string, error) {
	result, err := c.Execute("query-status", nil)
	if err != nil {
		return "", err
	}
	var status map[string]interface{}
	if err := json.Unmarshal(result, &status); err != nil {
		return "", err
	}
	if running, ok := status["running"].(bool); ok {
		if running {
			return "running", nil
		}
	}
	return "stopped", nil
}

func (c *QMPClient) AddFD(fd int, name string) error {
	// For file descriptor passing (used in virtio-serial)
	return fmt.Errorf("not implemented")
}

func SendFD(conn io.Writer, name string, fd int) error {
	// Implementation for sending file descriptors via SCM_RIGHTS
	return fmt.Errorf("not implemented")
}