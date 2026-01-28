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
	// The loginuid must be written to by the main thread or the write
	// may fail with EPERM sporadically. By locking the thread in an
	// init function we ensure that the write will always be done on
	// the main thread. See the comment in init() in lib/pam/pam.go
	// for more details.
	runtime.LockOSThread()
}

// WriteLoginUID writes the login UID to /proc/self/loginuid. This will
// ensure the kernel will update the audit session ID for the next
// child process.
func WriteLoginUID(uid string) error {
	err := os.WriteFile("/proc/self/loginuid", []byte(uid), 0o644)
	return trace.Wrap(err)
}
