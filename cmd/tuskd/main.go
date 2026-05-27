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
	socketPath = "/tusk/serial.sock"
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

		method, _ := req["method"].(string)
		id := req["id"]

		var resp map[string]interface{}
		resp = map[string]interface{}{
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
			params := req["params"].(map[string]interface{})
			c := &types.ContainerInfo{
				ID:    generateID(),
				Name:  params["name"].(string),
				Image: params["image"].(string),
			}
			if cmd, ok := params["command"].([]interface{}); ok {
				for _, v := range cmd {
					c.Cmd = append(c.Cmd, v.(string))
				}
			}
			if env, ok := params["env"].([]interface{}); ok {
				for _, v := range env {
					c.Env = append(c.Env, v.(string))
				}
			}
			store.Create(c)
			resp["result"] = c
		case "ContainerStart":
			params := req["params"].(map[string]interface{})
			id := params["id"].(string)
			store.Start(id)
			resp["result"] = map[string]string{"status": "started"}
		case "ContainerStop":
			params := req["params"].(map[string]interface{})
			id := params["id"].(string)
			store.Stop(id)
			resp["result"] = map[string]string{"status": "stopped"}
		case "ContainerExec":
			params := req["params"].(map[string]interface{})
			containerID := params["containerId"].(string)
			_ = containerID // TODO: use for actual container lookup
			var cmd []string
			if cmdList, ok := params["command"].([]interface{}); ok {
				for _, v := range cmdList {
					cmd = append(cmd, v.(string))
				}
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
			params := req["params"].(map[string]interface{})
			id := params["id"].(string)
			store.Remove(id)
			resp["result"] = map[string]string{"status": "removed"}
		case "ContainerLogs":
			params := req["params"].(map[string]interface{})
			id := params["id"].(string)
			logs := store.Logs(id)
			resp["result"] = map[string]string{"logs": logs}
		case "ImagePull":
			// Would download image from registry
			resp["result"] = map[string]string{"status": "pulled"}
		case "ImageList":
			resp["result"] = map[string]interface{}{
				"images": []types.ImageInfo{},
			}
		case "NetworkCreate":
			params := req["params"].(map[string]interface{})
			resp["result"] = map[string]string{
				"id":   generateID(),
				"name": params["name"].(string),
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