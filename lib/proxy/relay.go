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

package proxy

import (
	"context"
	"net"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/transport/transportv1"
	"github.com/gravitational/teleport/lib/sshagent"
	"github.com/gravitational/teleport/lib/utils"
)

type relayAccessPoint interface {
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

type nodeWatcher = *services.GenericWatcher[types.Server, readonly.Server]

func getServerForRelay(ctx context.Context, host, port string, accessPoint relayAccessPoint, nodeWatcher nodeWatcher) (types.Server, error) {
	return getServer(ctx, host, port, &relaySite{
		accessPoint: accessPoint,
		nodeWatcher: nodeWatcher,
	})
}

type relaySite struct {
	accessPoint relayAccessPoint
	nodeWatcher nodeWatcher
}

var _ site = (*relaySite)(nil)

// GetNodes implements [site].
func (s *relaySite) GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	return s.nodeWatcher.CurrentResourcesWithFilter(ctx, fn)
}

// GetClusterNetworkingConfig implements [site].
func (s *relaySite) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return s.accessPoint.GetClusterNetworkingConfig(ctx)

}

// GetGitServers implements [site].
func (s *relaySite) GetGitServers(context.Context, func(readonly.Server) bool) ([]types.Server, error) {
	return nil, trace.NotImplemented("connectivity to git servers through the Relay service is not supported")
}

type relayTunnelDialFunc = func(ctx context.Context, hostID string, tunnelType types.TunnelType, src, dst net.Addr) (net.Conn, error)
type relayPeerDialFunc = func(ctx context.Context, hostID string, tunnelType types.TunnelType, relayIDs []string, src, dst net.Addr) (net.Conn, error)

type RelayRouterConfig struct {
	ClusterName string
	GroupName   string
	LocalDial   relayTunnelDialFunc
	PeerDial    relayPeerDialFunc
	AccessPoint relayAccessPoint
	NodeWatcher nodeWatcher
}

func NewRelayRouter(cfg RelayRouterConfig) (*relayRouter, error) {
	return &relayRouter{
		clusterName: cfg.ClusterName,
		groupName:   cfg.GroupName,
		localDial:   cfg.LocalDial,
		peerDial:    cfg.PeerDial,
		accessPoint: cfg.AccessPoint,
		nodeWatcher: cfg.NodeWatcher,
	}, nil
}

type relayRouter struct {
	clusterName string
	groupName   string
	localDial   relayTunnelDialFunc
	peerDial    relayPeerDialFunc
	accessPoint relayAccessPoint
	nodeWatcher nodeWatcher
}

var _ transportv1.Dialer = (*relayRouter)(nil)

// DialHost implements [transportv1.Dialer].
func (r *relayRouter) DialHost(ctx context.Context, clientSrcAddr net.Addr, clientDstAddr net.Addr, host string, port string, cluster string, _ func(types.RemoteCluster) error, _ sshagent.ClientGetter, _ agentless.SignerCreator) (net.Conn, error) {
	if cluster != r.clusterName {
		return nil, trace.NotImplemented("dialing nodes for a different cluster through the Relay service is not supported")
	}

	src, err := r.accessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		// deliberately not wrapping the error to not surface this as a NotFound
		// or some other meaningful error
		return nil, trace.Errorf("unable to determine recording mode: %s", err.Error())
	}
	if services.IsRecordAtProxy(src.GetMode()) {
		return nil, trace.NotImplemented("connectivity to SSH servers through the Relay service is not supported in Proxy recording mode")
	}

	server, err := getServerForRelay(ctx, host, port, r.accessPoint, r.nodeWatcher)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server == nil {
		return nil, trace.NotImplemented("direct dialing to nodes through the Relay service is not supported")
	}

	if server.IsOpenSSHNode() {
		return nil, trace.NotImplemented("connectivity to agentless servers through the Relay service is not supported")
	}

	localConn, err := r.localDial(ctx, server.GetName()+"."+r.clusterName, types.NodeTunnel, clientSrcAddr, clientDstAddr)
	if err == nil {
		return utils.NewConnWithAddr(localConn, clientDstAddr, clientSrcAddr), nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if server.GetRelayGroup() == "" {
		return nil, trace.NotFound("dial target doesn't appear to be available through any Relay group")
	}
	if server.GetRelayGroup() != r.groupName {
		return nil, trace.NotFound("dial target doesn't appear to be connected through this Relay group")
	}

	// peerDial might still fail to find a connection based on the advertised
	// relay IDs, the filtering done above is just to make the logic explicit
	peerConn, err := r.peerDial(ctx, server.GetName()+"."+r.clusterName, types.NodeTunnel, slices.Clone(server.GetRelayIDs()), clientSrcAddr, clientDstAddr)
	if err == nil {
		return utils.NewConnWithAddr(peerConn, clientDstAddr, clientSrcAddr), nil
	}

	return nil, trace.Wrap(err)
}

// DialSite implements [transportv1.Dialer].
func (r *relayRouter) DialSite(context.Context, string, net.Addr, net.Addr) (net.Conn, error) {
	return nil, trace.NotImplemented("connectivity to Auth services through the Relay service is not supported")
}

// DialWindowsDesktop implements [transportv1.Dialer].
func (r *relayRouter) DialWindowsDesktop(context.Context, net.Addr, net.Addr, string, string, func(types.RemoteCluster) error) (net.Conn, error) {
	return nil, trace.NotImplemented("connectivity to Windows desktops through the Relay service is not supported")
}
