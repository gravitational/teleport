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

package reversetunnelclient

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
)

// PeerDialerFunc is a function that implements [peerdial.Dialer].
type PeerDialerFunc func(clusterName string, request peerdial.DialParams) (net.Conn, error)

// Dial implements [peerdial.Dialer].
func (f PeerDialerFunc) Dial(clusterName string, request peerdial.DialParams) (net.Conn, error) {
	return f(clusterName, request)
}

// NewPeerDialer implements [peerdial.Dialer] for a reverse tunnel server.
func NewPeerDialer(clusterGetter ClusterGetter) PeerDialerFunc {
	return func(clusterName string, request peerdial.DialParams) (net.Conn, error) {
		site, err := clusterGetter.Cluster(context.TODO(), clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialParams := DialParams{
			ServerID:      request.ServerID,
			ConnType:      request.ConnType,
			From:          request.From,
			To:            request.To,
			FromPeerProxy: true,
		}

		// peered dials should be passthru so we call [localCluster.DialTCP] directly
		conn, err := site.DialTCP(dialParams)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
}
