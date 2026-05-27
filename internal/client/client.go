package client

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/tusk/tusk/pkg/protocol"
)

type Client struct {
	sockPath string
	timeout  time.Duration
	conn     net.Conn
}

func New(sockPath string) *Client {
	return &Client{
		sockPath: sockPath,
		timeout:  30 * time.Second,
	}
}

func (c *Client) Connect() error {
	conn, err := net.DialTimeout("unix", c.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
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

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	reqData = append(reqData, '\n')

	if _, err := c.conn.Write(reqData); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Read response
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	respData := make([]byte, 65536)
	n, err := c.conn.Read(respData)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(respData[:n], &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
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
	if err := json.Unmarshal(data, &result); err != nil {
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
	if err := json.Unmarshal(data, &result); err != nil {
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
	if err := json.Unmarshal(data, &result); err != nil {
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
		Command:    cmd,
	})
	if err != nil {
		return nil, err
	}
	var result protocol.ContainerExecResult
	if err := json.Unmarshal(data, &result); err != nil {
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
	if err := json.Unmarshal(data, &result); err != nil {
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
	if err := json.Unmarshal(data, &result); err != nil {
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
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Networks, nil
}

type InfoResult struct {
	Version   string `json:"version"`
	APIVersion string `json:"apiVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

type ImageInfo struct {
	ID       string `json:"id"`
	Tags     []string `json:"tags"`
	Size     int64  `json:"size"`
	Created  string `json:"created"`
}

type NetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}