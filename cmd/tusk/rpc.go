package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runRPC() {
	if len(os.Args) < 3 {
		printRPCUsage()
		return
	}

	method := os.Args[2]
	if method == "-h" || method == "--help" {
		printRPCUsage()
		return
	}

	conn, err := net.DialTimeout("unix", filepath.Join(tuskDir, "vm", "serial.sock"), 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		fmt.Fprintf(os.Stderr, "Is the VM running? Run 'tusk start' first.\n")
		os.Exit(1)
	}
	defer conn.Close()

	req := map[string]interface{}{"jsonrpc": "2.0", "method": method, "id": time.Now().UnixNano()}
	paramArg := ""
	if len(os.Args) > 3 {
		paramArg = strings.Join(os.Args[3:], " ")
	}
	if paramArg != "" {
		var params interface{}
		if err := json.Unmarshal([]byte(paramArg), &params); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid params JSON: %v\n", err)
			fmt.Fprintf(os.Stderr, "Hint: pass params as one JSON object in one argument.\n")
			os.Exit(1)
		}
		req["params"] = params
	}

	reqPretty, err := json.MarshalIndent(req, "", "  ")
	if err == nil {
		fmt.Println("Request:")
		fmt.Println(string(reqPretty))
	}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write request: %v\n", err)
		os.Exit(1)
	}

	var rawResp json.RawMessage
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&rawResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read response: %v\n", err)
		os.Exit(1)
	}

	respPretty, err := json.MarshalIndent(rawResp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to format response: %v\n", err)
		fmt.Println(string(rawResp))
		os.Exit(1)
	}

	fmt.Println("Response:")
	fmt.Println(string(respPretty))
}

func printRPCUsage() {
	fmt.Println("Usage: tusk rpc <method> [params-json]")
	fmt.Println("Examples:")
	fmt.Println("  tusk rpc ping")
	fmt.Println("  tusk rpc ContainerList '{\"all\":true}'")
}
