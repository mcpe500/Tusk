package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tusk/tusk/internal/container"
	"github.com/tusk/tusk/pkg/types"
)

// handleRPC dispatches a JSON-RPC method against the container store and the
// real proot-backed runtime.
func handleRPC(store *ContainerStore, rt *Runtime, method string, req map[string]interface{}) map[string]interface{} {
	resp := map[string]interface{}{"jsonrpc": "2.0", "id": req["id"]}

	switch method {
	case "Ping":
		resp["result"] = "pong"
	case "Info":
		resp["result"] = map[string]string{"version": "1.0.0", "apiVersion": "v1", "os": "linux", "arch": "arm64", "runtime": "proot"}
	case "ContainerList":
		resp["result"] = map[string]interface{}{"containers": toProtocolList(store.List())}
	case "ContainerCreate":
		handleContainerCreate(store, rt, req, resp)
	case "ContainerStart":
		handleContainerStart(store, rt, req, resp)
	case "ContainerStop":
		handleContainerStop(store, rt, req, resp)
	case "ContainerExec":
		handleContainerExec(rt, req, resp)
	case "ContainerRemove":
		handleContainerRemove(store, rt, req, resp)
	case "ContainerInspect":
		handleContainerInspect(store, req, resp)
	case "ContainerLogs":
		handleContainerLogs(store, rt, req, resp)
	case "ImagePull":
		handleImagePull(rt, req, resp)
	case "ImageList":
		handleImageList(rt, resp)
	case "NetworkCreate":
		handleNetworkCreate(req, resp)
	case "NetworkList":
		resp["result"] = map[string]interface{}{"networks": []types.NetworkInfo{}}
	default:
		resp["error"] = errorObject(-32601, "method not found")
	}

	return resp
}

// toProtocolList converts internal ContainerInfo (json "state") into the wire
// shape the client expects (json "status"), fixing the blank-STATUS bug.
func toProtocolList(in []types.ContainerInfo) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(in))
	for _, c := range in {
		out = append(out, map[string]interface{}{
			"id":        c.ID,
			"name":      c.Name,
			"image":     c.Image,
			"status":    string(c.State),
			"created":   c.Created.Format("2006-01-02T15:04:05Z07:00"),
			"ipAddress": c.IPAddress,
		})
	}
	return out
}

