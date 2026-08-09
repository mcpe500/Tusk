package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type portForwarder struct{ forwards, removes int }

func (forwarder *portForwarder) ForwardTCP(context.Context, PublishedPort) error {
	forwarder.forwards++
	return nil
}
func (forwarder *portForwarder) RemoveTCP(context.Context, PublishedPort) error {
	forwarder.removes++
	return nil
}

func TestParsePublishedPortDefaultsLoopbackTCP(t *testing.T) {
	t.Parallel()
	port, err := ParsePublishedPort("8080:80", "container")
	if err != nil || port.HostIP != "127.0.0.1" || port.Protocol != "tcp" || port.HostPort != 8080 || port.ContainerPort != 80 {
		t.Fatalf("ParsePublishedPort() = %#v, %v", port, err)
	}
	if _, err := ParsePublishedPort("0.0.0.0:8080:80", "container"); err == nil {
		t.Fatal("non-loopback port was accepted")
	}
	ipv6, err := ParsePublishedPort("[::1]:8081:81", "container")
	if err != nil || ipv6.HostIP != "::1" || ipv6.HostPort != 8081 {
		t.Fatalf("IPv6 loopback parse = %#v, %v", ipv6, err)
	}
}

func TestPortReconcilerNeverClaimsWithoutForwarder(t *testing.T) {
	t.Parallel()
	reconciler := PortReconciler{Path: filepath.Join(t.TempDir(), "ports.json"), BackendID: "test"}
	port, err := ParsePublishedPort("8080:80", "container")
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Add(context.Background(), port); err == nil || !strings.Contains(err.Error(), "forwarder") {
		t.Fatal("port was claimed without a forwarder")
	}
}

func TestPortReconcilerPersistsForwardedState(t *testing.T) {
	var reconciler PortReconciler
	var forwarder *portForwarder
	var port PublishedPort
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		forwarder = &portForwarder{}
		reconciler = PortReconciler{Path: filepath.Join(t.TempDir(), fmt.Sprintf("ports-%d.json", attempt)), BackendID: "test", Forwarder: forwarder}
		freePort, reserveErr := reserveLoopbackPort()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		port, err = ParsePublishedPort(fmt.Sprintf("127.0.0.1:%d:80", freePort), "container")
		if err != nil {
			t.Fatal(err)
		}
		if err = reconciler.Add(context.Background(), port); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	stateValue, err := reconciler.Load()
	if err != nil || len(stateValue.Ports) != 1 || stateValue.Ports[0].State != "ready" || forwarder.forwards != 1 {
		t.Fatalf("port state = %#v, err=%v forwards=%d", stateValue, err, forwarder.forwards)
	}
	if err := reconciler.Remove(context.Background(), port.ID); err != nil || forwarder.removes != 1 {
		t.Fatalf("Remove() error=%v removes=%d", err, forwarder.removes)
	}
	if err := reconciler.Add(context.Background(), port); err != nil {
		t.Fatalf("re-add port error = %v", err)
	}
	if err := reconciler.Add(context.Background(), port); err != nil || forwarder.forwards != 2 {
		t.Fatalf("idempotent add error=%v forwards=%d", err, forwarder.forwards)
	}
}
