//go:build unix

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
	"net"
	"os"

	"github.com/gravitational/trace"
)

// NewSocketpair creates a unix socket pair, returning the halves as
// [*net.UnixConn]s.
func NewSocketpair(t SocketType) (left, right *net.UnixConn, err error) {
	lfd, rfd, err := cloexecSocketpair(t)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	lfile, rfile := os.NewFile(lfd, "lsock"), os.NewFile(rfd, "rsock")
	defer func() {
		lfile.Close()
		rfile.Close()
	}()

	left, err = FromFile(lfile)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	right, err = FromFile(rfile)
	if err != nil {
		left.Close()
		return nil, nil, trace.Wrap(err)
	}

	return left, right, nil
}
