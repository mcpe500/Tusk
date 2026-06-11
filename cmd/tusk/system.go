package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tusk/tusk/internal/client"
	"github.com/tusk/tusk/internal/image"
)

func runVersion() {
	fmt.Println("tusk version 0.1.0")
	fmt.Println("Tusk: Native container runtime for Termux/Android.")
}

func runUpdate() {
	fmt.Println("Updating Tusk...")

	tuskBin := filepath.Join(os.Getenv("HOME"), "tusk")
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: git not found. Cannot update.\n")
		os.Exit(1)
	}

	tuskRepo := filepath.Join(os.Getenv("HOME"), "Tusk")
	fmt.Println("Pulling latest from GitHub...")
	cmd := exec.Command("git", "-C", tuskRepo, "pull", "origin", "main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to pull updates: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Building tusk...")
	cmd = exec.Command("go", "build", "-o", tuskBin, "./cmd/tusk")
	cmd.Dir = tuskRepo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to build tusk: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Building tuskd (native)...")
	tuskdBin := filepath.Join(os.Getenv("HOME"), ".tusk", "tuskd")
	cmd = exec.Command("go", "build", "-ldflags=-s -w", "-o", tuskdBin, "./cmd/tuskd")
	cmd.Dir = tuskRepo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to build tuskd: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("")
	fmt.Println("Update complete!")
	fmt.Printf("Run 'tusk version' to verify\n")
}

func runInstall() {

	fmt.Println("tusk install: Use the install.sh script instead:")
	fmt.Println("  curl -fsSL https://raw.githubusercontent.com/mcpe500/Tusk/main/scripts/install.sh | bash")
}

func verifyInstallation(verbose bool) error {
	sockPath := filepath.Join(tuskDir, "vm", "serial.sock")
	if verbose {
		fmt.Println("Verifying tuskd socket and RPC readiness...")
	}

	// Wait up to 20s for socket to appear.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if _, err := os.Stat(sockPath); err != nil {
		return fmt.Errorf("tuskd socket not found at %s", sockPath)
	}

	c := client.New(sockPath)
	if err := c.Connect(); err != nil {
		return fmt.Errorf("failed to connect to tuskd: %w", err)
	}
	defer c.Close()

	if err := c.Ping(); err != nil {
		return fmt.Errorf("tuskd ping failed: %w", err)
	}
	if verbose {
		fmt.Println("tuskd is responding over RPC")
	}
	return nil
}

func runInit() {
	fmt.Println("Initializing Tusk...")

	store := image.New(filepath.Join(tuskDir, "images"))
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init image store: %v\n", err)
		os.Exit(1)
	}

	// Create required data directories.
	for _, dir := range []string{
		filepath.Join(tuskDir, "vm"),
		filepath.Join(tuskDir, "containers"),
		filepath.Join(tuskDir, "volumes"),
		filepath.Join(tuskDir, "tmp"),
	} {
		_ = os.MkdirAll(dir, 0755)
	}

	fmt.Println("Tusk initialized successfully!")
	fmt.Printf("Data directory: %s\n", tuskDir)
}

func runStart() {
	fmt.Println("Starting Tusk daemon...")
	startNativeDaemon()
}

