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

package utils

import (
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/uds"
)

// DualPipeNetConn creates a pipe to connect a client and a server. The
// two net.Conn instances are wrapped in an PipeNetConn which holds the source and
// destination addresses.
//
// The pipe is constructed from a syscall.Socketpair instead of a net.Pipe because
// the synchronous nature of net.Pipe causes it to deadlock when attempting to perform
// TLS or SSH handshakes.
func DualPipeNetConn(srcAddr net.Addr, dstAddr net.Addr) (net.Conn, net.Conn, error) {
	client, server, err := uds.NewSocketpair(uds.SocketTypeStream)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	serverConn := NewConnWithAddr(server, dstAddr, srcAddr)
	clientConn := NewConnWithAddr(client, srcAddr, dstAddr)

	return serverConn, clientConn, nil
}
