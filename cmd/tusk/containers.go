package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tusk/tusk/internal/client"
	"github.com/tusk/tusk/pkg/protocol"
)

func runRun() {
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

	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		fmt.Fprintf(os.Stderr, "Is the VM running? Run 'tusk start' first.\n")
		os.Exit(1)
	}
	defer cli.Close()

	if err := cli.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: tuskd not responding: %v\n", err)
		os.Exit(1)
	}

	params := &protocol.ContainerCreateParams{Image: imageName, Name: name, Command: cmdArgs, Env: envVars}
	result, err := cli.ContainerCreate(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create container: %v\n", err)
		os.Exit(1)
	}

	if err := cli.ContainerStart(result.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start container: %v\n", err)
		os.Exit(1)
	}

	if detach {
		fmt.Printf("Container %s started (PID: %d)\n", result.ID[:12], result.Pid)
		return
	}

	time.Sleep(500 * time.Millisecond)
	execResult, _ := cli.ContainerExec(result.ID, cmdArgs)
	if execResult.ExitCode != 0 {
		fmt.Print(execResult.Stderr)
	}
	fmt.Print(execResult.Stdout)
	os.Exit(execResult.ExitCode)
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

	cli := client.New(filepath.Join(tuskDir, "vm", "serial.sock"))
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to VM: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	logs, err := cli.ContainerLogs(os.Args[2])
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

func runContainer(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: tusk container <ls|stop|rm|inspect>")
		return
	}
	switch args[0] {
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
		fmt.Printf("Unknown container command: %s\n", args[0])
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
