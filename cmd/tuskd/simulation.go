package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/pkg/types"
)

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

		if stop := runSimulationCommand(store, parts[0], parts[1:]); stop {
			return
		}
		fmt.Print("tuskd> ")
	}
}

func runSimulationCommand(store *ContainerStore, cmd string, args []string) bool {
	switch cmd {
	case "exit", "quit":
		fmt.Println("Goodbye!")
		return true
	case "ping":
		fmt.Println(`{"jsonrpc":"2.0","result":"pong","id":1}`)
	case "info":
		fmt.Printf(`{"jsonrpc":"2.0","result":{"version":"1.0.0","apiVersion":"v1","os":"linux","arch":"x86_64"},"id":1}`)
		fmt.Println()
	case "containers":
		containers := store.List()
		data, _ := json.Marshal(map[string]interface{}{"containers": containers})
		fmt.Printf(`{"jsonrpc":"2.0","result":%s,"id":1}`, string(data))
		fmt.Println()
	case "create":
		if len(args) < 2 {
			fmt.Println(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"usage: create <image> <name>"}}`)
			return false
		}
		c := &types.ContainerInfo{ID: generateID(), Name: args[1], Image: args[0]}
		store.Create(c)
		data, _ := json.Marshal(c)
		fmt.Printf(`{"jsonrpc":"2.0","result":%s,"id":1}`, string(data))
		fmt.Println()
	case "help":
		fmt.Println("Available commands: ping, info, containers, create, exit, help")
	default:
		fmt.Println(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"method not found"}}`)
	}
	return false
}
