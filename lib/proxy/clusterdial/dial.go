// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
