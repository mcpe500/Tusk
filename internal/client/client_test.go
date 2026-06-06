package client

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tusk/tusk/pkg/protocol"
)

func TestCallDecodesChunkedResponse(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := New("unused")
	cli.conn = cConn
	cli.timeout = 500 * time.Millisecond

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			t.Fatalf("server failed to decode request: %v", err)
			return
		}

		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"version":"1.0.0","apiVersion":"v1","os":"linux","arch":"x86_64"}`),
			ID:      req.ID,
		}

		payload, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("server failed to marshal response: %v", err)
			return
		}

		delimiter := len(payload) / 2
		if delimiter <= 0 {
			delimiter = len(payload)
		}
		_, _ = sConn.Write(payload[:delimiter])
		time.Sleep(10 * time.Millisecond)
		_, _ = sConn.Write(payload[delimiter:])
	}()

	result, err := cli.Info()
	if err != nil {
		t.Fatalf("client Info failed: %v", err)
	}
	if result.Version != "1.0.0" || result.APIVersion != "v1" {
		t.Fatalf("unexpected info result: %+v", result)
	}
}

func TestCallReturnsEmptyResultError(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := New("unused")
	cli.conn = cConn
	cli.timeout = 500 * time.Millisecond

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			t.Fatalf("server failed to decode request: %v", err)
			return
		}

		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		_ = json.NewEncoder(sConn).Encode(resp)
	}()

	_, err := cli.call("Ping", nil)
	if err == nil {
		t.Fatal("expected error from empty result")
	}
	if !strings.Contains(err.Error(), "empty rpc result") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoReturnsUnmarshalError(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := New("unused")
	cli.conn = cConn
	cli.timeout = 500 * time.Millisecond

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			t.Fatalf("server failed to decode request: %v", err)
			return
		}

		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"bad"`),
			ID:      req.ID,
		}
		_ = json.NewEncoder(sConn).Encode(resp)
	}()

	_, err := cli.Info()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if !strings.Contains(err.Error(), "info:") {
		t.Fatalf("expected wrapped info error, got: %v", err)
	}
}
