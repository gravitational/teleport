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
)

// SocketType encodes the type of desired socket (datagram or stream).
type SocketType int

const (
	// SocketTypeDatagram indicates that the socket type should be a unix datagram. Most usecases should
	// prefer stream sockets, but datagram can be beneficial when passing fds since it lets you ensure that
	// the bytes you receive alongside the fds exactly match those sent with the fds.
	SocketTypeDatagram SocketType = syscall.SOCK_DGRAM
	// SocketTypeStream indicates that the socket type should be a streaming socket. This is a reasonable default
	// for most usecases, though datagram may be preferable if fd passing is being used.
	SocketTypeStream SocketType = syscall.SOCK_STREAM
)

// proto converts SocketType into the expected value for use as the
// 'proto' argument in syscall.Socketpair.
func (s SocketType) proto() int {
	return int(s)
}
