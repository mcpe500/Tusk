package vm

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestSerialClientWithGarbage(t *testing.T) {
	cConn, sConn := net.Pipe()
	t.Cleanup(func() {
		_ = cConn.Close()
		_ = sConn.Close()
	})

	cli := &SerialClient{
		conn:    cConn,
		reader:  bufio.NewReader(cConn),
		timeout: 1 * time.Second,
	}

	go func() {
		// Server sends garbage then JSON
		go func() {
			dec := json.NewDecoder(sConn)
			var req map[string]interface{}
			_ = dec.Decode(&req)
		}()

		_, _ = sConn.Write([]byte("Some boot logs here...\n"))
		_, _ = sConn.Write([]byte(`{"jsonrpc":"2.0","result":{"status":"pong"},"id":1}` + "\n"))
	}()

	res, err := cli.Send("Ping", nil)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	
	var resp map[string]interface{}
	if err := json.Unmarshal(res, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	
	result, ok := resp["result"].(map[string]interface{})
	if !ok || result["status"] != "pong" {
		t.Fatalf("unexpected result: %v", resp["result"])
	}
}
