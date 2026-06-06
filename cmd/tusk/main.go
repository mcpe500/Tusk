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
	"github.com/tusk/tusk/internal/compose"
	"github.com/tusk/tusk/internal/image"
	"github.com/tusk/tusk/internal/vm"
	"github.com/tusk/tusk/pkg/protocol"
)

var (
	tuskDir = filepath.Join(os.Getenv("HOME"), ".tusk")
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "version":
		runVersion()
	case "update":
		runUpdate()
	case "install":
		runInstall()
	case "init":
		runInit()
	case "start":
		runStart()
	case "stop":
		runStop()
	case "status":
		runStatus()
	case "pull":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tusk pull <image>")
			return
		}
		runPull(os.Args[2])
	case "images":
		runImages()
	case "run":
		runRun()
	case "ps":
		runPS()
	case "rm":
		runRMTop()
	case "exec":
		runExec()
	case "logs":
		runLogs()
	case "container":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tusk container <command>")
			return
		}
		runContainer(os.Args[2:])
	case "network":
		runNetwork()
	case "volume":
		runVolume()
	case "compose":
		runCompose()
	default:
		printUsage()
	}
}

func runVersion() {
	fmt.Println("tusk version 0.1.0")
	fmt.Println("Tusk: Hardware emulation for Termux, because sometimes working is better than fast.")
}

func runUpdate() {
	fmt.Println("Updating Tusk...")

	tuskBin := filepath.Join(os.Getenv("HOME"), "tusk")

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: git not found. Cannot update.\n")
		os.Exit(1)
	}

	// Get Tusk directory
	tuskDir := filepath.Join(os.Getenv("HOME"), "Tusk")

	// Pull latest from git
	fmt.Println("Pulling latest from GitHub...")
	cmd := exec.Command("git", "-C", tuskDir, "pull", "origin", "main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to pull updates: %v\n", err)
		os.Exit(1)
	}

	// Rebuild tusk binary
	fmt.Println("Building tusk...")
	cmd = exec.Command("go", "build", "-o", tuskBin, "./cmd/tusk")
	cmd.Dir = tuskDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to build tusk: %v\n", err)
		os.Exit(1)
	}

	// Rebuild tuskd for VM
	fmt.Println("Building tuskd (x86_64)...")
	tuskdBin := filepath.Join(os.Getenv("HOME"), ".tusk", "tuskd-amd64")
	cmd = exec.Command("go", "build", "-ldflags=-s -w", "-o", tuskdBin, "./cmd/tuskd")
	cmd.Dir = tuskDir
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

	// Run the prebuilt-install script
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
		// Fallback to auto-install
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

	client := client.New(mgr.SerialSocket())
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to tuskd: %w", err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		return fmt.Errorf("tuskd ping failed: %w", err)
	}

	if verbose {
		fmt.Println("tuskd is responding over serial RPC")
	}

	return nil
}

