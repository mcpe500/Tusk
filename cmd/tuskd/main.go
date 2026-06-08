package main

import (
	"os"
	"strings"
)

func main() {
	devicePath := deviceArg(os.Args)
	socketPath := socketArg(os.Args)

	// Explicit simulation mode via flag or env
	if os.Getenv("TUSK_SIMULATION") == "1" || hasFlag(os.Args, "--simulation") {
		if socketPath != "" {
			runSimulationSocket(socketPath)
		} else {
			runSimulationMode()
		}
		return
	}

	// Running outside VM — auto-detect
	if _, err := os.Stat("/tusk"); os.IsNotExist(err) && devicePath == "" {
		if socketPath != "" {
			runSimulationSocket(socketPath)
		} else {
			runSimulationMode()
		}
		return
	}

	if devicePath != "" {
		runDeviceDaemon(devicePath)
		return
	}
	runDaemon()
}

func deviceArg(args []string) string {
	for i, arg := range args {
		if arg == "--device" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func socketArg(args []string) string {
	for i, arg := range args {
		if arg == "--socket" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}
