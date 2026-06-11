package protocol

import (
	"encoding/json"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

const (
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternalError  = -32603
)

type ContainerCreateParams struct {
	Image    string            `json:"image"`
	Name     string            `json:"name,omitempty"`
	Command  []string          `json:"command,omitempty"`
	Env      []string          `json:"env,omitempty"`
	Mounts   []MountParams     `json:"mounts,omitempty"`
	Ports    []string          `json:"ports,omitempty"`
	Network  string            `json:"network,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type MountParams struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type ContainerCreateResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Pid    int    `json:"pid"`
	IPAddress string `json:"ipAddress"`
}

type ContainerListParams struct {
	All bool `json:"all,omitempty"`
}

type ContainerListResult struct {
	Containers []ContainerInfo `json:"containers"`
}

type ContainerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	Created   string `json:"created"`
	IPAddress string `json:"ipAddress,omitempty"`
}

type ContainerExecParams struct {
	ContainerID string   `json:"containerId"`
	Command     []string `json:"command"`
	AttachStdin bool     `json:"attachStdin,omitempty"`
	AttachStdout bool    `json:"attachStdout,omitempty"`
	AttachStderr bool    `json:"attachStderr,omitempty"`
	Tty         bool     `json:"tty,omitempty"`
}

type ContainerExecResult struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

type ImagePullParams struct {
	Reference string `json:"reference"`
}

type ImagePullResult struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

type NetworkCreateParams struct {
	Name    string `json:"name"`
	Driver  string `json:"driver,omitempty"`
	Subnet  string `json:"subnet,omitempty"`
}

type NetworkCreateResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type API struct{}

func (API) Methods() map[string]interface{} {
	return map[string]interface{}{
		"ContainerCreate":    nil,
		"ContainerList":     nil,
		"ContainerStart":     nil,
		"ContainerStop":      nil,
		"ContainerRemove":    nil,
		"ContainerExec":      nil,
		"ContainerLogs":      nil,
		"ContainerInspect":   nil,
		"ImagePull":          nil,
		"ImageList":          nil,
		"NetworkCreate":      nil,
		"NetworkList":        nil,
		"NetworkRemove":      nil,
		"Ping":               nil,
		"Info":               nil,
	}
}