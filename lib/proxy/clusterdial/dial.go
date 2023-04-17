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
	"github.com/gravitational/teleport/lib/reversetunnel"
)

// ClusterDialer allows dialing resources or the auth server of a cluster.
type ClusterDialer struct {
	server reversetunnel.Server
}

// Dial dials makes a dial request to the given cluster.
func (cd *ClusterDialer) Dial(clusterName string, request peer.DialParams) (net.Conn, error) {
	site, err := cd.server.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialParams := reversetunnel.DialParams{
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

// DialAuth dials the auth server of the given cluster.
func (cd *ClusterDialer) DialAuth(clusterName string, request peer.DialParams) (net.Conn, error) {
	site, err := cd.server.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return site.DialAuthServer(reversetunnel.DialParams{
		From:                  request.From,
		To:                    request.To,
		OriginalClientDstAddr: request.To,
		FromPeerProxy:         true,
	})
}

// NewClusterDialer implements proxy.ClusterDialer for a reverse tunnel server.
func NewClusterDialer(server reversetunnel.Server) *ClusterDialer {
	return &ClusterDialer{
		server: server,
	}
}
