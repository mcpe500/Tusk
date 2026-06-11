package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// hostBaseDir is the runtime root on a native Termux/Android host (no VM).
func hostBaseDir() string {
	return filepath.Join(os.Getenv("HOME"), ".tusk")
}

// runHostSocket runs the real proot-backed daemon on a Unix socket. This is
// the native (non-VM) execution path on Termux: containers really run via
// proot. It replaces the former simulation mode.
func runHostSocket(sockPath string) {
	base := hostBaseDir()
	fmt.Fprintf(os.Stderr, "=== Tuskd (native proot runtime) ===\n")
	fmt.Fprintf(os.Stderr, "Base dir: %s\n", base)
	fmt.Fprintf(os.Stderr, "Listening on: %s\n", sockPath)

	os.MkdirAll(filepath.Dir(sockPath), 0755)
	os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", sockPath, err)
		os.Exit(1)
	}
	defer ln.Close()

	store := NewContainerStore(filepath.Join(base, "containers"))
	rt := NewRuntime(base)

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

// runHostMode runs the native daemon on the default socket the CLI dials
// (~/.tusk/vm/serial.sock), so `tusk` works unchanged whether the backend is
// a real VM or the native proot runtime.
func runHostMode() {
	sockPath := filepath.Join(hostBaseDir(), "vm", "serial.sock")
	runHostSocket(sockPath)
}
