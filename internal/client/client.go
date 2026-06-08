package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tusk/tusk/pkg/protocol"
)

type Client struct {
	sockPath string
	timeout  time.Duration
	conn     net.Conn
	reader   *bufio.Reader
	mu       sync.Mutex
}

func New(sockPath string) *Client {
	return &Client{
		sockPath: sockPath,
		timeout:  120 * time.Second,
	}
}

func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

func (c *Client) Connect() error {
	conn, err := net.DialTimeout("unix", c.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) readJSONObject() ([]byte, error) {
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

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      time.Now().UnixNano(),
	}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}

	enc := json.NewEncoder(c.conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("send: %w", err)
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
			return nil, fmt.Errorf("read: %w", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			// Not a valid JSON-RPC object, probably a log message that happened to contain braces
			// Skip and continue reading
			continue
		}

		// Basic validation that this is our response
		if resp.JSONRPC != "2.0" {
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if len(bytes.TrimSpace(resp.Result)) == 0 {
			return nil, fmt.Errorf("empty rpc result")
		}

		return resp.Result, nil
	}
}

func unmarshalResult(method string, raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("%s: empty result", method)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

func (c *Client) Ping() error {
	_, err := c.call("Ping", nil)
	return err
}

func (c *Client) Info() (*InfoResult, error) {
	data, err := c.call("Info", nil)
	if err != nil {
		return nil, err
	}
	var result InfoResult
	if err := unmarshalResult("info", data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ContainerCreate(params *protocol.ContainerCreateParams) (*protocol.ContainerCreateResult, error) {
	data, err := c.call("ContainerCreate", params)
	if err != nil {
		return nil, err
	}
	var result protocol.ContainerCreateResult
	if err := unmarshalResult("container-create", data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ContainerList(all bool) ([]protocol.ContainerInfo, error) {
	data, err := c.call("ContainerList", protocol.ContainerListParams{All: all})
	if err != nil {
		return nil, err
	}
	var result struct {
		Containers []protocol.ContainerInfo `json:"containers"`
	}
	if err := unmarshalResult("container-list", data, &result); err != nil {
		return nil, err
	}
	return result.Containers, nil
}

func (c *Client) ContainerStart(id string) error {
	_, err := c.call("ContainerStart", map[string]string{"id": id})
	return err
}

func (c *Client) ContainerStop(id string) error {
	_, err := c.call("ContainerStop", map[string]string{"id": id})
	return err
}

func (c *Client) ContainerRemove(id string, force bool) error {
	_, err := c.call("ContainerRemove", map[string]interface{}{"id": id, "force": force})
	return err
}

func (c *Client) ContainerExec(id string, cmd []string) (*protocol.ContainerExecResult, error) {
	data, err := c.call("ContainerExec", protocol.ContainerExecParams{
		ContainerID: id,
		Command:     cmd,
	})
	if err != nil {
		return nil, err
	}
	var result protocol.ContainerExecResult
	if err := unmarshalResult("container-exec", data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ContainerLogs(id string) (string, error) {
	data, err := c.call("ContainerLogs", map[string]string{"id": id})
	if err != nil {
		return "", err
	}
	var result struct {
		Logs string `json:"logs"`
	}
	if err := unmarshalResult("container-logs", data, &result); err != nil {
		return "", err
	}
	return result.Logs, nil
}

func (c *Client) ImagePull(ref string) error {
	_, err := c.call("ImagePull", protocol.ImagePullParams{Reference: ref})
	return err
}

func (c *Client) ImageList() ([]ImageInfo, error) {
	data, err := c.call("ImageList", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Images []ImageInfo `json:"images"`
	}
	if err := unmarshalResult("image-list", data, &result); err != nil {
		return nil, err
	}
	return result.Images, nil
}

func (c *Client) NetworkCreate(name, driver string) error {
	_, err := c.call("NetworkCreate", protocol.NetworkCreateParams{Name: name, Driver: driver})
	return err
}

func (c *Client) NetworkList() ([]NetworkInfo, error) {
	data, err := c.call("NetworkList", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Networks []NetworkInfo `json:"networks"`
	}
	if err := unmarshalResult("network-list", data, &result); err != nil {
		return nil, err
	}
	return result.Networks, nil
}

type InfoResult struct {
	Version    string `json:"version"`
	APIVersion string `json:"apiVersion"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

type ImageInfo struct {
	ID      string   `json:"id"`
	Tags    []string `json:"tags"`
	Size    int64    `json:"size"`
	Created string   `json:"created"`
}

type NetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}
