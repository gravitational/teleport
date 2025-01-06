//go:build linux
// +build linux

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

package srv

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

func init() {
	// errors in open/openat are signaled by returning -1, we don't really care
	// about the specifics anyway, so we can just ignore the error value
	//
	// we're opening with O_PATH rather than O_RDONLY because the binary might
	// not actually be readable (but only executable)
	fd1, _ := syscall.Open("/proc/self/exe", unix.O_PATH|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	fd2, _ := syscall.Open("/proc/self/exe", unix.O_PATH|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)

	// this can happen if both calls failed (returning -1) or if we're
	// running in a version of qemu-user that's affected by this bug:
	// https://gitlab.com/qemu-project/qemu/-/issues/927
	// (hopefully they'll also add special handling for execve on /proc/self/exe
	// if they ever fix that bug)
	if fd1 == fd2 {
		return
	}

	// closing -1 is harmless, no need to check here
	syscall.Close(fd1)
	syscall.Close(fd2)

	// if one Open has failed but not the other we can't really
	// trust the availability of "/proc/self/exe"
	if fd1 == -1 || fd2 == -1 {
		return
	}

	reexecPath = "/proc/self/exe"
}

// reexecPath specifies a path to execute on reexec, overriding Path in the cmd
// passed to reexecCommandOSTweaks, if not empty.
var reexecPath string

func reexecCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// Linux only: when parent process (node) dies unexpectedly without
	// cleaning up child processes, send a signal for graceful shutdown
	// to children.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGQUIT

	// replace the path on disk (which might not exist, or refer to an
	// upgraded version of teleport) with reexecPath, which contains
	// some path that refers to the specific binary we're running
	if reexecPath != "" {
		cmd.Path = reexecPath
	}
}

// if we ever need to run parkers on macOS or other platforms with no PDEATHSIG
// we should rework the parker to block on a pipe so it can exit when its parent
// is terminated
func parkerCommandOSTweaks(cmd *exec.Cmd) {
	reexecCommandOSTweaks(cmd)

	// parker processes can leak if their PDEATHSIG is SIGQUIT, otherwise we
	// could just use reexecCommandOSTweaks
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
