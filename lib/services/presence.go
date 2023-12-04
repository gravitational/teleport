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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
)

// ProxyGetter is a service that gets proxies.
type ProxyGetter interface {
	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// NodesGetter is a service that gets nodes.
type NodesGetter interface {
	// GetNodes returns a list of registered servers.
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
}

// DatabaseServersGetter is a service that gets database servers.
type DatabaseServersGetter interface {
	GetDatabaseServers(context.Context, string, ...MarshalOption) ([]types.DatabaseServer, error)
}

// AppServersGetter is a service that gets application servers.
type AppServersGetter interface {
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)
}

// NodesStreamGetter is a service that gets nodes.
type NodesStreamGetter interface {
	// GetNodeStream returns a list of registered servers.
	GetNodeStream(ctx context.Context, namespace string) stream.Stream[types.Server]
}

// Presence records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type Presence interface {
	// Inventory is a subset of Presence dedicated to tracking the status of all
	// teleport instances independent of any specific service.
	Inventory

	// Semaphores is responsible for semaphore handling
	types.Semaphores

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// NodesGetter gets nodes
	NodesGetter

	// DeleteAllNodes deletes all nodes in a namespace.
	DeleteAllNodes(ctx context.Context, namespace string) error

	// DeleteNode deletes node in a namespace
	DeleteNode(ctx context.Context, namespace, name string) error

	// UpsertNode registers node presence, permanently if TTL is 0 or for the
	// specified duration with second resolution if it's >= 1 second.
	UpsertNode(ctx context.Context, server types.Server) (*types.KeepAlive, error)

	// GetAuthServers returns a list of registered servers
	GetAuthServers() ([]types.Server, error)

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(ctx context.Context, server types.Server) error

	// DeleteAuthServer deletes auth server by name
	DeleteAuthServer(name string) error

	// DeleteAllAuthServers deletes all auth servers
	DeleteAllAuthServers() error

	// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(ctx context.Context, server types.Server) error

	// ProxyGetter gets a list of proxies
	ProxyGetter

	// DeleteProxy deletes proxy by name
	DeleteProxy(ctx context.Context, name string) error

	// DeleteAllProxies deletes all proxies
	DeleteAllProxies() error

	// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
	UpsertReverseTunnel(tunnel types.ReverseTunnel) error

	// GetReverseTunnel returns reverse tunnel by name
	GetReverseTunnel(name string, opts ...MarshalOption) (types.ReverseTunnel, error)

	// GetReverseTunnels returns a list of registered servers
	GetReverseTunnels(ctx context.Context, opts ...MarshalOption) ([]types.ReverseTunnel, error)

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

	// GetServerInfos returns a stream of ServerInfos.
	GetServerInfos(ctx context.Context) stream.Stream[types.ServerInfo]

	// GetServerInfo returns a ServerInfo by name.
	GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error)

	// UpsertServerInfo upserts a ServerInfo.
	UpsertServerInfo(ctx context.Context, si types.ServerInfo) error

	// DeleteServerInfo deletes a ServerInfo by name.
	DeleteServerInfo(ctx context.Context, name string) error

	// DeleteAllServerInfos deletes all ServerInfos.
	DeleteAllServerInfos(ctx context.Context) error

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
	DeleteRemoteCluster(ctx context.Context, clusterName string) error

	// DeleteAllRemoteClusters deletes all remote clusters
	DeleteAllRemoteClusters() error

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

	// GetKubernetesServers returns a list of registered kubernetes servers.
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)

	// DeleteKubernetesServer deletes a named kubernetes servers.
	DeleteKubernetesServer(ctx context.Context, hostID, name string) error

	// DeleteAllKubernetesServers deletes all registered kubernetes servers.
	DeleteAllKubernetesServers(context.Context) error

	// UpsertKubernetesServer registers an kubernetes server.
	UpsertKubernetesServer(context.Context, types.KubeServer) (*types.KeepAlive, error)

	// GetWindowsDesktopServices returns all registered Windows desktop services.
	GetWindowsDesktopServices(context.Context) ([]types.WindowsDesktopService, error)
	// GetWindowsDesktopService returns a Windows desktop service by name
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)
	// UpsertWindowsDesktopService creates or updates a new Windows desktop service.
	UpsertWindowsDesktopService(context.Context, types.WindowsDesktopService) (*types.KeepAlive, error)
	// DeleteWindowsDesktopService removes the specified Windows desktop service.
	DeleteWindowsDesktopService(ctx context.Context, name string) error
	// DeleteAllWindowsDesktopServices removes all Windows desktop services.
	DeleteAllWindowsDesktopServices(context.Context) error

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// PresenceInternal extends the Presence interface with auth-specific internal methods.
type PresenceInternal interface {
	Presence
	InventoryInternal
}
