//go:build !unix

package vm

import (
	"fmt"
	"io"
)

func (c *QMPClient) AddFD(fd int, name string) error {
	return fmt.Errorf("QMP fd passing is not supported on this platform")
}

func SendFD(conn io.Writer, name string, fd int) error {
	return fmt.Errorf("QMP fd passing is not supported on this platform")
}
