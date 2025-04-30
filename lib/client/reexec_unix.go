//go:build unix

package client

import (
	"os"
	"os/exec"
)

func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uint64 {
	cmd.ExtraFiles = append(cmd.ExtraFiles, signal)
	return uint64(len(cmd.ExtraFiles) + 2)
}
