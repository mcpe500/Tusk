package client

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tusk/tusk/pkg/protocol"
)

func setupTestClient(cConn net.Conn) *Client {
	cli := New("unused")
	cli.conn = cConn
	cli.reader = bufio.NewReader(cConn)
	cli.timeout = 1 * time.Second
	return cli
}

func TestCallWithGarbage(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := setupTestClient(cConn)

	go func() {
		// Server should read the request in background to avoid deadlock with net.Pipe
		go func() {
			dec := json.NewDecoder(sConn)
			var req protocol.JSONRPCRequest
			_ = dec.Decode(&req)
		}()

		// Server sends garbage then JSON
		_, _ = sConn.Write([]byte("Linux version 6.6.13-0-virt (alpine@alpine) ...\n"))
		_, _ = sConn.Write([]byte("Login: [  0.123] { garbage brace }\n"))
		
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"pong"`),
			ID:      1,
		}
		_ = json.NewEncoder(sConn).Encode(resp)
	}()

	res, err := cli.call("Ping", nil)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if string(res) != `"pong"` {
		t.Fatalf("unexpected result: %s", string(res))
	}
}

func TestCallDecodesChunkedResponse(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := setupTestClient(cConn)

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			return
		}

		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"version":"1.0.0","apiVersion":"v1","os":"linux","arch":"x86_64"}`),
			ID:      req.ID,
		}

		payload, err := json.Marshal(resp)
		if err != nil {
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

	cli := setupTestClient(cConn)

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
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

	cli := setupTestClient(cConn)

	go func() {
		dec := json.NewDecoder(sConn)
		var req protocol.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
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
