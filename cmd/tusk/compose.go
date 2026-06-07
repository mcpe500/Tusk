package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
