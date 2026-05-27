package network

import (
	"fmt"
	"net"
	"time"
)

type Manager struct {
	bridgePrefix string
	subnet       string
}

func New() *Manager {
	return &Manager{
		bridgePrefix: "tusk",
		subnet:       "10.0.0.0/24",
	}
}

type Network struct {
	ID     string
	Name   string
	Driver string
	Subnet string
}

func (m *Manager) Create(name, driver string) (*Network, error) {
	if driver == "" {
		driver = "bridge"
	}

	net := &Network{
		ID:     generateID(),
		Name:   name,
		Driver: driver,
		Subnet: m.subnet,
	}

	fmt.Printf("Creating network: %s (driver: %s, subnet: %s)\n", name, driver, net.Subnet)
	return net, nil
}

func (m *Manager) List() []*Network {
	return []*Network{
		{ID: "1", Name: "bridge", Driver: "bridge", Subnet: "10.0.0.0/24"},
	}
}

func (m *Manager) Remove(name string) error {
	fmt.Printf("Removing network: %s\n", name)
	return nil
}

func (m *Manager) AllocateIP() string {
	// Simple: return next IP in range
	return "10.0.0.2"
}

func (m *Manager) PortForward(hostPort, containerPort int, protocol string) error {
	fmt.Printf("Port forwarding: host:%d -> container:%d (%s)\n", hostPort, containerPort, protocol)
	return nil
}

func generateID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

type IPAM struct {
	Subnet    string
	Gateway   string
	IPRange   string
	DNS       []string
}

func (m *Manager) RequestAddress(netID, subnet string) (string, string, error) {
	// Allocate IP from subnet
	ip, err := m.nextIP(subnet)
	if err != nil {
		return "", "", err
	}

	// Calculate gateway
	gw := net.ParseIP(subnet)
	if gw != nil {
		gw[3] = 1
		return ip, gw.String(), nil
	}

	return ip, "10.0.0.1", nil
}

func (m *Manager) nextIP(subnet string) (string, error) {
	return "10.0.0.2", nil
}