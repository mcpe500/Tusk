package main

import (
	"os"
	"strings"
)

func main() {
	devicePath := deviceArg(os.Args)
	socketPath := socketArg(os.Args)

	// Native host mode (Termux/Android, no VM): run the real proot-backed
	// daemon. There is no simulation mode — containers really execute.
	if _, err := os.Stat("/tusk"); os.IsNotExist(err) && devicePath == "" {
		if socketPath != "" {
			runHostSocket(socketPath)
		} else {
			runHostMode()
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
