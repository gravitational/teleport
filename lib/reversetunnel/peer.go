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

package reversetunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
)

func newExpectedLeafClusters(clusterName string) *expectedLeafClusters {
	return &expectedLeafClusters{
		clusterName: clusterName,
		clusters:    make(map[string]*expectedLeafCluster),
	}
}

// expectedLeafClusters is a collection of placeholders for a given cluster.
type expectedLeafClusters struct {
	clusterName string
	clusters    map[string]*expectedLeafCluster
}

func (p *expectedLeafClusters) GetTunnelsCount() int {
	return len(p.clusters)
}

func (p *expectedLeafClusters) pickCluster() (*expectedLeafCluster, error) {
	var currentCluster *expectedLeafCluster
	for _, cluster := range p.clusters {
		if currentCluster == nil || cluster.getConnInfo().GetLastHeartbeat().After(currentCluster.getConnInfo().GetLastHeartbeat()) {
			currentCluster = cluster
		}
	}
	if currentCluster == nil {
		return nil, trace.NotFound("no active clusters found for %v", p.clusterName)
	}
	return currentCluster, nil
}

func (p *expectedLeafClusters) updateCluster(conn types.TunnelConnection) bool {
	cluster, ok := p.clusters[conn.GetName()]
	if !ok {
		return false
	}
	cluster.setConnInfo(conn)
	return true
}

func (p *expectedLeafClusters) addCluster(cluster *expectedLeafCluster) {
	p.clusters[cluster.getConnInfo().GetName()] = cluster
}

func (p *expectedLeafClusters) removeCluster(connInfo types.TunnelConnection) {
	delete(p.clusters, connInfo.GetName())
}

func (p *expectedLeafClusters) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	cluster, err := p.pickCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster.CachingAccessPoint()
}

func (p *expectedLeafClusters) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	cluster, err := p.pickCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster.NodeWatcher()
}

func (p *expectedLeafClusters) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	cluster, err := p.pickCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster.GitServerWatcher()
}

func (p *expectedLeafClusters) GetClient() (authclient.ClientI, error) {
	cluster, err := p.pickCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster.GetClient()
}

func (p *expectedLeafClusters) String() string {
	return fmt.Sprintf("expectedLeafClusters(%v)", p.clusterName)
}

func (p *expectedLeafClusters) GetStatus() string {
	cluster, err := p.pickCluster()
	if err != nil {
		return teleport.RemoteClusterStatusOffline
	}
	return cluster.GetStatus()
}

func (p *expectedLeafClusters) GetName() string {
	return p.clusterName
}

func (p *expectedLeafClusters) GetLastConnected() time.Time {
	cluster, err := p.pickCluster()
	if err != nil {
		return time.Time{}
	}
	return cluster.GetLastConnected()
}

func (p *expectedLeafClusters) DialAuthServer(reversetunnelclient.DialParams) (net.Conn, error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial auth server in leaf cluster %q, the leaf cluster has not established all tunnels yet, try again later", p.clusterName)
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a leaf cluster, the connection goes through the
// reverse proxy tunnel.
func (p *expectedLeafClusters) Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return p.DialTCP(params)
}

func (p *expectedLeafClusters) DialTCP(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial %s in leaf cluster %q, the leaf cluster has not established all tunnels yet, try again later", params.String(), p.clusterName)
}

// IsClosed always returns false because expectedLeafCluster is never closed.
func (p *expectedLeafClusters) IsClosed() bool { return false }

// Close is noop.
func (p *expectedLeafClusters) Close() error { return nil }

// newExpectedLeafCluster returns new cluster placeholder.
func newExpectedLeafCluster(srv *server, connInfo types.TunnelConnection, offlineThreshold time.Duration) *expectedLeafCluster {
	return &expectedLeafCluster{
		srv:              srv,
		connInfo:         connInfo,
		clock:            clockwork.NewRealClock(),
		offlineThreshold: offlineThreshold,
	}

}

// expectedLeafCluster represents a connection to a leaf
// cluster that has yet to register any tunnels
type expectedLeafCluster struct {
	mu       sync.Mutex
	connInfo types.TunnelConnection
	srv      *server

	// clock is used to control time in tests.
	clock clockwork.Clock

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration
}

func (s *expectedLeafCluster) getConnInfo() types.TunnelConnection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo
}

func (s *expectedLeafCluster) setConnInfo(ci types.TunnelConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connInfo = ci
}

func (s *expectedLeafCluster) discoveryError(msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return trace.ConnectionProblem(nil, "%s, the leaf cluster %q has not established all tunnels yet, try again later", msg, s.connInfo.GetClusterName())
}

func (s *expectedLeafCluster) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return nil, s.discoveryError("unable to fetch access point for leaf cluster")
}

func (s *expectedLeafCluster) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return nil, s.discoveryError("unable to fetch node watcher for leaf cluster")
}

func (s *expectedLeafCluster) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return nil, s.discoveryError("unable to fetch git server watcher for leaf cluster")
}

func (s *expectedLeafCluster) GetClient() (authclient.ClientI, error) {
	return nil, s.discoveryError("unable to fetch auth client for leaf cluster")
}

func (s *expectedLeafCluster) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("expectedLeafCluster(%v)", s.connInfo)
}

func (s *expectedLeafCluster) GetStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return services.TunnelConnectionStatus(s.clock, s.connInfo, s.offlineThreshold)
}

func (s *expectedLeafCluster) GetName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo.GetClusterName()
}

func (s *expectedLeafCluster) GetLastConnected() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo.GetLastHeartbeat()
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a leaf clsuter, the connection goes through the
// reverse proxy tunnel.
func (s *expectedLeafCluster) Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return nil, s.discoveryError("unable to dial target")
}

// Close is a noop.
func (s *expectedLeafCluster) Close() error {
	return nil
}
