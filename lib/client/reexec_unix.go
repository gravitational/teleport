//go:build unix

package client

import (
	"os"
	"os/exec"
)

// addSignalFdToChild adds a file for the child process to inherit and returns
// the file descriptor of the file for the child.
func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uint64 {
	cmd.ExtraFiles = append(cmd.ExtraFiles, signal)
	return uint64(len(cmd.ExtraFiles) + 2)
}
