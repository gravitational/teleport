/*
Copyright 2020 Gravitational, Inc.

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
	"net"
	"sync"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// FakeServer is a fake reversetunnel.Server implementation used in tests.
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

// FakeRemoteSite is a fake reversetunnel.RemoteSite implementation used in tests.
type FakeRemoteSite struct {
	RemoteSite
	// Name is the remote site name.
	Name string
	// AccessPoint is the auth server client.
	AccessPoint auth.RemoteProxyAccessPoint
	// OfflineTunnels is a list of server IDs that will return connection error.
	OfflineTunnels map[string]struct{}
	// connCh receives the connection when dialing this site.
	connCh chan net.Conn
	// closedMtx is a mutex that protects closed.
	closedMtx sync.Mutex
	// closed is set to true after the site is being closed.
	closed bool
}

// NewFakeRemoteSite is a FakeRemoteSite constructor.
func NewFakeRemoteSite(clusterName string, accessPoint auth.RemoteProxyAccessPoint) *FakeRemoteSite {
	return &FakeRemoteSite{
		Name:        clusterName,
		connCh:      make(chan net.Conn),
		AccessPoint: accessPoint,
	}
}

// CachingAccessPoint returns caching auth server client.
func (s *FakeRemoteSite) CachingAccessPoint() (auth.RemoteProxyAccessPoint, error) {
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
