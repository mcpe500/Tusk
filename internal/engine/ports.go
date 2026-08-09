package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/mcpe500/Tusk/internal/state"
)

const PortStateSchemaVersion = 1

type PublishedPort struct {
	ID            string `json:"id"`
	ContainerID   string `json:"containerId"`
	HostIP        string `json:"hostIP"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	State         string `json:"state"`
	Error         string `json:"error,omitempty"`
}

type PortState struct {
	SchemaVersion int             `json:"schemaVersion"`
	BackendID     string          `json:"backendId"`
	Ports         []PublishedPort `json:"ports"`
	Error         string          `json:"error,omitempty"`
}

type PublishedPortSource interface {
	PublishedTCP(context.Context) ([]PublishedPort, error)
}

type AdapterPortSource struct {
	Adapter Adapter
}

func (source AdapterPortSource) PublishedTCP(ctx context.Context) ([]PublishedPort, error) {
	if source.Adapter == nil {
		return nil, errors.New("Podman port source adapter is required")
	}
	containers, err := source.Adapter.Execute(ctx, Command{Args: []string{"podman", "ps", "-q"}})
	if err != nil || containers.ExitCode != 0 {
		return nil, fmt.Errorf("list running containers: %v", err)
	}
	var published []PublishedPort
	for _, containerID := range strings.Fields(containers.Stdout) {
		ports, err := source.Adapter.Execute(ctx, Command{Args: []string{"podman", "port", containerID}})
		if err != nil || ports.ExitCode != 0 {
			return nil, fmt.Errorf("read ports for %s: %v", containerID, err)
		}
		for _, line := range strings.Split(ports.Stdout, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 3 || fields[len(fields)-2] != "->" {
				return nil, fmt.Errorf("unsupported Podman port output %q", line)
			}
			containerPort := strings.TrimSuffix(fields[0], "/tcp")
			host, hostPort, err := net.SplitHostPort(fields[len(fields)-1])
			if err != nil {
				return nil, fmt.Errorf("parse Podman host port %q: %w", fields[len(fields)-1], err)
			}
			publishedValue := host + ":" + hostPort + ":" + containerPort
			if strings.Contains(host, ":") {
				publishedValue = "[" + host + "]:" + hostPort + ":" + containerPort
			}
			port, err := ParsePublishedPort(publishedValue, containerID+":"+containerPort)
			if err != nil {
				return nil, err
			}
			published = append(published, port)
		}
	}
	return published, nil
}

type PortForwarder interface {
	ForwardTCP(context.Context, PublishedPort) error
	RemoveTCP(context.Context, PublishedPort) error
}

type PortReconciler struct {
	Path      string
	BackendID string
	Forwarder PortForwarder
}

func ParsePublishedPort(value, containerID string) (PublishedPort, error) {
	var parts []string
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]:")
		if end < 0 {
			return PublishedPort{}, fmt.Errorf("invalid published port %q", value)
		}
		parts = []string{value[1:end]}
		parts = append(parts, strings.Split(value[end+2:], ":")...)
	} else {
		parts = strings.Split(value, ":")
	}
	if len(parts) != 2 && len(parts) != 3 {
		return PublishedPort{}, fmt.Errorf("invalid published port %q", value)
	}
	hostIP := "127.0.0.1"
	hostPart := parts[0]
	containerPart := parts[1]
	if len(parts) == 3 {
		hostIP, hostPart, containerPart = parts[0], parts[1], parts[2]
	}
	if net.ParseIP(hostIP) == nil {
		return PublishedPort{}, fmt.Errorf("invalid published host address %q", hostIP)
	}
	hostPort, err := strconv.Atoi(hostPart)
	if err != nil || hostPort < 1 || hostPort > 65535 {
		return PublishedPort{}, fmt.Errorf("invalid host port %q", hostPart)
	}
	containerPort, err := strconv.Atoi(containerPart)
	if err != nil || containerPort < 1 || containerPort > 65535 {
		return PublishedPort{}, fmt.Errorf("invalid container port %q", containerPart)
	}
	if hostIP != "127.0.0.1" && hostIP != "::1" {
		return PublishedPort{}, fmt.Errorf("non-loopback publishing requires explicit security opt-in: %s", hostIP)
	}
	return PublishedPort{ID: containerID + ":" + hostPart + ":" + containerPart, ContainerID: containerID, HostIP: hostIP, HostPort: hostPort, ContainerPort: containerPort, Protocol: "tcp", State: "pending"}, nil
}

func (reconciler PortReconciler) Load() (PortState, error) {
	var stateValue PortState
	if err := state.ReadJSON(reconciler.Path, &stateValue); err != nil {
		return PortState{SchemaVersion: PortStateSchemaVersion, BackendID: reconciler.BackendID}, err
	}
	if stateValue.SchemaVersion != PortStateSchemaVersion || stateValue.BackendID != reconciler.BackendID {
		return PortState{}, errors.New("invalid port state schema or backend")
	}
	return stateValue, nil
}

func (reconciler PortReconciler) Add(ctx context.Context, port PublishedPort) error {
	if reconciler.Forwarder == nil {
		return errors.New("no adapter TCP forwarder is available; refusing to claim published port")
	}
	if port.Protocol != "tcp" {
		return errors.New("only TCP publishing is supported")
	}
	stateValue, err := reconciler.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stateValue.SchemaVersion, stateValue.BackendID = PortStateSchemaVersion, reconciler.BackendID
	for _, existing := range stateValue.Ports {
		if existing.ID == port.ID && existing.State == "ready" {
			return nil
		}
	}
	if err := hostPortAvailable(port.HostIP, port.HostPort); err != nil {
		port.State = "degraded"
		port.Error = err.Error()
		stateValue.Ports = upsertPort(stateValue.Ports, port)
		stateValue.Error = err.Error()
		if writeErr := state.WriteJSON(reconciler.Path, stateValue, 0o600); writeErr != nil {
			return fmt.Errorf("backend %s TCP publish %s:%d collision (%v); persist degraded state: %w", reconciler.BackendID, port.HostIP, port.HostPort, err, writeErr)
		}
		return fmt.Errorf("backend %s TCP publish %s:%d/tcp collision; conflicting listener owner unavailable: %w", reconciler.BackendID, port.HostIP, port.HostPort, err)
	}
	if err := reconciler.Forwarder.ForwardTCP(ctx, port); err != nil {
		port.State = "degraded"
		port.Error = err.Error()
		stateValue.Ports = upsertPort(stateValue.Ports, port)
		stateValue.Error = err.Error()
		if writeErr := state.WriteJSON(reconciler.Path, stateValue, 0o600); writeErr != nil {
			return fmt.Errorf("forward TCP: %v; persist degraded state: %w", err, writeErr)
		}
		return err
	}
	port.State = "ready"
	stateValue.Ports = upsertPort(stateValue.Ports, port)
	stateValue.Error = ""
	return state.WriteJSON(reconciler.Path, stateValue, 0o600)
}

func upsertPort(ports []PublishedPort, replacement PublishedPort) []PublishedPort {
	for index, port := range ports {
		if port.ID == replacement.ID {
			ports[index] = replacement
			return ports
		}
	}
	return append(ports, replacement)
}

func (reconciler PortReconciler) Reconcile(ctx context.Context, source PublishedPortSource) error {
	if source == nil {
		return errors.New("running container port source is required")
	}
	running, err := source.PublishedTCP(ctx)
	if err != nil {
		return fmt.Errorf("read Podman published ports: %w", err)
	}
	current, err := reconciler.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	active := make(map[string]PublishedPort, len(running))
	for _, port := range running {
		active[port.ID] = port
	}
	for _, existing := range current.Ports {
		if _, ok := active[existing.ID]; !ok {
			if err := reconciler.Remove(ctx, existing.ID); err != nil {
				return err
			}
		}
	}
	for _, port := range running {
		if err := reconciler.Add(ctx, port); err != nil {
			return err
		}
	}
	return nil
}

func (reconciler PortReconciler) Remove(ctx context.Context, id string) error {
	stateValue, err := reconciler.Load()
	if err != nil {
		return err
	}
	filtered := stateValue.Ports[:0]
	for _, port := range stateValue.Ports {
		if port.ID == id {
			if reconciler.Forwarder != nil {
				if err := reconciler.Forwarder.RemoveTCP(ctx, port); err != nil {
					return err
				}
			}
			continue
		}
		filtered = append(filtered, port)
	}
	stateValue.Ports = filtered
	return state.WriteJSON(reconciler.Path, stateValue, 0o600)
}

func hostPortAvailable(hostIP string, port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(hostIP, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("host port %s:%d is unavailable: %w", hostIP, port, err)
	}
	return listener.Close()
}
