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
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
)

// Config holds the parameters for [NewTunnelServerWrapper].
type Config struct {
	Log *slog.Logger

	// TunnelServer is the underlying protocol server that manages agent
	// connections and routes dials.
	TunnelServer *TunnelServer

	// DomainName is the local cluster's domain name.
	DomainName string

	// AuthServers is the list of auth server addresses to use for
	// DialAuthServer.
	AuthServers []string

	// Client is the local auth client.
	Client authclient.ClientI

	// AccessPoint is a cached access point for the local cluster.
	AccessPoint authclient.RemoteProxyAccessPoint

	// NodeWatcher is the node watcher for the local cluster.
	NodeWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// AppServerWatcher is the app server watcher for the local cluster.
	AppServerWatcher *services.GenericWatcher[types.AppServer, readonly.AppServer]

	// GitServerWatcher is the git server watcher for the local cluster.
	GitServerWatcher *services.GenericWatcher[types.Server, readonly.Server]
}

// TunnelServerWrapper implements [reversetunnelclient.Server] for the
// reversetunnelv3 protocol. It wraps a [TunnelServer] and exposes the
// [reversetunnelclient.Cluster] interface for the local cluster.
type TunnelServerWrapper struct {
	cfg  Config
	site *localCluster

	// userConnections tracks the count of active user connections, used
	// to prevent premature shutdown while sessions are in flight.
	userConnections atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

var _ reversetunnelclient.Server = (*TunnelServerWrapper)(nil)

// NewTunnelServerWrapper creates a [TunnelServerWrapper] ready to serve
// requests. Call [TunnelServerWrapper.Start] to begin accepting connections.
func NewTunnelServerWrapper(ctx context.Context, cfg Config) (*TunnelServerWrapper, error) {
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing Log")
	}
	if cfg.TunnelServer == nil {
		return nil, trace.BadParameter("missing TunnelServer")
	}
	if cfg.DomainName == "" {
		return nil, trace.BadParameter("missing DomainName")
	}

	wrapCtx, cancel := context.WithCancel(ctx)

	site := &localCluster{
		log:         cfg.Log,
		domainName:  cfg.DomainName,
		tunnelSrv:   cfg.TunnelServer,
		accessPoint: cfg.AccessPoint,
		client:      cfg.Client,
		authServers: cfg.AuthServers,
		nodeWatcher: cfg.NodeWatcher,
		appWatcher:  cfg.AppServerWatcher,
		gitWatcher:  cfg.GitServerWatcher,
		ctx:         wrapCtx,
	}

	return &TunnelServerWrapper{
		cfg:    cfg,
		site:   site,
		ctx:    wrapCtx,
		cancel: cancel,
	}, nil
}

// Start implements [reversetunnelclient.Server]. The underlying TunnelServer
// accepts connections through the gRPC credentials mechanism and does not
// require a separate Start call.
func (s *TunnelServerWrapper) Start() error { return nil }

// Close implements [reversetunnelclient.Server].
func (s *TunnelServerWrapper) Close() error {
	s.cancel()
	return s.cfg.TunnelServer.Close()
}

// DrainConnections implements [reversetunnelclient.Server]. It marks the
// tunnel server as terminating so that agents will start reconnecting
// elsewhere, but does not close existing connections.
func (s *TunnelServerWrapper) DrainConnections(ctx context.Context) error {
	s.cfg.TunnelServer.SetTerminating()
	return nil
}

// Shutdown implements [reversetunnelclient.Server]. It marks the server as
// terminating and then waits for all agent connections to close.
func (s *TunnelServerWrapper) Shutdown(ctx context.Context) error {
	s.cancel()
	s.cfg.TunnelServer.Shutdown(ctx)
	return nil
}

// Wait implements [reversetunnelclient.Server].
func (s *TunnelServerWrapper) Wait(ctx context.Context) {
	select {
	case <-s.ctx.Done():
	case <-ctx.Done():
	}
}

// GetProxyPeerClient implements [reversetunnelclient.Server]. Proxy peering is
// not yet wired into the reversetunnelv3 path; returns nil.
func (s *TunnelServerWrapper) GetProxyPeerClient() *peer.Client { return nil }

// TrackUserConnection implements [reversetunnelclient.Server]. The returned
// function must be called when the connection ends to decrement the counter.
func (s *TunnelServerWrapper) TrackUserConnection() (release func()) {
	s.userConnections.Add(1)
	return func() { s.userConnections.Add(-1) }
}

// Clusters implements [reversetunnelclient.ClusterGetter].
func (s *TunnelServerWrapper) Clusters(_ context.Context) ([]reversetunnelclient.Cluster, error) {
	return []reversetunnelclient.Cluster{s.site}, nil
}

// Cluster implements [reversetunnelclient.ClusterGetter].
func (s *TunnelServerWrapper) Cluster(_ context.Context, name string) (reversetunnelclient.Cluster, error) {
	if name == s.cfg.DomainName {
		return s.site, nil
	}
	return nil, trace.NotFound("cluster %q not found", name)
}
