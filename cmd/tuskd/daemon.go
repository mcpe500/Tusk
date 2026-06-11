package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var socketPath = "/tusk/vm/serial.sock"

func runDaemon() {
	os.MkdirAll(filepath.Dir(socketPath), 0755)
	os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Printf("Tuskd listening on %s\n", socketPath)
	store := NewContainerStore("/tusk/containers")
	rt := NewRuntime("/tusk")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			handleStream(c, c, store, rt)
		}(conn)
	}
}

func runDeviceDaemon(path string) {
	fmt.Printf("Tuskd listening on device %s\n", path)
	store := NewContainerStore("/tusk/containers")
	rt := NewRuntime("/tusk")
	paths := virtioDevicePaths(path)

	for {
		f, activePath, err := openFirstDevice(paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open any device (%v): %v\n", paths, err)
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Printf("Connected to device %s\n", activePath)
		handleStream(f, f, store, rt)
		f.Close()
		fmt.Printf("Device %s closed, reconnecting...\n", activePath)
		time.Sleep(1 * time.Second)
	}
}

func virtioDevicePaths(path string) []string {
	paths := []string{path}
	if path == "/dev/virtio-ports/tusk0" {
		paths = append(paths, "/dev/vport0p1", "/dev/vport0p2", "/dev/vport0p0")
	}
	return paths
}

func openFirstDevice(paths []string) (*os.File, string, error) {
	var lastErr error
	for _, p := range paths {
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err == nil {
			return f, p, nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

func handleStream(r io.Reader, w io.Writer, store *ContainerStore, rt *Runtime) {
	reader := bufio.NewReader(r)
	enc := json.NewEncoder(w)

	for {
		raw, err := readJSONObject(reader)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			}
			return
		}

		var req map[string]interface{}
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
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

		enc.Encode(handleRPC(store, rt, method, req))
	}
}
