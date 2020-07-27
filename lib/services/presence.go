/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// ProxyGetter is an service that gets proxies
type ProxyGetter interface {
	// GetProxies returns a list of registered proxies
	GetProxies() ([]Server, error)
}

// Presence records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type Presence interface {
	// UpsertLocalClusterName upserts local domain
	UpsertLocalClusterName(name string) error

	// GetLocalClusterName upserts local domain
	GetLocalClusterName() (string, error)

	// GetNodes returns a list of registered servers. Schema validation can be
	// skipped to improve performance.
	GetNodes(namespace string, opts ...MarshalOption) ([]Server, error)

	// DeleteAllNodes deletes all nodes in a namespace.
	DeleteAllNodes(namespace string) error

	// DeleteNode deletes node in a namespace
	DeleteNode(namespace, name string) error

	// UpsertNode registers node presence, permanently if TTL is 0 or for the
	// specified duration with second resolution if it's >= 1 second.
	UpsertNode(server Server) (*KeepAlive, error)

	// UpsertNodes bulk inserts nodes.
	UpsertNodes(namespace string, servers []Server) error

	// KeepAliveNode updates node TTL in the storage
	KeepAliveNode(ctx context.Context, h KeepAlive) error

	// GetAuthServers returns a list of registered servers
	GetAuthServers() ([]Server, error)

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(server Server) error

	// DeleteAuthServer deletes auth server by name
	DeleteAuthServer(name string) error

	// DeleteAllAuthServers deletes all auth servers
	DeleteAllAuthServers() error

	// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(server Server) error

	// ProxyGetter gets a list of proxies
	ProxyGetter

	// DeleteProxy deletes proxy by name
	DeleteProxy(name string) error

	// DeleteAllProxies deletes all proxies
	DeleteAllProxies() error

	// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
	UpsertReverseTunnel(tunnel ReverseTunnel) error

	// GetReverseTunnel returns reverse tunnel by name
	GetReverseTunnel(name string, opts ...MarshalOption) (ReverseTunnel, error)

	// GetReverseTunnels returns a list of registered servers
	GetReverseTunnels(opts ...MarshalOption) ([]ReverseTunnel, error)

	// DeleteReverseTunnel deletes reverse tunnel by it's domain name
	DeleteReverseTunnel(domainName string) error

	// DeleteAllReverseTunnels deletes all reverse tunnels
	DeleteAllReverseTunnels() error

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*Namespace, error)

	// DeleteAllNamespaces deletes all namespaces
	DeleteAllNamespaces() error

	// UpsertNamespace upserts namespace
	UpsertNamespace(Namespace) error

	// DeleteNamespace deletes namespace by name
	DeleteNamespace(name string) error

	// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
	UpsertTrustedCluster(ctx context.Context, tc TrustedCluster) (TrustedCluster, error)

	// GetTrustedCluster returns a single TrustedCluster by name.
	GetTrustedCluster(string) (TrustedCluster, error)

	// GetTrustedClusters returns all TrustedClusters in the backend.
	GetTrustedClusters() ([]TrustedCluster, error)

	// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
	DeleteTrustedCluster(ctx context.Context, name string) error

	// UpsertTunnelConnection upserts tunnel connection
	UpsertTunnelConnection(TunnelConnection) error

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...MarshalOption) ([]TunnelConnection, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...MarshalOption) ([]TunnelConnection, error)

	// DeleteTunnelConnection deletes tunnel connection by name
	DeleteTunnelConnection(clusterName string, connName string) error

	// DeleteTunnelConnections deletes all tunnel connections for cluster
	DeleteTunnelConnections(clusterName string) error

	// DeleteAllTunnelConnections deletes all tunnel connections for cluster
	DeleteAllTunnelConnections() error

	// CreateRemoteCluster creates a remote cluster
	CreateRemoteCluster(RemoteCluster) error

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(opts ...MarshalOption) ([]RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (RemoteCluster, error)

	// DeleteRemoteCluster deletes remote cluster by name
	DeleteRemoteCluster(clusterName string) error

	// DeleteAllRemoteClusters deletes all remote clusters
	DeleteAllRemoteClusters() error
}

// NewNamespace returns new namespace
func NewNamespace(name string) Namespace {
	return Namespace{
		Kind:    KindNamespace,
		Version: V2,
		Metadata: Metadata{
			Name: name,
		},
	}
}

// Site represents a cluster of teleport nodes who collectively trust the same
// certificate authority (CA) and have a common name.
//
// The CA is represented by an auth server (or multiple auth servers, if running
// in HA mode)
type Site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"lastconnected"`
	Status        string    `json:"status"`
}

// IsEmpty returns true if keepalive is empty,
// used to indicate that keepalive is not supported
func (s *KeepAlive) IsEmpty() bool {
	return s.LeaseID == 0 && s.ServerName == ""
}

func (s *KeepAlive) CheckAndSetDefaults() error {
	if s.IsEmpty() {
		return trace.BadParameter("no lease ID or server name is specified")
	}
	if s.Namespace == "" {
		s.Namespace = defaults.Namespace
	}
	return nil
}

// KeepAliver keeps object alive
type KeepAliver interface {
	// KeepAlives allows to receive keep alives
	KeepAlives() chan<- KeepAlive

	// Done returns the channel signalling the closure
	Done() <-chan struct{}

	// Close closes the watcher and releases
	// all associated resources
	Close() error

	// Error returns error associated with keep aliver if any
	Error() error
}
