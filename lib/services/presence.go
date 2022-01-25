/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// ProxyGetter is a service that gets proxies.
type ProxyGetter interface {
	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// Presence records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type Presence interface {
	// Semaphores is responsible for semaphore handling
	types.Semaphores

	// UpsertLocalClusterName upserts local domain
	UpsertLocalClusterName(name string) error

	// GetLocalClusterName upserts local domain
	GetLocalClusterName() (string, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers.
	GetNodes(ctx context.Context, namespace string, opts ...MarshalOption) ([]types.Server, error)

	// ListNodes returns a paginated list of registered servers.
	ListNodes(ctx context.Context, req proto.ListNodesRequest) (nodes []types.Server, nextKey string, err error)

	// DeleteAllNodes deletes all nodes in a namespace.
	DeleteAllNodes(ctx context.Context, namespace string) error

	// DeleteNode deletes node in a namespace
	DeleteNode(ctx context.Context, namespace, name string) error

	// UpsertNode registers node presence, permanently if TTL is 0 or for the
	// specified duration with second resolution if it's >= 1 second.
	UpsertNode(ctx context.Context, server types.Server) (*types.KeepAlive, error)

	// UpsertNodes bulk inserts nodes.
	UpsertNodes(namespace string, servers []types.Server) error

	// DELETE IN: 5.1.0
	//
	// This logic has been moved to KeepAliveServer.
	//
	// KeepAliveNode updates node TTL in the storage
	KeepAliveNode(ctx context.Context, h types.KeepAlive) error

	// GetAuthServers returns a list of registered servers
	GetAuthServers() ([]types.Server, error)

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(server types.Server) error

	// DeleteAuthServer deletes auth server by name
	DeleteAuthServer(name string) error

	// DeleteAllAuthServers deletes all auth servers
	DeleteAllAuthServers() error

	// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(server types.Server) error

	// ProxyGetter gets a list of proxies
	ProxyGetter

	// DeleteProxy deletes proxy by name
	DeleteProxy(name string) error

	// DeleteAllProxies deletes all proxies
	DeleteAllProxies() error

	// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
	UpsertReverseTunnel(tunnel types.ReverseTunnel) error

	// GetReverseTunnel returns reverse tunnel by name
	GetReverseTunnel(name string, opts ...MarshalOption) (types.ReverseTunnel, error)

	// GetReverseTunnels returns a list of registered servers
	GetReverseTunnels(opts ...MarshalOption) ([]types.ReverseTunnel, error)

	// DeleteReverseTunnel deletes reverse tunnel by it's domain name
	DeleteReverseTunnel(domainName string) error

	// DeleteAllReverseTunnels deletes all reverse tunnels
	DeleteAllReverseTunnels() error

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// DeleteAllNamespaces deletes all namespaces
	DeleteAllNamespaces() error

	// UpsertNamespace upserts namespace
	UpsertNamespace(types.Namespace) error

	// DeleteNamespace deletes namespace by name
	DeleteNamespace(name string) error

	// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
	UpsertTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)

	// GetTrustedCluster returns a single TrustedCluster by name.
	GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error)

	// GetTrustedClusters returns all TrustedClusters in the backend.
	GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error)

	// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
	DeleteTrustedCluster(ctx context.Context, name string) error

	// UpsertTunnelConnection upserts tunnel connection
	UpsertTunnelConnection(types.TunnelConnection) error

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...MarshalOption) ([]types.TunnelConnection, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...MarshalOption) ([]types.TunnelConnection, error)

	// DeleteTunnelConnection deletes tunnel connection by name
	DeleteTunnelConnection(clusterName string, connName string) error

	// DeleteTunnelConnections deletes all tunnel connections for cluster
	DeleteTunnelConnections(clusterName string) error

	// DeleteAllTunnelConnections deletes all tunnel connections for cluster
	DeleteAllTunnelConnections() error

	// CreateRemoteCluster creates a remote cluster
	CreateRemoteCluster(types.RemoteCluster) error

	// UpdateRemoteCluster updates a remote cluster
	UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) error

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(opts ...MarshalOption) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)

	// DeleteRemoteCluster deletes remote cluster by name
	DeleteRemoteCluster(clusterName string) error

	// DeleteAllRemoteClusters deletes all remote clusters
	DeleteAllRemoteClusters() error

	// UpsertKubeService registers kubernetes service presence.
	UpsertKubeService(context.Context, types.Server) error

	// GetAppServers gets all application servers.
	//
	// DELETE IN 9.0. Deprecated, use GetApplicationServers.
	GetAppServers(context.Context, string, ...MarshalOption) ([]types.Server, error)
	// UpsertAppServer adds an application server.
	//
	// DELETE IN 9.0. Deprecated, use UpsertApplicationServer.
	UpsertAppServer(context.Context, types.Server) (*types.KeepAlive, error)
	// DeleteAppServer removes an application server.
	//
	// DELETE IN 9.0. Deprecated, use DeleteApplicationServer.
	DeleteAppServer(ctx context.Context, namespace, name string) error
	// DeleteAllAppServers removes all application servers.
	//
	// DELETE IN 9.0. Deprecated, use DeleteAllApplicationServers.
	DeleteAllAppServers(context.Context, string) error

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(context.Context, string) ([]types.AppServer, error)
	// UpsertApplicationServer registers an application server.
	UpsertApplicationServer(context.Context, types.AppServer) (*types.KeepAlive, error)
	// DeleteApplicationServer deletes specified application server.
	DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error
	// DeleteAllApplicationServers removes all registered application servers.
	DeleteAllApplicationServers(context.Context, string) error

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(context.Context, string, ...MarshalOption) ([]types.DatabaseServer, error)
	// UpsertDatabaseServer creates or updates a new database proxy server.
	UpsertDatabaseServer(context.Context, types.DatabaseServer) (*types.KeepAlive, error)
	// DeleteDatabaseServer removes the specified database proxy server.
	DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error
	// DeleteAllDatabaseServers removes all database proxy servers.
	DeleteAllDatabaseServers(context.Context, string) error

	// KeepAliveServer updates TTL of the server resource in the backend.
	KeepAliveServer(ctx context.Context, h types.KeepAlive) error

	// GetKubeServices returns a list of registered kubernetes services.
	GetKubeServices(context.Context) ([]types.Server, error)

	// DeleteKubeService deletes a named kubernetes service.
	DeleteKubeService(ctx context.Context, name string) error

	// DeleteAllKubeServices deletes all registered kubernetes services.
	DeleteAllKubeServices(context.Context) error

	// GetWindowsDesktopServices returns all registered Windows desktop services.
	GetWindowsDesktopServices(context.Context) ([]types.WindowsDesktopService, error)
	// UpsertWindowsDesktopService creates or updates a new Windows desktop service.
	UpsertWindowsDesktopService(context.Context, types.WindowsDesktopService) (*types.KeepAlive, error)
	// DeleteWindowsDesktopService removes the specified Windows desktop service.
	DeleteWindowsDesktopService(ctx context.Context, name string) error
	// DeleteAllWindowsDesktopServices removes all Windows desktop services.
	DeleteAllWindowsDesktopServices(context.Context) error

	// ListResoures returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (resources []types.ResourceWithLabels, nextKey string, err error)
}
