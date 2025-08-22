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
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
)

// FakeServer is a fake Server implementation used in tests.
type FakeServer struct {
	Server
	// FakeClusters is a list of clusters registered via this fake reverse tunnel.
	FakeClusters []Cluster
}

// Clusters returns all available clusters.
func (s *FakeServer) Clusters(context.Context) ([]Cluster, error) {
	return s.FakeClusters, nil
}

// Cluster returns the cluster by name.
func (s *FakeServer) Cluster(_ context.Context, name string) (Cluster, error) {
	for _, cluster := range s.FakeClusters {
		if cluster.GetName() == name {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("cluster %q not found", name)
}

// FakeCluster is a fake reversetunnelclient.FakeCluster implementation used in tests.
type FakeCluster struct {
	Cluster
	// Name is the cluster name.
	Name string
	// AccessPoint is the auth server client.
	AccessPoint authclient.RemoteProxyAccessPoint
	// OfflineTunnels is a list of server IDs that will return connection error.
	OfflineTunnels map[string]struct{}
	// connCh receives the connection when dialing this cluster.
	connCh chan net.Conn
	// connCounter count how many connection requests the remote received.
	connCounter int64
	// closedMtx is a mutex that protects closed.
	closedMtx sync.Mutex
	// closed is set to true after the cluster is being closed.
	closed bool
}

// NewFakeCluster is a FakeCluster constructor.
func NewFakeCluster(clusterName string, accessPoint authclient.RemoteProxyAccessPoint) *FakeCluster {
	return &FakeCluster{
		Name:        clusterName,
		connCh:      make(chan net.Conn),
		AccessPoint: accessPoint,
	}
}

// CachingAccessPoint returns caching auth server client.
func (s *FakeCluster) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return s.AccessPoint, nil
}

// GetName returns the remote cluster name.
func (s *FakeCluster) GetName() string {
	return s.Name
}

// ProxyConn returns proxy connection channel with incoming connections.
func (s *FakeCluster) ProxyConn() <-chan net.Conn {
	return s.connCh
}

// Dial returns the connection to the remote cluster.
func (s *FakeCluster) Dial(params DialParams) (net.Conn, error) {
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

func (s *FakeCluster) Close() error {
	s.closedMtx.Lock()
	defer s.closedMtx.Unlock()
	close(s.connCh)
	s.closed = true
	return nil
}

func (s *FakeCluster) DialCount() int64 {
	return atomic.LoadInt64(&s.connCounter)
}
