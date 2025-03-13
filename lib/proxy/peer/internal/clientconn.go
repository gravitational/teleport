// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package internal

import (
	"context"
	"net"

	"github.com/gravitational/teleport/api/types"
)

// ClientConn manages client connections to a specific peer proxy (with a fixed
// host ID and address).
type ClientConn interface {
	// PeerID returns the host ID of the peer proxy.
	PeerID() string
	// PeerAddr returns the address of the peer proxy.
	PeerAddr() string

	// Dial opens a connection of a given tunnel type to a node with the given
	// ID through the peer proxy managed by the clientConn.
	Dial(
		nodeID string,
		src net.Addr,
		dst net.Addr,
		tunnelType types.TunnelType,
		permit []byte,
	) (net.Conn, error)

	// Ping checks if the peer is reachable and responsive.
	Ping(context.Context) error

	// Close closes all connections and releases any background resources
	// immediately.
	Close() error

	// Shutdown waits until all connections are closed or the context is done,
	// then acts like Close.
	Shutdown(context.Context)
}
