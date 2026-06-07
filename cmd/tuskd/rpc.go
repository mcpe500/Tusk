package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/tusk/tusk/pkg/types"
)

func handleRPC(store *ContainerStore, method string, req map[string]interface{}) map[string]interface{} {
	resp := map[string]interface{}{"jsonrpc": "2.0", "id": req["id"]}

	switch method {
	case "Ping":
		resp["result"] = "pong"
	case "Info":
		resp["result"] = map[string]string{"version": "1.0.0", "apiVersion": "v1", "os": "linux", "arch": "x86_64"}
	case "ContainerList":
		resp["result"] = map[string]interface{}{"containers": store.List()}
	case "ContainerCreate":
		handleContainerCreate(store, req, resp)
	case "ContainerStart":
		handleContainerState(store.Start, req, resp, "started")
	case "ContainerStop":
		handleContainerState(store.Stop, req, resp, "stopped")
	case "ContainerExec":
		handleContainerExec(req, resp)
	case "ContainerRemove":
		handleContainerState(store.Remove, req, resp, "removed")
	case "ContainerLogs":
		handleContainerLogs(store, req, resp)
	case "ImagePull":
		resp["result"] = map[string]string{"status": "pulled"}
	case "ImageList":
		resp["result"] = map[string]interface{}{"images": []types.ImageInfo{}}
	case "NetworkCreate":
		handleNetworkCreate(req, resp)
	case "NetworkList":
		resp["result"] = map[string]interface{}{"networks": []types.NetworkInfo{}}
	default:
		resp["error"] = errorObject(-32601, "method not found")
	}

	return resp
}

func handleContainerCreate(store *ContainerStore, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}

	image := strings.TrimSpace(getStringOrDefault(params["image"]))
	if image == "" {
		resp["error"] = errorObject(-32602, "invalid params: image is required")
		return
	}

	c := &types.ContainerInfo{ID: generateID(), Name: getStringOrDefault(params["name"]), Image: image}
	if rawCmd, exists := params["command"]; exists {
		cmd, err := asStringSlice(rawCmd)
		if err != nil {
			resp["error"] = errorObject(-32602, "invalid params: command must be []string")
			return
		}
		c.Cmd = cmd
	}
	if rawEnv, exists := params["env"]; exists {
		env, err := asStringSlice(rawEnv)
		if err != nil {
			resp["error"] = errorObject(-32602, "invalid params: env must be []string")
			return
		}
		c.Env = env
	}

	store.Create(c)
	resp["result"] = c
}

func handleContainerState(change func(string), req, resp map[string]interface{}, status string) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	idStr, ok := params["id"].(string)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	change(idStr)
	resp["result"] = map[string]string{"status": status}
}

func handleContainerExec(req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	containerID, ok := params["containerId"].(string)
	_ = containerID
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params: containerId is required")
		return
	}

	var cmd []string
	if rawCmd, exists := params["command"]; exists {
		cmdParsed, err := asStringSlice(rawCmd)
		if err != nil {
			resp["error"] = errorObject(-32602, "invalid params: command must be []string")
			return
		}
		cmd = cmdParsed
	}

	var stdout, stderr string
	exitCode := 0
	if len(cmd) == 0 {
		exitCode = 1
		stderr = "No command specified"
	} else {
		execCmd := exec.Command(cmd[0], cmd[1:]...)
		output, err := execCmd.CombinedOutput()
		if err != nil {
			exitCode = 1
			stderr = string(output)
		} else {
			stdout = string(output)
		}
	}

	resp["result"] = map[string]interface{}{"exitCode": exitCode, "stdout": stdout, "stderr": stderr}
}

func handleContainerLogs(store *ContainerStore, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	idStr, ok := params["id"].(string)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	resp["result"] = map[string]string{"logs": store.Logs(idStr)}
}

func handleNetworkCreate(req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	name := getStringOrDefault(params["name"])
	if strings.TrimSpace(name) == "" {
		resp["error"] = errorObject(-32602, "invalid params: name is required")
		return
	}
	resp["result"] = map[string]string{"id": generateID(), "name": name}
}

func writeJSONRPCError(enc *json.Encoder, id interface{}, code int, message string) {
	enc.Encode(map[string]interface{}{"jsonrpc": "2.0", "id": id, "error": errorObject(code, message)})
}

func errorObject(code int, message string) map[string]interface{} {
	return map[string]interface{}{"code": code, "message": message}
}
