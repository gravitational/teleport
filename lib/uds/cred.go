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
)

// Creds contains information about the peer connected to the UDS.
type Creds struct {
	// PID is the process ID of the peer.
	PID int
	// UID is the ID of the user that the peer process is running as.
	UID int
	// GID is the ID of the primary group that the peer process is running as.
	GID int
}

// GetCreds returns information about the peer connected to the UDS. It must
// be passed a net.Conn which encapsulates a *net.UnixConn.
func GetCreds(conn net.Conn) (*Creds, error) {
	udsConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, trace.BadParameter("requires *UnixConn, got %T", conn)
	}

	return getCreds(udsConn)
}