func handleContainerCreate(store *ContainerStore, rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}

	imageRef := strings.TrimSpace(getStringOrDefault(params["image"]))
	if imageRef == "" {
		resp["error"] = errorObject(-32602, "invalid params: image is required")
		return
	}

	c := &types.ContainerInfo{ID: generateID(), Name: getStringOrDefault(params["name"]), Image: imageRef, State: types.StatusCreated}
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
	if rawPorts, exists := params["ports"]; exists {
		c.Ports = parsePortMappings(rawPorts)
	}
	if rawLabels, exists := params["labels"]; exists {
		if m, ok := rawLabels.(map[string]interface{}); ok {
			c.Labels = make(map[string]string, len(m))
			for k, v := range m {
				c.Labels[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	if rawMounts, exists := params["mounts"]; exists {
		c.Mounts = parseMountParams(rawMounts)
	}

	// Extract the rootfs now so creation fails early if the image is missing.
	if _, err := rt.PrepareRootfs(c.ID, imageRef); err != nil {
		resp["error"] = errorObject(-32603, "prepare rootfs: "+err.Error())
		return
	}

	store.Create(c)
	resp["result"] = map[string]interface{}{"id": c.ID, "name": c.Name, "pid": 0, "ipAddress": ""}
}

func handleContainerStart(store *ContainerStore, rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	id, ok := params["id"].(string)
	if !ok || id == "" {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	c, err := store.Get(id)
	if err != nil {
		resp["error"] = errorObject(-32602, "no such container: "+id)
		return
	}

	pid, err := rt.Start(c.ID, c.Image, c.Cmd, c.Env, mountsFromInfo(c.Mounts))
	if err != nil {
		resp["error"] = errorObject(-32603, "start: "+err.Error())
		return
	}
	store.SetState(c.ID, types.StatusRunning, pid)
	resp["result"] = map[string]interface{}{"status": "started", "pid": pid}
}

func handleContainerStop(store *ContainerStore, rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	id, ok := params["id"].(string)
	if !ok || id == "" {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	_ = rt.Stop(id)
	store.SetState(id, types.StatusStopped, 0)
	resp["result"] = map[string]string{"status": "stopped"}
}

func handleContainerRemove(store *ContainerStore, rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	id, ok := params["id"].(string)
	if !ok || id == "" {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	_ = rt.Remove(id)
	store.Remove(id)
	resp["result"] = map[string]string{"status": "removed"}
}

func handleContainerInspect(store *ContainerStore, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	id, ok := params["id"].(string)
	if !ok || id == "" {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	c, err := store.FindByNameOrID(id)
	if err != nil {
		resp["error"] = errorObject(-32602, err.Error())
		return
	}
	resp["result"] = c
}

func handleContainerExec(rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	containerID, ok := params["containerId"].(string)
	if !ok || containerID == "" {
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
	if len(cmd) == 0 {
		resp["result"] = map[string]interface{}{"exitCode": 1, "stdout": "", "stderr": "No command specified"}
		return
	}

	stdout, stderr, exitCode := rt.Exec(containerID, cmd)
	resp["result"] = map[string]interface{}{"exitCode": exitCode, "stdout": stdout, "stderr": stderr}
}

func handleContainerLogs(store *ContainerStore, rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	id, ok := params["id"].(string)
	if !ok || id == "" {
		resp["error"] = errorObject(-32602, "invalid params: id is required")
		return
	}
	resp["result"] = map[string]string{"logs": rt.Logs(id)}
}

func handleImagePull(rt *Runtime, req, resp map[string]interface{}) {
	params, ok := reqParams(req)
	if !ok {
		resp["error"] = errorObject(-32602, "invalid params")
		return
	}
	ref := strings.TrimSpace(getStringOrDefault(params["reference"]))
	if ref == "" {
		resp["error"] = errorObject(-32602, "invalid params: reference is required")
		return
	}
	if err := rt.Pull(ref); err != nil {
		resp["error"] = errorObject(-32603, "pull: "+err.Error())
		return
	}
	resp["result"] = map[string]string{"status": "pulled"}
}

func handleImageList(rt *Runtime, resp map[string]interface{}) {
	resp["result"] = map[string]interface{}{"images": rt.ImageList()}
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

// parsePortMappings parses compose/CLI "host:container[/proto]" entries.
func parsePortMappings(raw interface{}) []types.PortMapping {
	list, err := asStringSlice(raw)
	if err != nil {
		return nil
	}
	var out []types.PortMapping
	for _, p := range list {
		proto := "tcp"
		spec := p
		if i := strings.IndexByte(spec, '/'); i >= 0 {
			proto = spec[i+1:]
			spec = spec[:i]
		}
		parts := strings.Split(spec, ":")
		var host, target int
		switch len(parts) {
		case 2:
			host = atoiSafe(parts[0])
			target = atoiSafe(parts[1])
		case 1:
			target = atoiSafe(parts[0])
			host = target
		default:
			continue
		}
		out = append(out, types.PortMapping{Protocol: proto, HostPort: host, TargetPort: target})
	}
	return out
}

// parseMountParams converts the raw JSON mounts array from ContainerCreateParams
// into types.Mount entries. Entries with missing source/destination are skipped.
// Named volumes (non-absolute source) should already be resolved to absolute paths
// by the compose orchestrator or CLI before reaching here.
func parseMountParams(raw interface{}) []types.Mount {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var out []types.Mount
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		src := fmt.Sprintf("%v", m["source"])
		dst := fmt.Sprintf("%v", m["destination"])
		if src == "" || src == "<nil>" || dst == "" || dst == "<nil>" {
			continue
		}
		// Auto-create host directory for named volumes (absolute path expected).
		if filepath.IsAbs(src) {
			_ = os.MkdirAll(src, 0755)
		}
		mountType := "bind"
		if t, ok := m["type"].(string); ok && t != "" {
			mountType = t
		}
		out = append(out, types.Mount{Source: src, Destination: dst, Type: mountType})
	}
	return out
}

// mountsFromInfo converts stored types.Mount entries into container.BindMount
// for the proot backend.
func mountsFromInfo(mounts []types.Mount) []container.BindMount {
	if len(mounts) == 0 {
		return nil
	}
	out := make([]container.BindMount, 0, len(mounts))
	for _, m := range mounts {
		if m.Source == "" || m.Destination == "" {
			continue
		}
		out = append(out, container.BindMount{Source: m.Source, Destination: m.Destination})
	}
	return out
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func writeJSONRPCError(enc *json.Encoder, id interface{}, code int, message string) {
	enc.Encode(map[string]interface{}{"jsonrpc": "2.0", "id": id, "error": errorObject(code, message)})
}

func errorObject(code int, message string) map[string]interface{} {
	return map[string]interface{}{"code": code, "message": message}
}
