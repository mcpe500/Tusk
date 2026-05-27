package vm

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type SerialClient struct {
	conn    net.Conn
	timeout time.Duration
}

func NewSerialClient(sockPath string, timeout time.Duration) (*SerialClient, error) {
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial serial: %w", err)
	}
	return &SerialClient{
		conn:    conn,
		timeout: timeout,
	}, nil
}

func (c *SerialClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *SerialClient) Send(method string, params interface{}) ([]byte, error) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      time.Now().UnixNano(),
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	resp := make([]byte, 4096)
	n, err := c.conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return resp[:n], nil
}

func (c *SerialClient) Ping() error {
	_, err := c.Send("Ping", nil)
	return err
}

func (c *SerialClient) Info() (map[string]interface{}, error) {
	data, err := c.Send("Info", nil)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal info: %w", err)
	}
	return resp, nil
}

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorInfo      `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

type ErrorInfo struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}