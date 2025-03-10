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

func newClusterPeers(clusterName string) *clusterPeers {
	return &clusterPeers{
		clusterName: clusterName,
		peers:       make(map[string]*clusterPeer),
	}
}

// clusterPeers is a collection of cluster peers to a given cluster
type clusterPeers struct {
	clusterName string
	peers       map[string]*clusterPeer
}

func (p *clusterPeers) GetTunnelsCount() int {
	return len(p.peers)
}

func (p *clusterPeers) pickPeer() (*clusterPeer, error) {
	var currentPeer *clusterPeer
	for _, peer := range p.peers {
		if currentPeer == nil || peer.getConnInfo().GetLastHeartbeat().After(currentPeer.getConnInfo().GetLastHeartbeat()) {
			currentPeer = peer
		}
	}
	if currentPeer == nil {
		return nil, trace.NotFound("no active peers found for %v", p.clusterName)
	}
	return currentPeer, nil
}

func (p *clusterPeers) updatePeer(conn types.TunnelConnection) bool {
	peer, ok := p.peers[conn.GetName()]
	if !ok {
		return false
	}
	peer.setConnInfo(conn)
	return true
}

func (p *clusterPeers) addPeer(peer *clusterPeer) {
	p.peers[peer.getConnInfo().GetName()] = peer
}

func (p *clusterPeers) removePeer(connInfo types.TunnelConnection) {
	delete(p.peers, connInfo.GetName())
}

func (p *clusterPeers) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	peer, err := p.pickPeer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return peer.CachingAccessPoint()
}

func (p *clusterPeers) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	peer, err := p.pickPeer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return peer.NodeWatcher()
}

func (p *clusterPeers) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	peer, err := p.pickPeer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return peer.GitServerWatcher()
}

func (p *clusterPeers) GetClient() (authclient.ClientI, error) {
	peer, err := p.pickPeer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return peer.GetClient()
}

func (p *clusterPeers) String() string {
	return fmt.Sprintf("clusterPeer(%v)", p.clusterName)
}

func (p *clusterPeers) GetStatus() string {
	peer, err := p.pickPeer()
	if err != nil {
		return teleport.RemoteClusterStatusOffline
	}
	return peer.GetStatus()
}

func (p *clusterPeers) GetName() string {
	return p.clusterName
}

func (p *clusterPeers) GetLastConnected() time.Time {
	peer, err := p.pickPeer()
	if err != nil {
		return time.Time{}
	}
	return peer.GetLastConnected()
}

func (p *clusterPeers) DialAuthServer(reversetunnelclient.DialParams) (net.Conn, error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial to auth server, this proxy has not been discovered yet, try again later")
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (p *clusterPeers) Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return p.DialTCP(params)
}

func (p *clusterPeers) DialTCP(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial, this proxy has not been discovered yet, try again later")
}

// IsClosed always returns false because clusterPeers is never closed.
func (p *clusterPeers) IsClosed() bool { return false }

// Close always returns nil because a clusterPeers isn't closed.
func (p *clusterPeers) Close() error { return nil }

// newClusterPeer returns new cluster peer
func newClusterPeer(srv *server, connInfo types.TunnelConnection, offlineThreshold time.Duration) (*clusterPeer, error) {
	clusterPeer := &clusterPeer{
		srv:              srv,
		connInfo:         connInfo,
		clock:            clockwork.NewRealClock(),
		offlineThreshold: offlineThreshold,
	}

	return clusterPeer, nil
}

// clusterPeer is a remote cluster that has established
// a tunnel to the peers
type clusterPeer struct {
	mu       sync.Mutex
	connInfo types.TunnelConnection
	srv      *server

	// clock is used to control time in tests.
	clock clockwork.Clock

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration
}

func (s *clusterPeer) getConnInfo() types.TunnelConnection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo
}

func (s *clusterPeer) setConnInfo(ci types.TunnelConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connInfo = ci
}

func (s *clusterPeer) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return nil, trace.ConnectionProblem(nil, "unable to fetch access point, this proxy %v has not been discovered yet, try again later", s)
}

func (s *clusterPeer) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return nil, trace.ConnectionProblem(nil, "unable to fetch node watcher, this proxy %v has not been discovered yet, try again later", s)
}

func (s *clusterPeer) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return nil, trace.ConnectionProblem(nil, "unable to fetch git server watcher, this proxy %v has not been discovered yet, try again later", s)
}

func (s *clusterPeer) GetClient() (authclient.ClientI, error) {
	return nil, trace.ConnectionProblem(nil, "unable to fetch client, this proxy %v has not been discovered yet, try again later", s)
}

func (s *clusterPeer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("clusterPeer(%v)", s.connInfo)
}

func (s *clusterPeer) GetStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return services.TunnelConnectionStatus(s.clock, s.connInfo, s.offlineThreshold)
}

func (s *clusterPeer) GetName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo.GetClusterName()
}

func (s *clusterPeer) GetLastConnected() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connInfo.GetLastHeartbeat()
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (s *clusterPeer) Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial, this proxy %v has not been discovered yet, try again later", s)
}

// Close closes cluster peer connections
func (s *clusterPeer) Close() error {
	return nil
}