func printUsage() {
	fmt.Println(`Tusk: Hardware emulation for Termux, because sometimes working is better than fast.

Usage:
  tusk version           Show version
  tusk update            Update Tusk to latest
   tusk install [--verbose]  Download pre-built VM and start
  tusk init              Initialize Tusk storage
  tusk start             Start the Tusk VM
  tusk stop              Stop the Tusk VM
  tusk status            Show VM status

  tusk pull <image>      Pull image from registry
  tusk images            List local images

  tusk run [opts] <image>   Run a container
  tusk ps                List running containers
  tusk exec <id> <cmd>   Execute command in container
  tusk logs <id>         View container logs
  tusk stop <id>         Stop container
  tusk rm <id>           Remove container

  tusk network ls        List networks
  tusk volume ls         List volumes

  tusk compose up        Start compose services
  tusk compose down      Stop compose services
  tusk compose ps        List compose services

Examples:
  tusk init
  tusk pull alpine:latest
  tusk run alpine echo hello
  tusk compose -f docker-compose.yml up`)
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

	mgr := vm.New(tuskDir)
	ctx := context.Background()

	cfg := &vm.Config{
		Memory: 512,
		CPUs:   2,
	}

	if err := mgr.Start(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start VM: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("VM started. Waiting for tuskd...")

	// Wait for serial socket with timeout
	conn, err := mgr.WaitForSerial(60 * time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: serial socket not available: %v\n", err)
		fmt.Println("VM started. Connect to serial socket manually to interact with tuskd.")
		return
	}
	conn.Close()

	// Try to ping tuskd
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

func runStop() {
	fmt.Println("Stopping Tusk VM...")
	mgr := vm.New(tuskDir)
	if err := mgr.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop VM: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("VM stopped.")
}

func runStatus() {
	mgr := vm.New(tuskDir)
	status := mgr.Status()

	fmt.Printf("VM Status: %s\n", status)
	fmt.Printf("QMP Socket: %s\n", mgr.QMPSocket())
	fmt.Printf("Serial Socket: %s\n", mgr.SerialSocket())

	if mgr.QMPSocketExists() {
		qmp, err := mgr.WaitForQMP(5 * 1e9)
		if err == nil {
			qmp.Close()
			fmt.Println("QMP: Connected")
		}
	}
}

func runPull(ref string) {
	fmt.Printf("Pulling %s...\n", ref)

	store := image.New(filepath.Join(tuskDir, "images"))
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init image store: %v\n", err)
		os.Exit(1)
	}

	puller := image.NewPuller(store)
	ctx := context.Background()

	if err := puller.Pull(ctx, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Pull failed: %v\n", err)
		os.Exit(1)
	}
}

func runImages() {
	// List blobs to find images
	blobsDir := filepath.Join(tuskDir, "images", "blobs", "sha256")
	entries, err := os.ReadDir(blobsDir)
	if err != nil {
		fmt.Println("No images found")
		return
	}

	// Count blobs (each blob is a layer)
	blobCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			blobCount++
		}
	}

	if blobCount == 0 {
		fmt.Println("No images found")
		return
	}

	// List manifests to find image names
	manifestsDir := filepath.Join(tuskDir, "images", "manifests")
	manifestEntries, _ := os.ReadDir(manifestsDir)
	indexDir := filepath.Join(tuskDir, "images", "index")

	fmt.Println("REPOSITORY   TAG      DIGEST                                   SIZE")
	for _, entry := range manifestEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			digest := strings.TrimSuffix(entry.Name(), ".json")
			// Check if there's a tag in index
			tag := "latest"
			if indexDir != "" {
				// TODO: lookup tag from index
				_ = indexDir
			}
			shortDigest := digest
			if len(digest) > 16 {
				shortDigest = digest[:16]
			}
			fmt.Printf("%-12s %-8s %s...\n", "local", tag, shortDigest)
		}
	}

	fmt.Printf("\nTotal: %d blobs stored\n", blobCount)
}

func runRun() {
	// Parse arguments
	var imageName string
	var cmdArgs []string
	var detach bool
	var name string
	var envVars []string

	args := os.Args[2:]
	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "-d", "--detach":
			detach = true
			i++
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: --name requires a value\n")
				os.Exit(1)
			}
			name = args[i+1]
			i += 2
		case "-e", "--env":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: -e/--env requires a value\n")
				os.Exit(1)
			}
			envVars = append(envVars, args[i+1])
			i += 2
		case "-i", "-t", "--interactive", "--tty":
			i++
		case "-v", "--volume", "-p", "--publish":
			// Skip for now - not implemented
			i += 2
		default:
			if !strings.HasPrefix(arg, "-") {
				imageName = arg
				cmdArgs = args[i+1:]
				break
			}
			i++
		}
	}

	if imageName == "" {
		fmt.Fprintf(os.Stderr, "Error: image name required\n")
		fmt.Fprintf(os.Stderr, "Usage: tusk run [opts] <image> [command...]\n")
		os.Exit(1)
	}

	// Connect to tuskd
	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		fmt.Fprintf(os.Stderr, "Is the VM running? Run 'tusk start' first.\n")
		os.Exit(1)
	}
	defer cli.Close()

	// Verify tuskd is ready
	if err := cli.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: tuskd not responding: %v\n", err)
		os.Exit(1)
	}

	// Create container
	params := &protocol.ContainerCreateParams{
		Image:   imageName,
		Name:    name,
		Command: cmdArgs,
		Env:     envVars,
	}

	result, err := cli.ContainerCreate(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create container: %v\n", err)
		os.Exit(1)
	}

	// Start container
	if err := cli.ContainerStart(result.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start container: %v\n", err)
		os.Exit(1)
	}

	if detach {
		fmt.Printf("Container %s started (PID: %d)\n", result.ID[:12], result.Pid)
	} else {
		// Wait for container to finish and get output
		time.Sleep(500 * time.Millisecond)
		execResult, _ := cli.ContainerExec(result.ID, cmdArgs)
		if execResult.ExitCode != 0 {
			fmt.Print(execResult.Stderr)
		}
		fmt.Print(execResult.Stdout)
		os.Exit(execResult.ExitCode)
	}
}

func runPS() {
	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		fmt.Fprintf(os.Stderr, "Is the VM running? Run 'tusk start' first.\n")
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list containers: %v\n", err)
		return
	}

	if len(containers) == 0 {
		fmt.Println("No containers found")
		return
	}

	fmt.Printf("%-12s %-20s %-15s %-10s\n", "CONTAINER ID", "NAME", "IMAGE", "STATUS")
	for _, c := range containers {
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Printf("%-12s %-20s %-15s %-10s\n", id, c.Name, c.Image, c.Status)
	}
}

