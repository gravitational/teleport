//go:build unix && !darwin

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

package uds

import (
	"syscall"

	"github.com/gravitational/trace"
)

// cloexecSocketpair returns a unix/local stream socketpair whose file
// descriptors are flagged close-on-exec. This implementation creates the
// socketpair directly in close-on-exec mode.
func cloexecSocketpair(t SocketType) (uintptr, uintptr, error) {
	// SOCK_CLOEXEC on socketpair is supported since Linux 2.6.27 and go's
	// minimum requirement is 2.6.32 (FreeBSD supports it since FreeBSD 10 and
	// go 1.20+ requires FreeBSD 12)
	fds, err := syscall.Socketpair(syscall.AF_UNIX, t.proto()|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}

	return uintptr(fds[0]), uintptr(fds[1]), nil
}
