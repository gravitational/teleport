//go:build unix

package common

import (
	"os"
	"os/exec"
)

const signalFd = 3

func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uintptr {
	cmd.ExtraFiles = []*os.File{signal}
	return signalFd
}