// startNativeDaemon launches tuskd (proot runtime) on the native socket.
func startNativeDaemon() {
	sockPath := filepath.Join(tuskDir, "vm", "serial.sock")
	os.MkdirAll(filepath.Dir(sockPath), 0755)

	// Already running?
	if _, err := os.Stat(sockPath); err == nil {
		cli := client.New(sockPath)
		cli.SetTimeout(2 * time.Second)
		if err := cli.Connect(); err == nil {
			if err := cli.Ping(); err == nil {
				fmt.Println("Tusk daemon already running.")
				cli.Close()
				return
			}
			cli.Close()
		}
		os.Remove(sockPath)
	}

	// Prefer installed binary; fall back to building from source.
	tuskdPath := filepath.Join(tuskDir, "tuskd")
	if _, err := os.Stat(tuskdPath); err != nil {
		repo := filepath.Join(os.Getenv("HOME"), "Tusk")
		if _, err := os.Stat(filepath.Join(repo, "cmd", "tuskd")); err == nil {
			fmt.Println("Building tuskd...")
			build := exec.Command("go", "build", "-o", tuskdPath, "./cmd/tuskd")
			build.Dir = repo
			if err := build.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to build tuskd: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if _, err := os.Stat(tuskdPath); err != nil {
		fmt.Fprintf(os.Stderr, "tuskd not found at %s — run the installer first\n", tuskdPath)
		os.Exit(1)
	}

	logPath := filepath.Join(tuskDir, "tuskd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open log file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting tuskd (proot runtime)...")
	cmd := exec.Command(tuskdPath, "--socket", sockPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		fmt.Fprintf(os.Stderr, "Failed to start tuskd: %v\n", err)
		os.Exit(1)
	}
	logFile.Close()

	// Wait for socket to appear and respond.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			cli := client.New(sockPath)
			if err := cli.Connect(); err == nil {
				if err := cli.Ping(); err == nil {
					fmt.Printf("Tusk daemon ready! (PID: %d)\n", cmd.Process.Pid)
					fmt.Printf("Log: %s\n", logPath)
					cli.Close()
					return
				}
				cli.Close()
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "Daemon did not respond in time. Check %s\n", logPath)
	os.Exit(1)
}

func runStop() {
	exec.Command("pkill", "-f", "tuskd").Run()
	os.Remove(filepath.Join(tuskDir, "vm", "serial.sock"))
	fmt.Println("Tusk daemon stopped.")
}

func runStatus() {
	sockPath := filepath.Join(tuskDir, "vm", "serial.sock")
	if _, err := os.Stat(sockPath); err != nil {
		fmt.Println("Tusk daemon: not running")
		fmt.Printf("Socket: %s (missing)\n", sockPath)
		return
	}

	cli := client.New(sockPath)
	cli.SetTimeout(2 * time.Second)
	if err := cli.Connect(); err != nil {
		fmt.Printf("Tusk daemon: socket present but not connectable: %v\n", err)
		return
	}
	defer cli.Close()

	if err := cli.Ping(); err != nil {
		fmt.Printf("Tusk daemon: not responding: %v\n", err)
		return
	}
	fmt.Println("Tusk daemon: running (proot runtime)")
	fmt.Printf("Socket: %s\n", sockPath)
}

func runUninstall() {
	// Check for -y flag
	autoYes := false
	for _, arg := range os.Args[2:] {
		if arg == "-y" || arg == "--yes" || arg == "-f" || arg == "--force" {
			autoYes = true
		}
	}

	fmt.Println("==================================")
	fmt.Println("  Tusk Uninstaller")
	fmt.Println("==================================")
	fmt.Println("")
	fmt.Println("WARNING: This will delete:")
	fmt.Printf("1. Tusk data directory (%s) - Includes images and containers!\n", tuskDir)
	fmt.Printf("2. Tusk binary (%s)\n", filepath.Join(os.Getenv("HOME"), "tusk"))
	fmt.Printf("3. Tusk source repository (%s)\n", filepath.Join(os.Getenv("HOME"), "Tusk"))
	fmt.Println("")

	if !autoYes {
		fmt.Print("Are you sure you want to proceed? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			fmt.Println("Uninstall cancelled.")
			return
		}
	}

	exec.Command("pkill", "-f", "tuskd").Run()
	time.Sleep(500 * time.Millisecond)

	fmt.Println("Removing tusk data directory...")
	if err := os.RemoveAll(tuskDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove data directory: %v\n", err)
	}

	tuskBin := filepath.Join(os.Getenv("HOME"), "tusk")
	fmt.Println("Removing tusk binary...")
	if err := os.Remove(tuskBin); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove tusk binary: %v\n", err)
	}

	tuskSource := filepath.Join(os.Getenv("HOME"), "Tusk")
	fmt.Println("Removing tusk source repository...")
	if err := os.RemoveAll(tuskSource); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove source repository: %v\n", err)
	}

	fmt.Println("\nTusk uninstalled successfully!")
	fmt.Println("Note: If you added '~/tusk' to your PATH manually, you may want to remove it from your .bashrc/.zshrc.")
}

// ensureSocket returns the tuskd socket path if the daemon is reachable,
// or an error with a helpful message if it is not.
func ensureSocket() (string, error) {
	sockPath := filepath.Join(tuskDir, "vm", "serial.sock")
	if _, err := os.Stat(sockPath); err != nil {
		return "", fmt.Errorf("tuskd not running (socket missing: %s). Run 'tusk start' first", sockPath)
	}
	cli := client.New(sockPath)
	cli.SetTimeout(5 * time.Second)
	if err := cli.Connect(); err != nil {
		return "", fmt.Errorf("cannot connect to tuskd: %w", err)
	}
	pingErr := cli.Ping()
	cli.Close()
	if pingErr != nil {
		return "", fmt.Errorf("tuskd not responding: %w", pingErr)
	}
	return sockPath, nil
}

// ensureVM is kept for backward compatibility with compose.go callers that
// only need the socket path. Returns a fake *struct{} so callers can keep
// the two-value assignment; use the string form (ensureSocket) for new code.
func ensureVM() (interface{ SerialSocket() string }, error) {
	sock, err := ensureSocket()
	if err != nil {
		return nil, err
	}
	return &socketWrapper{sock}, nil
}

type socketWrapper struct{ path string }

func (s *socketWrapper) SerialSocket() string { return s.path }

func execLookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", file)
	}
	return path, nil
}
