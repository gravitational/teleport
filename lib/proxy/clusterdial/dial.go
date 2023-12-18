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

package clusterdial

import (
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

// ClusterDialerFunc is a function that implements a peer.ClusterDialer.
type ClusterDialerFunc func(clusterName string, request peer.DialParams) (net.Conn, error)

// Dial dials makes a dial request to the given cluster.
func (f ClusterDialerFunc) Dial(clusterName string, request peer.DialParams) (net.Conn, error) {
	return f(clusterName, request)
}

// NewClusterDialer implements proxy.ClusterDialer for a reverse tunnel server.
func NewClusterDialer(server reversetunnelclient.Server) ClusterDialerFunc {
	return func(clusterName string, request peer.DialParams) (net.Conn, error) {
		site, err := server.GetSite(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialParams := reversetunnelclient.DialParams{
			ServerID:      request.ServerID,
			ConnType:      request.ConnType,
			From:          request.From,
			To:            request.To,
			FromPeerProxy: true,
		}

		conn, err := site.Dial(dialParams)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
}
