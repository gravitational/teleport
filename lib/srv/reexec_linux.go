// +build linux

package srv

import (
	"os/exec"
	"syscall"
)

func reexecCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// Linux only: when parent process (node) dies unexpectedly without
	// cleaning up child processes, send a signal for graceful shutdown
	// to children.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGQUIT
}

func userCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// Linux only: when parent process (this process) dies unexpectedly, kill
	// the child process instead of orphaning it.
	// SIGKILL because we don't control the child process and it could choose
	// to ignore other signals.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
