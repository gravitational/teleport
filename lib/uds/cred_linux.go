//go:build linux

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
		ucred  *unix.Ucred
		rawErr error
	)
	err = raw.Control(func(fd uintptr) {
		// On Linux, we can just call Getsockopt for LOCAL_PEERCRED and we
		// get back a single ucred struct which contains all the info we need.
		ucred, err = unix.GetsockoptUcred(
			int(fd),
			unix.SOL_SOCKET,
			unix.SO_PEERCRED,
		)
		if err != nil {
			rawErr = trace.Wrap(err, "getting ucred for conn")
			return
		}
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling raw control")
	}
	if rawErr != nil {
		return nil, trace.Wrap(rawErr)
	}
	return &Creds{
		PID: int(ucred.Pid),
		UID: int(ucred.Uid),
		GID: int(ucred.Gid),
	}, nil
}
