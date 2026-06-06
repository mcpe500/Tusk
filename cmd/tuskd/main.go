package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tusk/tusk/pkg/types"
)

var (
	socketPath = "/tusk/vm/serial.sock"
)

func main() {
	// Check if running in VM or simulation mode
	if _, err := os.Stat("/tusk"); os.IsNotExist(err) {
		// Simulation mode for testing
		runSimulationMode()
		return
	}

	// Production mode - run as daemon
	runDaemon()
}

func runSimulationMode() {
	fmt.Println("=== Tuskd Simulation Mode ===")
	fmt.Println("Running in host simulation mode (VM mode not active)")
	fmt.Println("Commands will be processed but containers won't actually run")
	fmt.Println()

	store := NewContainerStore(filepath.Join(os.Getenv("HOME"), ".tusk", "containers"))

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("tuskd> ")

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			fmt.Print("tuskd> ")
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			fmt.Print("tuskd> ")
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "ping":
			fmt.Println(`{"jsonrpc":"2.0","result":"pong","id":1}`)
		case "info":
			fmt.Printf(`{"jsonrpc":"2.0","result":{"version":"1.0.0","apiVersion":"v1","os":"linux","arch":"x86_64"},"id":1}`)
			fmt.Println()
		case "containers":
			containers := store.List()
			data, _ := json.Marshal(map[string]interface{}{
				"containers": containers,
			})
			fmt.Printf(`{"jsonrpc":"2.0","result":%s,"id":1}`, string(data))
			fmt.Println()
		case "create":
			if len(args) < 2 {
				fmt.Println(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"usage: create <image> <name>"}}`)
			} else {
				c := &types.ContainerInfo{
					ID:    generateID(),
					Name:  args[1],
					Image: args[0],
				}
				store.Create(c)
				data, _ := json.Marshal(c)
				fmt.Printf(`{"jsonrpc":"2.0","result":%s,"id":1}`, string(data))
				fmt.Println()
			}
		case "help":
			fmt.Println("Available commands: ping, info, containers, create, exit, help")
		default:
			fmt.Println(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"method not found"}}`)
		}
		fmt.Print("tuskd> ")
	}
}

func runDaemon() {
	// Ensure socket directory exists
	os.MkdirAll(filepath.Dir(socketPath), 0755)

	// Remove existing socket
	os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Printf("Tuskd listening on %s\n", socketPath)

	store := NewContainerStore("/tusk/containers")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, store)
	}
}