func runExec() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: tusk exec <container-id> <command...>\n")
		os.Exit(1)
	}

	containerID := os.Args[2]
	cmd := os.Args[3:]

	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	result, err := cli.ContainerExec(containerID, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to exec: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(result.Stdout)
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}
	os.Exit(result.ExitCode)
}

func runLogs() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: tusk logs <container-id>\n")
		os.Exit(1)
	}

	containerID := os.Args[2]

	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	logs, err := cli.ContainerLogs(containerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(logs)
}

func runRMTop() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: tusk rm <container-id>\n")
		os.Exit(1)
	}
	runRM(os.Args[2])
}

func runRM(id string) {
	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	if err := cli.ContainerRemove(id, false); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to remove container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container %s removed\n", id)
}

func runCompose() {
	// Check for help flag
	for _, arg := range os.Args[2:] {
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: tusk compose [opts] <command>")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  -f <file>, --file <file>   Compose file (default: docker-compose.yml)")
			fmt.Println("")
			fmt.Println("Commands:")
			fmt.Println("  up      Start services")
			fmt.Println("  down    Stop services")
			fmt.Println("  ps      List services")
			fmt.Println("  build   Build images")
			fmt.Println("  logs    View logs")
			fmt.Println("  rm      Remove services")
			fmt.Println("  stop    Stop services")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  tusk compose up")
			fmt.Println("  tusk compose -f docker-compose.yml up")
			return
		}
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage: tusk compose [opts] <command>")
		fmt.Println("  -f <file>   Compose file (default: docker-compose.yml)")
		return
	}

	// Parse flags
	var composeFile string
	args := os.Args[2:]
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if args[i] == "-f" || args[i] == "--file" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: -f requires a file argument\n")
				os.Exit(1)
			}
			composeFile = args[i+1]
			i += 2
		} else {
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if i >= len(args) {
		fmt.Println("Usage: tusk compose [opts] <command>")
		fmt.Println("Commands: up, down, ps, build, logs, rm, stop")
		return
	}

	subcmd := args[i]
	switch subcmd {
	case "up":
		runComposeUp(composeFile)
	case "down":
		fmt.Println("Compose down not implemented yet")
	case "ps":
		fmt.Println("Compose ps not implemented yet")
	case "build":
		fmt.Println("Compose build not implemented yet")
	case "logs":
		fmt.Println("Compose logs not implemented yet")
	case "rm":
		fmt.Println("Compose rm not implemented yet")
	case "stop":
		fmt.Println("Compose stop not implemented yet")
	default:
		fmt.Printf("Unknown compose command: %s\n", subcmd)
	}
}

func runContainer(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: tusk container <ls|stop|rm|inspect>")
		return
	}
	sub := args[0]
	switch sub {
	case "ls":
		runPS()
	case "stop":
		if len(args) < 2 {
			fmt.Println("Usage: tusk container stop <id>")
			return
		}
		runContainerStop(args[1])
	case "rm":
		if len(args) < 2 {
			fmt.Println("Usage: tusk container rm <id>")
			return
		}
		runRM(args[1])
	case "inspect":
		fmt.Println("Container inspect not implemented yet")
	default:
		fmt.Printf("Unknown container command: %s\n", sub)
	}
}

func runContainerStop(id string) {
	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	if err := cli.ContainerStop(id); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to stop container: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container %s stopped\n", id)
}

func runComposeUp(composeFile string) {
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	// Check if file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: compose file not found: %s\n", composeFile)
		fmt.Println("Use -f <file> to specify a different compose file")
		os.Exit(1)
	}

	fmt.Printf("Parsing compose file: %s\n", composeFile)

	// Parse compose file
	parser := compose.NewParser()
	spec, err := parser.Parse(composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse compose file: %v\n", err)
		os.Exit(1)
	}

	// Get work directory
	workDir, _ := filepath.Abs(filepath.Dir(composeFile))

	// Create orchestrator
	orch := compose.NewOrchestrator(spec, workDir)

	fmt.Println("Starting services...")

	// Start services
	if err := orch.Up(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start services: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("")
	fmt.Println("Services started!")
	fmt.Println("Use 'tusk ps' to see running containers")
}

func runNetwork() {
	fmt.Println("Network management not implemented yet")
}

func runVolume() {
	fmt.Println("Volume management not implemented yet")
}

// Ensure VM is running
func ensureVM() (*vm.Manager, error) {
	mgr := vm.New(tuskDir)
	if !mgr.QMPSocketExists() {
		return nil, fmt.Errorf("VM not running. Run 'tusk start' first.")
	}

	client := client.New(mgr.SerialSocket())
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("cannot connect to tuskd: %w", err)
	}

	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("tuskd not responding: %w", err)
	}

	return mgr, nil
}

func execLookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", file)
	}
	return path, nil
}
