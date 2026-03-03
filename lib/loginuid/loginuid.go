//go:build bpf && !386

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package loginuid

import (
	"os"
	"runtime"

	"github.com/gravitational/trace"
)

func init() {
	// Lock writing to loginuid to the startup thread. From LockOSThread docs:
	//
	//     All init functions are run on the startup thread. Calling LockOSThread from
	//     an init function will cause the main function to be invoked on that thread.
	//
	// This is needed when writing to "/proc/self/loginuid"
	// which, on Linux, depends on being called from a specific thread. If
	// it's not running on the right thread, writing to "/proc/self/loginuid"
	// may fail with EPERM sporadically.
	//
	// > Why the startup thread specifically?
	//   The kernel does some validation based on the thread context. I could
	//   not find what the kernel uses specifically. Some relevant code:
	//   https://github.com/torvalds/linux/blob/9d99b1647fa56805c1cfef2d81ee7b9855359b62/kernel/audit.c#L2284-L2317
	//   Locking to the startup thread seems to make the kernel happy.
	//   If you figure out more, please update this comment.
	//
	// > Why not call LockOSThread from loginuid.Write?
	//   By the time loginuid.Write gets called, more goroutines could've been
	//   spawned.  This means that the main goroutine (running loginuid.Write) could
	//   get re-scheduled to a different thread.
	//
	// > Why does loginuid.Write run on the main goroutine?
	//   This is an assumption. As of today, this is true because teleport
	//   re-executes itself and calls loginuid.Write synchronously. If we change this
	//   later, loginuid can become flaky again.
	//
	// > What does OpenSSH do?
	//   OpenSSH has a separate "authentication thread" which does all the PAM
	//   stuff (and also writes to "/proc/self/loginuid" if necessary):
	//   https://github.com/openssh/openssh-portable/blob/598c3a5e3885080ced0d7c40fde00f1d5cdbb32b/auth-pam.c#L470-L474
	//
	// Some historic context:
	// https://github.com/gravitational/teleport/issues/2476
	runtime.LockOSThread()
}

// Write writes the login UID to /proc/self/loginuid. This will
// ensure the kernel will update the audit session ID for the next
// child process.
func Write(uid string) error {
	err := os.WriteFile("/proc/self/loginuid", []byte(uid), 0o644)
	return trace.Wrap(err)
}
