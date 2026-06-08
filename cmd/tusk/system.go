package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tusk/tusk/internal/client"
	"github.com/tusk/tusk/internal/image"
	"github.com/tusk/tusk/internal/vm"
)

func runVersion() {
	fmt.Println("tusk version 0.1.0")
	fmt.Println("Tusk: Hardware emulation for Termux, because sometimes working is better than fast.")
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

	fmt.Println("Building tuskd (x86_64)...")
	tuskdBin := filepath.Join(os.Getenv("HOME"), ".tusk", "tuskd-amd64")
	cmd = exec.Command("go", "build", "-ldflags=-s -w", "-o", tuskdBin, "./cmd/tuskd")
	cmd.Dir = tuskRepo
	cmd.Env = []string{
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
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
	installArgs := os.Args[2:]
	verbose := false
	scriptArgs := make([]string, 0, len(installArgs))

	for _, arg := range installArgs {
		switch arg {
		case "--verbose", "-v":
			verbose = true
		default:
			scriptArgs = append(scriptArgs, arg)
		}
	}

	fmt.Println("Tusk Installer")
	fmt.Println("")
	fmt.Println("This will:")
	fmt.Println("1. Download pre-built Alpine VM with tuskd")
	fmt.Println("2. Start the VM automatically")
	fmt.Println("")
	fmt.Println("If download fails, it will build from scratch.")
	fmt.Println("")

	if verbose {
		fmt.Println("Running with verbose logs enabled")
	}

	scriptPath := filepath.Join(os.Getenv("HOME"), "Tusk", "scripts", "prebuilt-install.sh")
	if verbose {
		fmt.Printf("Executing: bash %s %s\n", scriptPath, strings.Join(scriptArgs, " "))
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, scriptArgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Pre-built download failed, trying build from scratch...\n")
		autoScript := filepath.Join(os.Getenv("HOME"), "Tusk", "scripts", "auto-install.sh")
		if verbose {
			fmt.Printf("Executing: bash %s %s\n", autoScript, strings.Join(scriptArgs, " "))
		}
		cmd = exec.Command("bash", append([]string{autoScript}, scriptArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
	}

	if err := verifyInstallation(verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Install verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Tusk installed and ready.")
}

func verifyInstallation(verbose bool) error {
	mgr := vm.New(tuskDir)
	if verbose {
		fmt.Println("Verifying tuskd socket and RPC readiness...")
	}

	conn, err := mgr.WaitForSerial(20 * time.Second)
	if err != nil {
		return fmt.Errorf("serial socket not available: %w", err)
	}
	if conn != nil {
		if closeErr := conn.Close(); closeErr != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to close serial probe connection: %v\n", closeErr)
		}
	}

	c := client.New(mgr.SerialSocket())
	if err := c.Connect(); err != nil {
		return fmt.Errorf("failed to connect to tuskd: %w", err)
	}
	defer c.Close()

	if err := c.Ping(); err != nil {
		return fmt.Errorf("tuskd ping failed: %w", err)
	}
	if verbose {
		fmt.Println("tuskd is responding over serial RPC")
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

	mgr := vm.New(tuskDir)
	if err := mgr.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init VM manager: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Tusk initialized successfully!")
	fmt.Printf("Data directory: %s\n", tuskDir)
}

func runStart() {
	fmt.Println("Starting Tusk VM...")

	// Check for --simulation flag
	for _, arg := range os.Args[2:] {
		if arg == "--simulation" {
			startSimulation()
			return
		}
	}

	// Check if a usable disk image exists (> 50MB = real install)
	diskPath := filepath.Join(tuskDir, "vm", "disk.qcow2")
	fi, err := os.Stat(diskPath)
	if err != nil || fi.Size() < 50*1024*1024 {
		fmt.Fprintf(os.Stderr, "No bootable VM disk found. Starting in simulation mode.\n")
		fmt.Fprintf(os.Stderr, "Run 'tusk install' first to set up a real VM.\n")
		startSimulation()
		return
	}

	mgr := vm.New(tuskDir)
	ctx := context.Background()
	cfg := &vm.Config{Memory: 512, CPUs: 2, DiskPath: diskPath}

	if err := mgr.Start(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start VM: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back to simulation mode.\n")
		startSimulation()
		return
	}

	fmt.Println("VM started. Waiting for tuskd...")
	conn, err := mgr.WaitForSerial(60 * time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: serial socket not available: %v\n", err)
		fmt.Println("VM started. Connect to serial socket manually to interact with tuskd.")
		return
	}
	conn.Close()

	cli := client.New(mgr.SerialSocket())
	if err := cli.Connect(); err == nil {
		if err := cli.Ping(); err == nil {
			fmt.Println("Tusk VM is ready!")
			cli.Close()
			return
		}
		cli.Close()
	}

	fmt.Println("VM started but tuskd not responding yet.")
	fmt.Println("Run 'tusk status' to check VM status.")
}

// startSimulation launches tuskd-local in simulation socket mode.
func startSimulation() {
	sockPath := filepath.Join(tuskDir, "vm", "serial.sock")
	os.MkdirAll(filepath.Dir(sockPath), 0755)

	// Check if already running
	if _, err := os.Stat(sockPath); err == nil {
		cli := client.New(sockPath)
		cli.SetTimeout(2 * time.Second)
		if err := cli.Connect(); err == nil {
			if err := cli.Ping(); err == nil {
				fmt.Println("Tusk simulation mode already running.")
				cli.Close()
				return
			}
			cli.Close()
		}
		os.Remove(sockPath)
	}

	// Find or build tuskd binary (native ARM for simulation)
	tuskdPath := filepath.Join(tuskDir, "tuskd-local")
	if _, err := os.Stat(tuskdPath); err != nil {
		repo := filepath.Join(os.Getenv("HOME"), "Tusk")
		if _, err := os.Stat(filepath.Join(repo, "cmd", "tuskd")); err == nil {
			fmt.Println("Building tuskd for simulation mode...")
			cmd := exec.Command("go", "build", "-o", tuskdPath, "./cmd/tuskd")
			cmd.Dir = repo
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to build tuskd: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if _, err := os.Stat(tuskdPath); err != nil {
		fmt.Fprintf(os.Stderr, "tuskd binary not found at %s\n", tuskdPath)
		os.Exit(1)
	}

	fmt.Println("Starting tuskd in simulation mode...")
	cmd := exec.Command(tuskdPath, "--socket", sockPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start tuskd: %v\n", err)
		os.Exit(1)
	}

	// Wait for socket
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			cli := client.New(sockPath)
			if err := cli.Connect(); err == nil {
				if err := cli.Ping(); err == nil {
					fmt.Printf("Tusk simulation mode ready! (PID: %d)\n", cmd.Process.Pid)
					cli.Close()
					return
				}
				cli.Close()
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "Simulation mode failed to start.\n")
	os.Exit(1)
}

func runStop() {
	fmt.Println("Stopping Tusk VM...")
	mgr := vm.New(tuskDir)
	_ = mgr.Stop()

	// Also kill simulation tuskd
	exec.Command("pkill", "-f", "tuskd-local").Run()
	exec.Command("pkill", "-f", "tuskd").Run()

	fmt.Println("VM stopped.")
}

func runStatus() {
	mgr := vm.New(tuskDir)
	status := mgr.Status()

	fmt.Printf("VM Status: %s\n", status)
	fmt.Printf("QMP Socket: %s\n", mgr.QMPSocket())
	fmt.Printf("Serial Socket (API): %s\n", mgr.SerialSocket())
	fmt.Printf("Console Socket: %s\n", mgr.ConsoleSocket())

	if mgr.QMPSocketExists() {
		qmp, err := mgr.WaitForQMP(5 * time.Second)
		if err == nil {
			qmp.Close()
			fmt.Println("QMP: Connected")
		}
	}

	// Check simulation mode
	serialSock := mgr.SerialSocket()
	if _, err := os.Stat(serialSock); err == nil {
		cli := client.New(serialSock)
		cli.SetTimeout(2 * time.Second)
		if err := cli.Connect(); err == nil {
			if err := cli.Ping(); err == nil {
				fmt.Println("tuskd: responding (simulation mode)")
			}
			cli.Close()
		}
	}
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
	fmt.Printf("1. Tusk data directory (%s) - Includes VM disks and containers!\n", tuskDir)
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

	mgr := vm.New(tuskDir)
	fmt.Println("Stopping Tusk VM if running...")
	_ = mgr.Stop()

	time.Sleep(1 * time.Second)
	exec.Command("pkill", "-f", "qemu-system-x86_64").Run()
	exec.Command("pkill", "-f", "tuskd").Run()

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

	fmt.Println("Removing Alpine ISO files...")
	files, _ := filepath.Glob(filepath.Join(os.Getenv("HOME"), "alpine-virt-*.iso"))
	for _, f := range files {
		_ = os.Remove(f)
	}

	fmt.Println("\nTusk uninstalled successfully!")
	fmt.Println("Note: If you added '~/tusk' to your PATH manually, you may want to remove it from your .bashrc/.zshrc.")
}

func ensureVM() (*vm.Manager, error) {
	mgr := vm.New(tuskDir)
	deadline := time.Now().Add(2 * time.Minute)
	var lastErr error

	for time.Now().Before(deadline) {
		// Accept either a full QEMU VM (QMP + serial) or a standalone
		// tuskd simulation socket (serial only, no QMP).
		serialOK := false
		if _, err := os.Stat(mgr.SerialSocket()); err == nil {
			serialOK = true
		}
		if !mgr.QMPSocketExists() && !serialOK {
			lastErr = fmt.Errorf("VM not running. Run 'tusk start' first.")
			time.Sleep(200 * time.Millisecond)
			continue
		}

		cli := client.New(mgr.SerialSocket())
		cli.SetTimeout(5 * time.Second)
		if err := cli.Connect(); err != nil {
			lastErr = fmt.Errorf("cannot connect to tuskd: %w", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		pingErr := cli.Ping()
		if pingErr == nil {
			cli.Close()
			return mgr, nil
		}

		lastErr = fmt.Errorf("tuskd not responding: %w", pingErr)
		cli.Close()
		time.Sleep(200 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("VM not running. Run 'tusk start' first.")
	}
	return nil, lastErr
}

func execLookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", file)
	}
	return path, nil
}
