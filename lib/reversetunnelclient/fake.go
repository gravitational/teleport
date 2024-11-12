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

package reversetunnelclient

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
)

// FakeServer is a fake Server implementation used in tests.
type FakeServer struct {
	Server
	// Sites is a list of sites registered via this fake reverse tunnel.
	Sites []RemoteSite
}

// GetSites returns all available remote sites.
func (s *FakeServer) GetSites() ([]RemoteSite, error) {
	return s.Sites, nil
}

// GetSite returns the remote site by name.
func (s *FakeServer) GetSite(name string) (RemoteSite, error) {
	for _, site := range s.Sites {
		if site.GetName() == name {
			return site, nil
		}
	}
	return nil, trace.NotFound("site %q not found", name)
}

// FakeRemoteSite is a fake reversetunnelclient.RemoteSite implementation used in tests.
type FakeRemoteSite struct {
	RemoteSite
	// Name is the remote site name.
	Name string
	// AccessPoint is the auth server client.
	AccessPoint authclient.RemoteProxyAccessPoint
	// OfflineTunnels is a list of server IDs that will return connection error.
	OfflineTunnels map[string]struct{}
	// connCh receives the connection when dialing this site.
	connCh chan net.Conn
	// connCounter count how many connection requests the remote received.
	connCounter int64
	// closedMtx is a mutex that protects closed.
	closedMtx sync.Mutex
	// closed is set to true after the site is being closed.
	closed bool
}

// NewFakeRemoteSite is a FakeRemoteSite constructor.
func NewFakeRemoteSite(clusterName string, accessPoint authclient.RemoteProxyAccessPoint) *FakeRemoteSite {
	return &FakeRemoteSite{
		Name:        clusterName,
		connCh:      make(chan net.Conn),
		AccessPoint: accessPoint,
	}
}

// CachingAccessPoint returns caching auth server client.
func (s *FakeRemoteSite) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return s.AccessPoint, nil
}

// GetName returns the remote site name.
func (s *FakeRemoteSite) GetName() string {
	return s.Name
}

// ProxyConn returns proxy connection channel with incoming connections.
func (s *FakeRemoteSite) ProxyConn() <-chan net.Conn {
	return s.connCh
}

// Dial returns the connection to the remote site.
func (s *FakeRemoteSite) Dial(params DialParams) (net.Conn, error) {
	atomic.AddInt64(&s.connCounter, 1)

	if _, ok := s.OfflineTunnels[params.ServerID]; ok {
		return nil, trace.ConnectionProblem(nil, "server %v tunnel is offline",
			params.ServerID)
	}

	s.closedMtx.Lock()
	defer s.closedMtx.Unlock()

	if s.closed {
		return nil, trace.ConnectionProblem(nil, "tunnel has been closed")
	}

	readerConn, writerConn := net.Pipe()
	s.connCh <- readerConn
	return writerConn, nil
}

func (s *FakeRemoteSite) Close() error {
	s.closedMtx.Lock()
	defer s.closedMtx.Unlock()
	close(s.connCh)
	s.closed = true
	return nil
}

func (s *FakeRemoteSite) DialCount() int64 {
	return atomic.LoadInt64(&s.connCounter)
}
