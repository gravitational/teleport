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
	// ConnCh receives the connection when dialing this site.
	ConnCh chan net.Conn
	// AccessPoint is the auth server client.
	AccessPoint auth.AccessPoint
}

// CachingAccessPoint returns caching auth server client.
func (s *FakeRemoteSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return s.AccessPoint, nil
}

// GetName returns the remote site name.
func (s *FakeRemoteSite) GetName() string {
	return s.Name
}

// Dial returns the connection to the remote site.
func (s *FakeRemoteSite) Dial(params DialParams) (net.Conn, error) {
	readerConn, writerConn := net.Pipe()
	s.ConnCh <- readerConn
	return writerConn, nil
}
