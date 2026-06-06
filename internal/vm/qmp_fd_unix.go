//go:build unix

package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"syscall"
	"time"
)

func (c *QMPClient) AddFD(fd int, name string) error {
	if c.conn == nil {
		return fmt.Errorf("qmp connection is nil")
	}

	requestID := fmt.Sprintf("%d", time.Now().UnixNano())
	request := map[string]interface{}{
		"execute": "add-fd",
		"arguments": map[string]interface{}{
			"fdset-id": 0,
			"opaque":   name,
		},
		"id": requestID,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal add-fd request: %w", err)
	}
	payload = append(payload, '\n')

	if err := SendFD(c.conn, string(payload), fd); err != nil {
		return err
	}

	for {
		msg, err := c.Read()
		if err != nil {
			return err
		}

		if msg.Type == "event" {
			continue
		}

		if fmt.Sprintf("%v", msg.Id) != requestID {
			continue
		}

		if msg.Error != nil {
			return fmt.Errorf("add-fd failed: %s - %s", msg.Error.Class, msg.Error.Desc)
		}

		return nil
	}
}

func SendFD(conn io.Writer, name string, fd int) error {
	if fd < 0 {
		return fmt.Errorf("invalid fd: %d", fd)
	}
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	payload := []byte(name)

	syscallConn, ok := conn.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return fmt.Errorf("connection does not support FD passing")
	}

	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("qmp syscall conn: %w", err)
	}

	var sendErr error
	if err := rawConn.Write(func(sfd uintptr) bool {
		sendErr = syscall.Sendmsg(int(sfd), payload, syscall.UnixRights(fd), nil, 0)
		return true
	}); err != nil {
		return err
	} else if sendErr != nil {
		return fmt.Errorf("sendmsg: %w", sendErr)
	}

	return nil
}
