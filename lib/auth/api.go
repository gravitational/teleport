/*
Copyright 2015-2020 Gravitational, Inc.

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

package auth

import (
	"context"
	"io"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Announcer specifies interface responsible for announcing presence
type Announcer interface {
	// UpsertNode registers node presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error)

	// UpsertProxy registers proxy presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(s types.Server) error

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(s types.Server) error

	// UpsertKubeService registers kubernetes presence, permanently if ttl is 0
	// or for the specified duration with second resolution if it's >= 1 second
	UpsertKubeService(context.Context, types.Server) error

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (types.KeepAliver, error)

	// UpsertAppServer adds an application server.
	//
	// DELETE IN 9.0. Deprecated, use UpsertApplicationServer.
	UpsertAppServer(context.Context, types.Server) (*types.KeepAlive, error)

	// UpsertApplicationServer registers an application server.
	UpsertApplicationServer(context.Context, types.AppServer) (*types.KeepAlive, error)

	// UpsertDatabaseServer registers a database proxy server.
	UpsertDatabaseServer(context.Context, types.DatabaseServer) (*types.KeepAlive, error)

	// UpsertWindowsDesktopService registers a Windows desktop service.
	UpsertWindowsDesktopService(context.Context, types.WindowsDesktopService) (*types.KeepAlive, error)

	// CreateWindowsDesktop registers a Windows desktop host.
	CreateWindowsDesktop(context.Context, types.WindowsDesktop) error
	// UpdateWindowsDesktop updates a Windows desktop host.
	UpdateWindowsDesktop(context.Context, types.WindowsDesktop) error
}

// accessPoint is an API interface implemented by a certificate authority (CA)
type accessPoint interface {
	// Announcer adds methods used to announce presence
	Announcer
	// Streamer creates and manages audit streams
	events.Streamer

	// Semaphores provides semaphore operations
	types.Semaphores

	// UpsertTunnelConnection upserts tunnel connection
	UpsertTunnelConnection(conn types.TunnelConnection) error

	// DeleteTunnelConnection deletes tunnel connection
	DeleteTunnelConnection(clusterName, connName string) error

	// GenerateCertAuthorityCRL returns an empty CRL for a CA.
	GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error)
}

// ReadNodeAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentNode.
//
// NOTE: This interface must match the resources replicated in cache.ForNode.
type ReadNodeAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetNetworkRestrictions returns networking restrictions for restricted shell to enforce
	GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error)
}

// NodeAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by teleport.ComponentNode.
type NodeAccessPoint interface {
	// ReadNodeAccessPoint provides methods to read data
	ReadNodeAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadProxyAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
//
// NOTE: This interface must match the resources replicated in cache.ForProxy.
type ReadProxyAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// GetAppServers gets all application servers.
	//
	// DELETE IN 9.0. Deprecated, use GetApplicationServers.
	GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// GetApps returns all application resources.
	GetApps(ctx context.Context) ([]types.Application, error)

	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)

	// GetNetworkRestrictions returns networking restrictions for restricted shell to enforce
	GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error)

	// GetAppSession gets an application web session.
	GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error)

	// GetWebSession gets a web session for the given request
	GetWebSession(context.Context, types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken gets a web token for the given request
	GetWebToken(context.Context, types.GetWebTokenRequest) (types.WebToken, error)

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)

	// GetKubeServices returns a list of kubernetes services registered in the cluster
	GetKubeServices(context.Context) ([]types.Server, error)

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)

	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error)

	// GetWindowsDesktop returns a named windows desktop host.
	GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)
}

// ProxyAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
type ProxyAccessPoint interface {
	// ReadProxyAccessPoint provides methods to read data
	ReadProxyAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadRemoteProxyAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
//
// NOTE: This interface must match the resources replicated in cache.ForRemoteProxy.
type ReadRemoteProxyAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetAppServers gets all application servers.
	//
	// DELETE IN 9.0. Deprecated, use GetApplicationServers.
	GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)

	// GetKubeServices returns a list of kubernetes services registered in the cluster
	GetKubeServices(context.Context) ([]types.Server, error)

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)
}

// RemoteProxyAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
type RemoteProxyAccessPoint interface {
	// ReadRemoteProxyAccessPoint provides methods to read data
	ReadRemoteProxyAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadKubernetesAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentKube.
//
// NOTE: This interface must match the resources replicated in cache.ForKubernetes.
type ReadKubernetesAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetKubeServices returns a list of kubernetes services registered in the cluster
	GetKubeServices(context.Context) ([]types.Server, error)
}

// KubernetesAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentKube.
type KubernetesAccessPoint interface {
	// ReadKubernetesAccessPoint provides methods to read data
	ReadKubernetesAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadAppsAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentApp.
//
// NOTE: This interface must match the resources replicated in cache.ForApps.
type ReadAppsAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetApps returns all application resources.
	GetApps(ctx context.Context) ([]types.Application, error)

	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)
}

// AppsAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentApp.
type AppsAccessPoint interface {
	// ReadAppsAccessPoint provides methods to read data
	ReadAppsAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadDatabaseAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDatabase.
//
// NOTE: This interface must match the resources replicated in cache.ForDatabases.
type ReadDatabaseAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)

	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)
}

// DatabaseAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDatabase.
type DatabaseAccessPoint interface {
	// ReadDatabaseAccessPoint provides methods to read data
	ReadDatabaseAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadWindowsDesktopAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentWindowsDesktop.
//
// NOTE: This interface must match the resources replicated in cache.ForWindowsDesktop.
type ReadWindowsDesktopAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error)

	// GetWindowsDesktop returns a named windows desktop host.
	GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)
}

// WindowsDesktopAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentWindowsDesktop.
type WindowsDesktopAccessPoint interface {
	// ReadWindowsDesktopAccessPoint provides methods to read data
	ReadWindowsDesktopAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// AccessCache is a subset of the interface working on the certificate authorities
type AccessCache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// Cache is a subset of the auth interface handling
// access to the discovery API and static tokens
type Cache interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]types.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*types.Namespace, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// ListNodes returns a paginated list of registered servers for this cluster.
	ListNodes(ctx context.Context, req proto.ListNodesRequest) (nodes []types.Server, nextKey string, err error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetUsers returns a list of local users registered with this domain
	GetUsers(withSecrets bool) ([]types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetAppServers gets all application servers.
	//
	// DELETE IN 9.0. Deprecated, use GetApplicationServers.
	GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

	// GetApps returns all application resources.
	GetApps(ctx context.Context) ([]types.Application, error)

	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// GetAppSession gets an application web session.
	GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error)

	// GetWebSession gets a web session for the given request
	GetWebSession(context.Context, types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken gets a web token for the given request
	GetWebToken(context.Context, types.GetWebTokenRequest) (types.WebToken, error)

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)

	// GetKubeServices returns a list of kubernetes services registered in the cluster
	GetKubeServices(context.Context) ([]types.Server, error)

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)

	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)

	// GetNetworkRestrictions returns networking restrictions for restricted shell to enforce
	GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error)

	// GetWindowsDesktop returns a named windows desktop host.
	GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)

	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() (types.StaticTokens, error)

	// GetTokens returns all active (non-expired) provisioning tokens
	GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]types.ProvisionToken, error)

	// GetToken finds and returns token by ID
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// GetLock gets a lock by name.
	// NOTE: This method is intentionally available only for the auth server
	// cache, the other Teleport components should make use of
	// services.LockWatcher that provides the necessary freshness guarantees.
	GetLock(ctx context.Context, name string) (types.Lock, error)

	// GetLocks gets all/in-force locks that match at least one of the targets
	// when specified.
	// NOTE: This method is intentionally available only for the auth server
	// cache, the other Teleport components should make use of
	// services.LockWatcher that provides the necessary freshness guarantees.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (resources []types.ResourceWithLabels, nextKey string, err error)
}

type NodeWrapper struct {
	ReadNodeAccessPoint
	accessPoint
	NoCache NodeAccessPoint
}

func NewNodeWrapper(base NodeAccessPoint, cache ReadNodeAccessPoint) NodeAccessPoint {
	return &NodeWrapper{
		NoCache:             base,
		accessPoint:         base,
		ReadNodeAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *NodeWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadNodeAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type ProxyWrapper struct {
	ReadProxyAccessPoint
	accessPoint
	NoCache ProxyAccessPoint
}

func NewProxyWrapper(base ProxyAccessPoint, cache ReadProxyAccessPoint) ProxyAccessPoint {
	return &ProxyWrapper{
		NoCache:              base,
		accessPoint:          base,
		ReadProxyAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *ProxyWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadProxyAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type RemoteProxyWrapper struct {
	ReadRemoteProxyAccessPoint
	accessPoint
	NoCache RemoteProxyAccessPoint
}

func NewRemoteProxyWrapper(base RemoteProxyAccessPoint, cache ReadRemoteProxyAccessPoint) RemoteProxyAccessPoint {
	return &RemoteProxyWrapper{
		NoCache:                    base,
		accessPoint:                base,
		ReadRemoteProxyAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *RemoteProxyWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadRemoteProxyAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type KubernetesWrapper struct {
	ReadKubernetesAccessPoint
	accessPoint
	NoCache KubernetesAccessPoint
}

func NewKubernetesWrapper(base KubernetesAccessPoint, cache ReadKubernetesAccessPoint) KubernetesAccessPoint {
	return &KubernetesWrapper{
		NoCache:                   base,
		accessPoint:               base,
		ReadKubernetesAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *KubernetesWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadKubernetesAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type DatabaseWrapper struct {
	ReadDatabaseAccessPoint
	accessPoint
	NoCache DatabaseAccessPoint
}

func NewDatabaseWrapper(base DatabaseAccessPoint, cache ReadDatabaseAccessPoint) DatabaseAccessPoint {
	return &DatabaseWrapper{
		NoCache:                 base,
		accessPoint:             base,
		ReadDatabaseAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *DatabaseWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadDatabaseAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type AppsWrapper struct {
	ReadAppsAccessPoint
	accessPoint
	NoCache AppsAccessPoint
}

func NewAppsWrapper(base AppsAccessPoint, cache ReadAppsAccessPoint) AppsAccessPoint {
	return &AppsWrapper{
		NoCache:             base,
		accessPoint:         base,
		ReadAppsAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *AppsWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadAppsAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type WindowsDesktopWrapper struct {
	ReadWindowsDesktopAccessPoint
	accessPoint
	NoCache WindowsDesktopAccessPoint
}

func NewWindowsDesktopWrapper(base WindowsDesktopAccessPoint, cache ReadWindowsDesktopAccessPoint) WindowsDesktopAccessPoint {
	return &WindowsDesktopWrapper{
		NoCache:                       base,
		accessPoint:                   base,
		ReadWindowsDesktopAccessPoint: cache,
	}
}

// Close closes all associated resources
func (w *WindowsDesktopWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadWindowsDesktopAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

// NewRemoteProxyCachingAccessPoint returns new caching access point using
// access point policy
type NewRemoteProxyCachingAccessPoint func(clt ClientI, cacheName []string) (RemoteProxyAccessPoint, error)

// notImplementedMessage is the message to return for endpoints that are not
// implemented. This is due to how service interfaces are used with Teleport.
const notImplementedMessage = "not implemented: can only be called by auth locally"
