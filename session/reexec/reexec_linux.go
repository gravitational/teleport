/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reexec

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/gravitational/trace"
)

func CommandOSTweaks(cmd *exec.Cmd) {
	setLinuxReexecPath(cmd)

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}

	// Linux only: when parent process (node) dies unexpectedly without
	// cleaning up child processes, send a signal for graceful shutdown
	// to children.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGQUIT
}

// if we ever need to run parkers on macOS or other platforms with no PDEATHSIG
// we should rework the parker to block on a pipe so it can exit when its parent
// is terminated
func parkerCommandOSTweaks(cmd *exec.Cmd) {
	setLinuxReexecPath(cmd)

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}

	// parker processes can leak if their PDEATHSIG is SIGQUIT, otherwise we
	// could just use [commandOSTweaks]
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

func userCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}

	// Linux only: when parent process (this process) dies unexpectedly, kill
	// the child process instead of orphaning it.
	// SIGKILL because we don't control the child process, and it could choose
	// to ignore other signals.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

// setNeutralOOMScore sets the OOM score for the current process to 0 (the
// middle between -1000 and 1000). This value is inherited by all child processes.
func setNeutralOOMScore() error {
	// Use os.OpenFile() instead of os.WriteFile() to avoid creating the file
	// if for some extremely weird reason doesn't exist. Permission in this case
	// won't be used as os.O_WRONLY won't create the file.
	f, err := os.OpenFile("/proc/self/oom_score_adj", os.O_WRONLY, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	if _, err := f.WriteString("0"); err != nil {
		return trace.NewAggregate(err, f.Close())
	}

	// Make sure to return errors from Close(),
	// as sync error may be returned here.
	return trace.Wrap(f.Close())
}
