/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reversetunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
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

func (p *clusterPeers) CachingAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	peer, err := p.pickPeer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return peer.CachingAccessPoint()
}

func (p *clusterPeers) GetClient() (auth.ClientI, error) {
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

func (p *clusterPeers) DialAuthServer() (net.Conn, error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial to auth server, this proxy has not been discovered yet, try again later")
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (p *clusterPeers) Dial(params DialParams) (conn net.Conn, err error) {
	return p.DialTCP(params)
}

func (p *clusterPeers) DialTCP(params DialParams) (conn net.Conn, err error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial, this proxy has not been discovered yet, try again later")
}

// IsClosed always returns false because clusterPeers is never closed.
func (p *clusterPeers) IsClosed() bool { return false }

// Close always returns nil because a clusterPeers isn't closed.
func (p *clusterPeers) Close() error { return nil }

// newClusterPeer returns new cluster peer
func newClusterPeer(srv *server, connInfo types.TunnelConnection, offlineThreshold time.Duration) (*clusterPeer, error) {
	clusterPeer := &clusterPeer{
		srv:      srv,
		connInfo: connInfo,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentReverseTunnelServer,
			trace.ComponentFields: map[string]string{
				"cluster": connInfo.GetClusterName(),
			},
		}),
		clock:            clockwork.NewRealClock(),
		offlineThreshold: offlineThreshold,
	}

	return clusterPeer, nil
}

// clusterPeer is a remote cluster that has established
// a tunnel to the peers
type clusterPeer struct {
	log *log.Entry

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

func (s *clusterPeer) CachingAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	return nil, trace.ConnectionProblem(nil, "unable to fetch access point, this proxy %v has not been discovered yet, try again later", s)
}

func (s *clusterPeer) GetClient() (auth.ClientI, error) {
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
func (s *clusterPeer) Dial(params DialParams) (conn net.Conn, err error) {
	return nil, trace.ConnectionProblem(nil, "unable to dial, this proxy %v has not been discovered yet, try again later", s)
}

// Close closes cluster peer connections
func (s *clusterPeer) Close() error {
	return nil
}
