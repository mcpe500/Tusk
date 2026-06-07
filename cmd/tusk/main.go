package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var tuskDir = filepath.Join(os.Getenv("HOME"), ".tusk")

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "version":
		runVersion()
	case "ls":
		runLS()
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
	case "uninstall":
		runUninstall()
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
	case "rpc":
		runRPC()
	default:
		printUsage()
	}
}

func runLS() {
	if len(os.Args) >= 3 {
		switch os.Args[2] {
		case "ps":
			runPS()
		case "images":
			runImages()
		default:
			fmt.Println("Usage: tusk ls [images|ps]")
		}
		return
	}
	runImages()
}