func handleConnection(conn net.Conn, store *ContainerStore) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var req map[string]interface{}
		if err := dec.Decode(&req); err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "Decode error: %v\n", err)
			}
			return
		}

		jsonrpc := req["jsonrpc"]
		if v, ok := jsonrpc.(string); ok && v != "2.0" {
			writeJSONRPCError(enc, req["id"], -32600, "invalid request: jsonrpc must be 2.0")
			continue
		}

		method, ok := req["method"].(string)
		if !ok || strings.TrimSpace(method) == "" {
			writeJSONRPCError(enc, req["id"], -32600, "invalid request: method is missing or not a string")
			continue
		}

		id := req["id"]

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
		}

		switch method {
		case "Ping":
			resp["result"] = "pong"
		case "Info":
			resp["result"] = map[string]string{
				"version":    "1.0.0",
				"apiVersion": "v1",
				"os":         "linux",
				"arch":       "x86_64",
			}
		case "ContainerList":
			resp["result"] = map[string]interface{}{
				"containers": store.List(),
			}
		case "ContainerCreate":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			image := strings.TrimSpace(getStringOrDefault(params["image"]))
			if image == "" {
				resp["error"] = errorObject(-32602, "invalid params: image is required")
				break
			}
			c := &types.ContainerInfo{
				ID:    generateID(),
				Name:  getStringOrDefault(params["name"]),
				Image: image,
			}
			if rawCmd, exists := params["command"]; exists {
				cmd, err := asStringSlice(rawCmd)
				if err != nil {
					resp["error"] = errorObject(-32602, "invalid params: command must be []string")
					break
				}
				c.Cmd = cmd
			}
			if rawEnv, exists := params["env"]; exists {
				env, err := asStringSlice(rawEnv)
				if err != nil {
					resp["error"] = errorObject(-32602, "invalid params: env must be []string")
					break
				}
				c.Env = env
			}
			store.Create(c)
			resp["result"] = c
		case "ContainerStart":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			idStr, ok := params["id"].(string)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params: id is required")
				break
			}
			store.Start(idStr)
			resp["result"] = map[string]string{"status": "started"}
		case "ContainerStop":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			idStr, ok := params["id"].(string)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params: id is required")
				break
			}
			store.Stop(idStr)
			resp["result"] = map[string]string{"status": "stopped"}
		case "ContainerExec":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			containerID, ok := params["containerId"].(string)
			_ = containerID // TODO: use for actual container lookup
			var cmd []string
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params: containerId is required")
				break
			}
			if rawCmd, exists := params["command"]; exists {
				cmdParsed, err := asStringSlice(rawCmd)
				if err != nil {
					resp["error"] = errorObject(-32602, "invalid params: command must be []string")
					break
				}
				cmd = cmdParsed
			}

			// Execute the command in the container
			// For now, execute directly using /bin/sh -c
			var stdout, stderr string
			var exitCode int

			if len(cmd) == 0 {
				exitCode = 1
				stderr = "No command specified"
			} else {
				execCmd := exec.Command(cmd[0], cmd[1:]...)
				output, err := execCmd.CombinedOutput()
				if err != nil {
					exitCode = 1
					stderr = string(output)
				} else {
					exitCode = 0
					stdout = string(output)
				}
			}

			resp["result"] = map[string]interface{}{
				"exitCode": exitCode,
				"stdout":   stdout,
				"stderr":   stderr,
			}
		case "ContainerRemove":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			idStr, ok := params["id"].(string)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params: id is required")
				break
			}
			store.Remove(idStr)
			resp["result"] = map[string]string{"status": "removed"}
		case "ContainerLogs":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			idStr, ok := params["id"].(string)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params: id is required")
				break
			}
			logs := store.Logs(idStr)
			resp["result"] = map[string]string{"logs": logs}
		case "ImagePull":
			// Would download image from registry
			resp["result"] = map[string]string{"status": "pulled"}
		case "ImageList":
			resp["result"] = map[string]interface{}{
				"images": []types.ImageInfo{},
			}
		case "NetworkCreate":
			params, ok := reqParams(req)
			if !ok {
				resp["error"] = errorObject(-32602, "invalid params")
				break
			}
			name := getStringOrDefault(params["name"])
			if strings.TrimSpace(name) == "" {
				resp["error"] = errorObject(-32602, "invalid params: name is required")
				break
			}
			resp["result"] = map[string]string{
				"id":   generateID(),
				"name": name,
			}
		case "NetworkList":
			resp["result"] = map[string]interface{}{
				"networks": []types.NetworkInfo{},
			}
		default:
			resp["error"] = map[string]interface{}{
				"code":    -32601,
				"message": "method not found",
			}
		}

		enc.Encode(resp)
	}
}

func writeJSONRPCError(enc *json.Encoder, id interface{}, code int, message string) {
	enc.Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorObject(code, message),
	})
}

func errorObject(code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"code":    code,
		"message": message,
	}
}

func reqParams(req map[string]interface{}) (map[string]interface{}, bool) {
	raw, ok := req["params"]
	if !ok || raw == nil {
		return map[string]interface{}{}, true
	}
	params, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	return params, true
}

func asStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("value is not a string")
			}
			result = append(result, s)
		}
		return result, nil
	case []string:
		return append([]string(nil), v...), nil
	default:
		return nil, fmt.Errorf("value is not an array")
	}
}

func getStringOrDefault(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

type ContainerStore struct {
	baseDir string
}

func NewContainerStore(baseDir string) *ContainerStore {
	os.MkdirAll(baseDir, 0755)
	return &ContainerStore{baseDir: baseDir}
}

func (s *ContainerStore) Create(c *types.ContainerInfo) {
	data, _ := json.Marshal(c)
	path := filepath.Join(s.baseDir, c.ID+".json")
	os.WriteFile(path, data, 0644)
}

func (s *ContainerStore) Get(id string) (*types.ContainerInfo, error) {
	path := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c types.ContainerInfo
	json.Unmarshal(data, &c)
	return &c, nil
}

func (s *ContainerStore) List() []types.ContainerInfo {
	var containers []types.ContainerInfo
	entries, _ := os.ReadDir(s.baseDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			path := filepath.Join(s.baseDir, e.Name())
			data, _ := os.ReadFile(path)
			var c types.ContainerInfo
			json.Unmarshal(data, &c)
			containers = append(containers, c)
		}
	}
	return containers
}

func (s *ContainerStore) Start(id string) {
	if c, err := s.Get(id); err == nil {
		c.State = types.StatusRunning
		c.Pid = 12345 // Simulated PID
		s.Create(c)
	}
}

func (s *ContainerStore) Stop(id string) {
	if c, err := s.Get(id); err == nil {
		c.State = types.StatusStopped
		s.Create(c)
	}
}

func (s *ContainerStore) Remove(id string) {
	path := filepath.Join(s.baseDir, id+".json")
	os.Remove(path)
}

func (s *ContainerStore) Logs(id string) string {
	return fmt.Sprintf("[%s] Container logs for %s\n", time.Now().Format(time.RFC3339), id)
}

func generateID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
