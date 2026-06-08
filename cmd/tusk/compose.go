package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/internal/client"
	"github.com/tusk/tusk/internal/compose"
)

func runCompose() {
	for _, arg := range os.Args[2:] {
		if arg == "-h" || arg == "--help" {
			printComposeUsage()
			return
		}
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage: tusk compose [opts] <command>")
		fmt.Println("  -f <file>   Compose file (default: docker-compose.yml)")
		return
	}

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

	switch args[i] {
	case "up":
		runComposeUp(composeFile)
	case "down":
		runComposeDown(composeFile)
	case "ps":
		runComposePS(composeFile)
	case "build":
		fmt.Println("Compose build not implemented yet")
	case "logs":
		fmt.Println("Compose logs not implemented yet")
	case "rm":
		fmt.Println("Compose rm not implemented yet")
	case "stop":
		fmt.Println("Compose stop not implemented yet")
	default:
		fmt.Printf("Unknown compose command: %s\n", args[i])
	}
}

func printComposeUsage() {
	fmt.Println("Usage: tusk compose [opts] <command>")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -f <file>, --file <file>   Compose file (default: docker-compose.yml)")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  up      Start services")
	fmt.Println("  down    Stop and remove services")
	fmt.Println("  ps      List services")
	fmt.Println("  build   Build images")
	fmt.Println("  logs    View logs")
	fmt.Println("  rm      Remove services")
	fmt.Println("  stop    Stop services")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  tusk compose up")
	fmt.Println("  tusk compose -f docker-compose.yml up")
}

func runComposeUp(composeFile string) {
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: compose file not found: %s\n", composeFile)
		fmt.Println("Use -f <file> to specify a different compose file")
		os.Exit(1)
	}

	fmt.Printf("Parsing compose file: %s\n", composeFile)
	parser := compose.NewParser()
	spec, err := parser.Parse(composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse compose file: %v\n", err)
		os.Exit(1)
	}

	workDir, _ := filepath.Abs(filepath.Dir(composeFile))
	orch := compose.NewOrchestrator(spec, workDir)
	if _, err := ensureVM(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting services...")
	if err := orch.Up(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start services: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("")
	fmt.Println("Services started!")
	fmt.Println("Use 'tusk ps' to see running containers")
}

func runComposeDown(composeFile string) {
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: compose file not found: %s\n", composeFile)
		os.Exit(1)
	}

	fmt.Printf("Parsing compose file: %s\n", composeFile)
	parser := compose.NewParser()
	spec, err := parser.Parse(composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse compose file: %v\n", err)
		os.Exit(1)
	}

	workDir, _ := filepath.Abs(filepath.Dir(composeFile))
	orch := compose.NewOrchestrator(spec, workDir)
	if _, err := ensureVM(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Stopping services...")
	if err := orch.Down(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Also remove containers via RPC
	mgr, _ := ensureVM()
	if mgr != nil {
		cli := client.New(mgr.SerialSocket())
		if err := cli.Connect(); err == nil {
			containers, err := cli.ContainerList(false)
			if err == nil {
				for _, c := range containers {
					cli.ContainerRemove(c.ID, true)
					id := c.ID
					if len(id) > 12 {
						id = id[:12]
					}
					fmt.Printf("  Removed: %s (%s)\n", c.Name, id)
				}
			}
			cli.Close()
		}
	}

	fmt.Println("")
	fmt.Println("Services stopped and removed!")
}

func runComposePS(composeFile string) {
	if _, err := ensureVM(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	mgr, _ := ensureVM()
	if mgr == nil {
		return
	}
	cli := client.New(mgr.SerialSocket())
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()
	containers, err := cli.ContainerList(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(containers) == 0 {
		fmt.Println("No containers found")
		return
	}
	fmt.Printf("%-12s %-20s %-20s %-10s\n", "CONTAINER ID", "NAME", "IMAGE", "STATUS")
	for _, c := range containers {
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Printf("%-12s %-20s %-20s %-10s\n", id, c.Name, c.Image, c.Status)
	}
}
