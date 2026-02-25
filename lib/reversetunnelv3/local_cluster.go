// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package reversetunnelv3

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// localCluster implements [reversetunnelclient.Cluster] for the local Teleport
// cluster using [TunnelServer] as the dial backend.
//
// All inbound tunnel connections are tracked by TunnelServer. DialTCP locates
// the appropriate agent by hostID and scope and opens a new yamux stream.
// Dial delegates to DialTCP (forwarding-server and PROXY-header logic will be
// wired in when the reversetunnelv3 server is integrated into the main proxy).
type localCluster struct {
	log        *slog.Logger
	domainName string

	tunnelSrv   *TunnelServer
	accessPoint authclient.RemoteProxyAccessPoint
	client      authclient.ClientI
	authServers []string

	nodeWatcher *services.GenericWatcher[types.Server, readonly.Server]
	appWatcher  *services.GenericWatcher[types.AppServer, readonly.AppServer]
	gitWatcher  *services.GenericWatcher[types.Server, readonly.Server]

	ctx context.Context
}

var _ reversetunnelclient.Cluster = (*localCluster)(nil)

// GetName implements [reversetunnelclient.Cluster].
func (c *localCluster) GetName() string { return c.domainName }

// GetStatus implements [reversetunnelclient.Cluster].
func (c *localCluster) GetStatus() string { return teleport.RemoteClusterStatusOnline }

// GetLastConnected implements [reversetunnelclient.Cluster]. The local cluster
// is always connected so we return the current time.
func (c *localCluster) GetLastConnected() time.Time { return time.Now() }

// IsClosed implements [reversetunnelclient.Cluster]. The local cluster is never
// closed independently of the server.
func (c *localCluster) IsClosed() bool { return false }

// Close implements [reversetunnelclient.Cluster]. No-op for the local cluster.
func (c *localCluster) Close() error { return nil }

// String implements fmt.Stringer.
func (c *localCluster) String() string { return fmt.Sprintf("local(%v)", c.domainName) }

// GetClient implements [reversetunnelclient.Cluster].
func (c *localCluster) GetClient() (authclient.ClientI, error) { return c.client, nil }

// CachingAccessPoint implements [reversetunnelclient.Cluster].
func (c *localCluster) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return c.accessPoint, nil
}

// NodeWatcher implements [reversetunnelclient.Cluster].
func (c *localCluster) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return c.nodeWatcher, nil
}

// AppServerWatcher implements [reversetunnelclient.Cluster].
func (c *localCluster) AppServerWatcher() (*services.GenericWatcher[types.AppServer, readonly.AppServer], error) {
	return c.appWatcher, nil
}

// GitServerWatcher implements [reversetunnelclient.Cluster].
func (c *localCluster) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return c.gitWatcher, nil
}

// GetTunnelsCount implements [reversetunnelclient.Cluster].
func (c *localCluster) GetTunnelsCount() int {
	return c.tunnelSrv.tunnelCount()
}

// DialAuthServer implements [reversetunnelclient.Cluster].
func (c *localCluster) DialAuthServer(_ reversetunnelclient.DialParams) (net.Conn, error) {
	if len(c.authServers) == 0 {
		return nil, trace.ConnectionProblem(nil, "no auth servers available")
	}
	addr := utils.ChooseRandomString(c.authServers)
	conn, err := net.DialTimeout("tcp", addr, apidefaults.DefaultIOTimeout)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "unable to connect to auth server")
	}
	return conn, nil
}

// Dial implements [reversetunnelclient.Cluster]. For the v3 protocol, Dial
// currently delegates to DialTCP. The forwarding-server path (proxy recording
// mode, agentless nodes) will be added when this package is integrated into
// the main proxy.
func (c *localCluster) Dial(params reversetunnelclient.DialParams) (net.Conn, error) {
	return c.DialTCP(params)
}

// DialTCP implements [reversetunnelclient.Cluster]. It resolves the target
// agent from DialParams and opens a new yamux dial stream via TunnelServer.
func (c *localCluster) DialTCP(params reversetunnelclient.DialParams) (net.Conn, error) {
	c.log.DebugContext(c.ctx, "Initiating dial", "params", params)

	hostID := hostIDFromServerID(params.ServerID)
	scope := params.TargetScope
	if scope == "" && params.TargetServer != nil {
		scope = params.TargetServer.GetScope()
	}

	conn, err := c.tunnelSrv.Dial(c.ctx, hostID, scope, params.ConnType, params.From, params.To)
	if err != nil {
		return nil, trace.ConnectionProblem(err,
			"reverse tunnel to %q (%s) not found; check that the agent is running",
			params.ServerID, params.ConnType)
	}
	return conn, nil
}

// hostIDFromServerID extracts the bare host UUID from a ServerID that may be
// in the "uuid.clusterName" format used by DialParams.
func hostIDFromServerID(serverID string) string {
	hostID, _, _ := strings.Cut(serverID, ".")
	return hostID
}

// tunnelCount returns the total number of distinct agents currently connected
// to the TunnelServer.
func (s *TunnelServer) tunnelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, cs := range s.conns {
		n += len(cs)
	}
	return n
}
