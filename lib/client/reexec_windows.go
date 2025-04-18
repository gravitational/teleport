//go:build windows

package client

import (
	"os"
	"os/exec"
	"syscall"
)

func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uintptr {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(signal.Fd())},
	}
	return signal.Fd()
}
