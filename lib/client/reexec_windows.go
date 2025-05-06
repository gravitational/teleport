//go:build windows

package client

import (
	"os"
	"os/exec"
	"syscall"
)

// addSignalFdToChild adds a file for the child process to inherit and returns
// the file descriptor of the file for the child.
func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uint64 {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.AdditionalInheritedHandles = append(
		cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(signal.Fd()),
	)
	return uint64(signal.Fd())
}
