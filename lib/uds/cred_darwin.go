//go:build darwin

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

func getCreds(conn *net.UnixConn) (*Creds, error) {
	// Get the underlying raw conn so we can use it for syscalls.
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, trace.Wrap(err, "converting *net.UnixConn to underlying raw conn")
	}

	var (
		xucred *unix.Xucred
		pid    int
		rawErr error
	)
	err = raw.Control(func(fd uintptr) {
		// On MacOS, we need to call Getsockopt twice, once for LOCAL_PEERCRED
		// and once for LOCAL_PEERPID. Unfortunately, the syscall differs from
		// the Linux version which offers all this information in a single
		// syscall.
		xucred, err = unix.GetsockoptXucred(
			int(fd),
			unix.SOL_LOCAL,
			unix.LOCAL_PEERCRED,
		)
		if err != nil {
			rawErr = trace.Wrap(err, "getting xucred for conn")
			return
		}
		pid, err = unix.GetsockoptInt(
			int(fd),
			unix.SOL_LOCAL,
			unix.LOCAL_PEERPID,
		)
		if err != nil {
			rawErr = trace.Wrap(err, "getting pid for conn")
			return
		}
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling raw control")
	}
	if rawErr != nil {
		return nil, trace.Wrap(rawErr)
	}

	creds := &Creds{
		PID: pid,
		UID: int(xucred.Uid),
	}
	if xucred.Ngroups > 0 {
		// Return just the first group ID. This is for consistency with Linux
		// where Getsockopt LOCAL_PEERCRED only returns the first group ID.
		creds.GID = int(xucred.Groups[0])
	}

	return creds, nil
}
