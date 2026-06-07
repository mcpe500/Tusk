package main

import "fmt"

func printUsage() {
	fmt.Println(`Tusk: Hardware emulation for Termux, because sometimes working is better than fast.

Usage:
  tusk version           Show version
  tusk update            Update Tusk to latest
  tusk install [--verbose]  Download pre-built VM and start
  tusk ls [images|ps]    Alias for images or ps
  tusk init              Initialize Tusk storage
  tusk start             Start the Tusk VM
  tusk stop              Stop the Tusk VM
  tusk status            Show VM status
  tusk uninstall         Uninstall Tusk and delete all data

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

  tusk rpc <method> [params-json]  Send raw JSON-RPC request (debug)

Examples:
  tusk init
  tusk pull alpine:latest
  tusk run alpine echo hello
  tusk compose -f docker-compose.yml up`)
}
