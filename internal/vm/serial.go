package vm

import (
	"bytes"
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

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}

	enc := json.NewEncoder(c.conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}

	// Read response
	dec := json.NewDecoder(c.conn)
	var resp json.RawMessage
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(bytes.TrimSpace(resp)) == 0 {
		return nil, fmt.Errorf("empty rpc response")
	}

	return resp, nil
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
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty info response")
	}
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
