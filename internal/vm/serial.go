package vm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type SerialClient struct {
	conn    net.Conn
	timeout time.Duration
	reader  *bufio.Reader
	mu      sync.Mutex
}

func NewSerialClient(sockPath string, timeout time.Duration) (*SerialClient, error) {
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial serial: %w", err)
	}
	return &SerialClient{
		conn:    conn,
		timeout: timeout,
		reader:  bufio.NewReader(conn),
	}, nil
}

func (c *SerialClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *SerialClient) readJSONObject() ([]byte, error) {
	// Skip leading garbage until '{'
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '{' {
			if err := c.reader.UnreadByte(); err != nil {
				return nil, err
			}
			break
		}
	}

	var buf bytes.Buffer
	braces := 0
	inString := false
	escaped := false

	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' {
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if b == '{' {
				braces++
			} else if b == '}' {
				braces--
				if braces == 0 {
					return buf.Bytes(), nil
				}
			}
		}
	}
}

func (c *SerialClient) Send(method string, params interface{}) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	// Read and parse responses until we find a valid JSON-RPC one
	for {
		raw, err := c.readJSONObject()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("connection closed")
			}
			return nil, fmt.Errorf("read response: %w", err)
		}

		// Try to unmarshal into a generic map to check if it's a valid JSON-RPC response
		var resp map[string]interface{}
		if err := json.Unmarshal(raw, &resp); err != nil {
			// Not a valid JSON object, probably a log message that happened to contain braces
			continue
		}

		// Basic validation
		if resp["jsonrpc"] != "2.0" {
			continue
		}

		return raw, nil
	}
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

	// Handle result field
	if result, ok := resp["result"].(map[string]interface{}); ok {
		return result, nil
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
