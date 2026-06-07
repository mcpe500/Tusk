package main

import "os"

func main() {
	devicePath := deviceArg(os.Args)

	if _, err := os.Stat("/tusk"); os.IsNotExist(err) && devicePath == "" {
		runSimulationMode()
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
