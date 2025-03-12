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

package authclient

import (
	"context"
	"io"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/gitserver"
	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrules "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// Announcer specifies interface responsible for announcing presence
type Announcer interface {
	// UpsertNode registers node presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error)

	// UpsertProxy registers proxy presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(ctx context.Context, s types.Server) error

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(ctx context.Context, s types.Server) error

	// UpsertKubernetesServer registers a kubernetes server
	UpsertKubernetesServer(context.Context, types.KubeServer) (*types.KeepAlive, error)

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (types.KeepAliver, error)

	// UpsertApplicationServer registers an application server.
	UpsertApplicationServer(context.Context, types.AppServer) (*types.KeepAlive, error)

	// UpsertDatabaseServer registers a database proxy server.
	UpsertDatabaseServer(context.Context, types.DatabaseServer) (*types.KeepAlive, error)

	// UpsertWindowsDesktopService registers a Windows desktop service.
	UpsertWindowsDesktopService(context.Context, types.WindowsDesktopService) (*types.KeepAlive, error)

	// UpsertWindowsDesktop registers a Windows desktop host.
	UpsertWindowsDesktop(context.Context, types.WindowsDesktop) error

	// UpsertDatabaseService registers a DatabaseService.
	UpsertDatabaseService(context.Context, types.DatabaseService) (*types.KeepAlive, error)
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

	// ConnectionDiagnosticTraceAppender adds a method to append traces into ConnectionDiagnostics.
	services.ConnectionDiagnosticTraceAppender
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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetUIConfig returns configuration for the UI served by the proxy service
	GetUIConfig(ctx context.Context) (types.UIConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

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
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)

	// GetKubernetesServers returns a list of kubernetes servers registered in the cluster
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)

	// ListKubernetesWaitingContainers lists Kubernetes ephemeral
	// containers that are waiting to be created until moderated
	// session conditions are met.
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)

	// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
	// container that are waiting to be created until moderated
	// session conditions are met.
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)

	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)
	// GetWindowsDesktopService returns a windows desktop host by name.
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)

	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error)
	// GetKubernetesCluster returns the specified kubernetes cluster resource.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)

	// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)

	// ListSAMLIdPServiceProviders returns a paginated list of all SAML IdP service provider resources.
	ListSAMLIdPServiceProviders(context.Context, int, string) ([]types.SAMLIdPServiceProvider, string, error)

	// GetSAMLIdPSession gets a SAML IdP session.
	GetSAMLIdPSession(context.Context, types.GetSAMLIdPSessionRequest) (types.WebSession, error)

	// ListUserGroups returns a paginated list of user group resources.
	ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error)

	// GetUserGroup returns the specified user group resources.
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)

	// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)

	// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)

	// GitServerReadOnlyClient returns the read-only client for Git servers.
	GitServerReadOnlyClient() gitserver.ReadOnlyClient
}

// SnowflakeSessionWatcher is watcher interface used by Snowflake web session watcher.
type SnowflakeSessionWatcher interface {
	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	// GetSnowflakeSession gets a Snowflake web session for a given request.
	GetSnowflakeSession(context.Context, types.GetSnowflakeSessionRequest) (types.WebSession, error)
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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)

	// GetKubernetesServers returns a list of kubernetes servers registered in the cluster
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)

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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetKubernetesServers returns a list of kubernetes servers registered in the cluster
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)

	// ListKubernetesWaitingContainers lists Kubernetes ephemeral
	// containers that are waiting to be created until moderated
	// session conditions are met.
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)

	// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
	// container that are waiting to be created until moderated
	// session conditions are met.
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)

	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error)
	// GetKubernetesCluster returns the specified kubernetes cluster resource.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

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
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)

	// GetWindowsDesktopService returns a windows desktop host by name.
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)
}

// WindowsDesktopAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentWindowsDesktop.
type WindowsDesktopAccessPoint interface {
	// ReadWindowsDesktopAccessPoint provides methods to read data
	ReadWindowsDesktopAccessPoint

	// accessPoint provides common access point functionality
	accessPoint
}

// ReadDiscoveryAccessPoint is a read only API interface to be
// used by a teleport.ComponentDiscovery.
//
// NOTE: This interface must match the resources replicated in cache.ForDiscovery.
type ReadDiscoveryAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
	// GetKubernetesCluster returns a kubernetes cluster resource identified by name.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error)
	// GetKubernetesServers returns all registered kubernetes servers.
	GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)
	// GetDatabase returns a database resource with the given name if it exists.
	GetDatabase(ctx context.Context, name string) (types.Database, error)

	// GetApps returns all application resources.
	GetApps(context.Context) ([]types.Application, error)
	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)

	// ListDiscoveryConfigs returns a paginated list of Discovery Config resources.
	ListDiscoveryConfigs(ctx context.Context, pageSize int, nextKey string) ([]*discoveryconfig.DiscoveryConfig, string, error)

	// GetIntegration returns the specified integration resource.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)

	// GetUserTask gets a single User Task by its name.
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// DiscoveryAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDiscovery
type DiscoveryAccessPoint interface {
	// ReadDiscoveryAccessPoint provides methods to read data
	ReadDiscoveryAccessPoint

	// accessPoint provides common access point functionality
	accessPoint

	// CreateKubernetesCluster creates a new kubernetes cluster resource.
	CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error
	// UpdateKubernetesCluster updates existing kubernetes cluster resource.
	UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error
	// DeleteKubernetesCluster deletes specified kubernetes cluster resource.
	DeleteKubernetesCluster(ctx context.Context, name string) error

	// CreateDatabase creates a new database resource.
	CreateDatabase(ctx context.Context, database types.Database) error
	// UpdateDatabase updates an existing database resource.
	UpdateDatabase(ctx context.Context, database types.Database) error
	// DeleteDatabase deletes a database resource.
	DeleteDatabase(ctx context.Context, name string) error
	// UpsertServerInfo upserts a server info resource.
	UpsertServerInfo(ctx context.Context, si types.ServerInfo) error

	// CreateApp creates a new application resource.
	CreateApp(context.Context, types.Application) error
	// UpdateApp updates an existing application resource.
	UpdateApp(context.Context, types.Application) error
	// DeleteApp removes the specified application resource.
	DeleteApp(ctx context.Context, name string) error

	// SubmitUsageEvent submits an external usage event.
	SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error

	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)

	// GenerateAzureOIDCToken generates a token to be used to execute an Azure OIDC Integration action.
	GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error)

	// EnrollEKSClusters enrolls EKS clusters into Teleport by installing teleport-kube-agent chart on the clusters.
	EnrollEKSClusters(context.Context, *integrationpb.EnrollEKSClustersRequest, ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error)

	// Ping gets basic info about the auth server.
	Ping(context.Context) (proto.PingResponse, error)

	// UpdateDiscoveryConfigStatus updates the status of a discovery config.
	UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error)

	// UpsertUserTask creates or updates an User Task
	UpsertUserTask(ctx context.Context, req *usertasksv1.UserTask) (*usertasksv1.UserTask, error)
}

// ExpiryAccessPoint is the API used by the expiry service.
type ExpiryAccessPoint interface {
	// Semaphores provides semaphore operations
	types.Semaphores

	// ListAccessRequests is an access request getter with pagination and sorting options.
	ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error)

	// DeleteAccessRequest deletes an access request.
	DeleteAccessRequest(ctx context.Context, reqID string) error
}

// ReadOktaAccessPoint is a read only API interface to be
// used by an Okta component.
//
// NOTE: This interface must provide read interfaces for the [types.WatchKind] registered in [cache.ForOkta].
type ReadOktaAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	AccessCache

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetUsers returns a list of users with the cluster
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)

	// ListUserGroups returns a paginated list of all user group resources.
	ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error)

	// GetUserGroup returns the specified user group resources.
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)

	// ListOktaImportRules returns a paginated list of all Okta import rule resources.
	ListOktaImportRules(context.Context, int, string) ([]types.OktaImportRule, string, error)

	// GetOktaImportRule returns the specified Okta import rule resources.
	GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error)

	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)

	// GetOktaAssignment returns the specified Okta assignment resource.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	// GetLocks lists the locks that target a given set of resources.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)
}

// OktaAccessPoint is a read caching interface used by an Okta component.
type OktaAccessPoint interface {
	// ReadOktaAccessPoint provides methods to read data
	ReadOktaAccessPoint

	// accessPoint provides common access point functionality
	accessPoint

	// CreateUser creates a new user in the cluster
	CreateUser(ctx context.Context, user types.User) (types.User, error)

	// UpdateUser updates the given user record
	UpdateUser(ctx context.Context, user types.User) (types.User, error)

	// DeleteUser deletes the given user from the cluster
	DeleteUser(ctx context.Context, user string) error

	// CreateUserGroup creates a new user group resource.
	CreateUserGroup(context.Context, types.UserGroup) error

	// UpdateUserGroup updates an existing user group resource.
	UpdateUserGroup(context.Context, types.UserGroup) error

	// DeleteUserGroup removes the specified user group resource.
	DeleteUserGroup(ctx context.Context, name string) error

	// CreateOktaImportRule creates a new Okta import rule resource.
	CreateOktaImportRule(context.Context, types.OktaImportRule) (types.OktaImportRule, error)

	// UpdateOktaImportRule updates an existing Okta import rule resource.
	UpdateOktaImportRule(context.Context, types.OktaImportRule) (types.OktaImportRule, error)

	// DeleteOktaImportRule removes the specified Okta import rule resource.
	DeleteOktaImportRule(ctx context.Context, name string) error

	// CreateOktaAssignment creates a new Okta assignment resource.
	CreateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)

	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)

	// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
	// since the last transition.
	UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error

	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error

	// DeleteApplicationServer removes specified application server.
	DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error

	// UpsertLock creates or updates a given lock
	UpsertLock(ctx context.Context, lock types.Lock) error

	// DeleteLock deletes a given lock
	DeleteLock(ctx context.Context, name string) error
}

// AccessCache is a subset of the interface working on the certificate authorities
type AccessCache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

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
	GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error)

	// ListReverseTunnels returns a paginated list of reverse tunnels.
	ListReverseTunnels(ctx context.Context, pageSize int, pageToken string) ([]types.ReverseTunnel, string, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)

	// GetNodes returns a list of registered servers for this cluster.
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]types.Server, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// GetUser returns a services.User for this cluster.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// ListUsers returns a page of users.
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)

	// ListRoles is a paginated role getter.
	ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)

	// GetApps returns all application resources.
	GetApps(ctx context.Context) ([]types.Application, error)

	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)

	// GetApplicationServers returns all registered application servers.
	GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error)

	// GetAppSession gets an application web session.
	GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error)

	// ListAppSessions returns a page of application web sessions.
	ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error)

	// GetSnowflakeSession gets a Snowflake web session.
	GetSnowflakeSession(context.Context, types.GetSnowflakeSessionRequest) (types.WebSession, error)

	// GetSAMLIdPSession gets a SAML IdP session.
	GetSAMLIdPSession(context.Context, types.GetSAMLIdPSessionRequest) (types.WebSession, error)

	// GetWebSession gets a web session for the given request
	GetWebSession(context.Context, types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken gets a web token for the given request
	GetWebToken(context.Context, types.GetWebTokenRequest) (types.WebToken, error)

	// GetRemoteClusters returns a list of remote clusters
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)

	// GetKubernetesServers returns a list of kubernetes servers registered in the cluster
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)

	// ListKubernetesWaitingContainers lists Kubernetes ephemeral
	// containers that are waiting to be created until moderated
	// session conditions are met.
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)

	// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
	// container that are waiting to be created until moderated
	// session conditions are met.
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)

	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)

	// GetDatabases returns all database resources.
	GetDatabases(ctx context.Context) ([]types.Database, error)

	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)

	// GetNetworkRestrictions returns networking restrictions for restricted shell to enforce
	GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error)

	// GetWindowsDesktops returns windows desktop hosts.
	GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)

	// GetWindowsDesktopServices returns windows desktop hosts.
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)

	// GetWindowsDesktopService returns a windows desktop host by name.
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)

	// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
	GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error)

	// ListDynamicWindowsDesktops returns all registered dynamic Windows desktop.
	ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error)

	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() (types.StaticTokens, error)

	// GetTokens returns all active (non-expired) provisioning tokens
	GetTokens(ctx context.Context) ([]types.ProvisionToken, error)

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
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	// ListWindowsDesktops returns a paginated list of windows desktops.
	ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error)
	// ListWindowsDesktopServices returns a paginated list of windows desktops.
	ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error)

	// GetUIConfig gets the config for the UI served by the proxy service
	GetUIConfig(ctx context.Context) (types.UIConfig, error)

	// GetInstaller gets installer resource for this cluster
	GetInstaller(ctx context.Context, name string) (types.Installer, error)

	// GetInstallers gets all the installer resources.
	GetInstallers(ctx context.Context) ([]types.Installer, error)

	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error)
	// GetKubernetesCluster returns the specified kubernetes cluster resource.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)

	// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
	ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextKey string) ([]types.SAMLIdPServiceProvider, string, error)
	// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)

	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
	// GetOktaAssignment returns the specified Okta assignment resources.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)

	// ListUserGroups returns a paginated list of all user group resources.
	ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error)
	// GetUserGroup returns the specified user group resources.
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)

	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
	// ListAccessLists returns a paginated list of access lists.
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)

	// CountAccessListMembers will count all access list members.
	CountAccessListMembers(ctx context.Context, accessListName string) (users uint32, lists uint32, err error)
	// ListAccessListMembers returns a paginated list of all access list members.
	// May return a DynamicAccessListError if the requested access list has an
	// implicit member list and the underlying implementation does not have
	// enough information to compute the dynamic member list.
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// ListAllAccessListMembers returns a paginated list of all members of all access lists.
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// GetAccessListMember returns the specified access list member resource.
	// May return a DynamicAccessListError if the requested access list has an
	// implicit member list and the underlying implementation does not have
	// enough information to compute the dynamic member record.
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)

	// ListAccessListReviews will list access list reviews for a particular access list.
	ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error)

	// ListCrownJewels returns a paginated list of crown jewels.
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)

	// GetCrownJewel returns the specified crown jewel.
	GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error)

	// IntegrationsGetter defines read/list methods for integrations.
	services.IntegrationsGetter

	// GetUserTask returns the user tasks resource by name.
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
	// ListUserTasks returns the user tasks resources.
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error)

	// NotificationGetter defines list methods for notifications.
	services.NotificationGetter

	// ListAccessMonitoringRules returns a paginated list of access monitoring rules.
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrules.AccessMonitoringRule, string, error)
	// GetAccessMonitoringRule returns the specified access monitoring rule.
	GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrules.AccessMonitoringRule, error)
	// ListAccessMonitoringRulesWithFilter returns a paginated list of access monitoring rules.
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrules.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrules.AccessMonitoringRule, string, error)

	// DatabaseObjectsGetter defines methods for fetching database objects.
	services.DatabaseObjectsGetter

	// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)

	// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)

	// GetAccessGraphSettings returns the access graph settings.
	GetAccessGraphSettings(context.Context) (*clusterconfigpb.AccessGraphSettings, error)

	// GetSPIFFEFederation gets a SPIFFE Federation by name.
	GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error)
	// ListSPIFFEFederations lists all SPIFFE Federations using Google style
	// pagination.
	ListSPIFFEFederations(ctx context.Context, pageSize int, lastToken string) ([]*machineidv1.SPIFFEFederation, string, error)

	// GetWorkloadIdentity gets a WorkloadIdentity by name.
	GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error)
	// ListWorkloadIdentities lists all SPIFFE Federations using Google style
	// pagination.
	ListWorkloadIdentities(ctx context.Context, pageSize int, lastToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)

	// ListStaticHostUsers lists static host users.
	ListStaticHostUsers(ctx context.Context, pageSize int, startKey string) ([]*userprovisioningpb.StaticHostUser, string, error)
	// GetStaticHostUser returns a static host user by name.
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error)

	// GetProvisioningState gets a specific provisioning state
	GetProvisioningState(context.Context, services.DownstreamID, services.ProvisioningStateID) (*provisioningv1.PrincipalState, error)

	// GetAccountAssignment fetches specific IdentityCenter Account Assignment
	GetAccountAssignment(context.Context, services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error)

	// ListAccountAssignments fetches a paginated list of IdentityCenter Account Assignments
	ListAccountAssignments(context.Context, int, *pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error)

	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)

	// GitServerGetter defines methods for fetching Git servers.
	services.GitServerGetter
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

type DiscoveryWrapper struct {
	ReadDiscoveryAccessPoint
	accessPoint
	NoCache DiscoveryAccessPoint
}

func NewDiscoveryWrapper(base DiscoveryAccessPoint, cache ReadDiscoveryAccessPoint) DiscoveryAccessPoint {
	return &DiscoveryWrapper{
		NoCache:                  base,
		accessPoint:              base,
		ReadDiscoveryAccessPoint: cache,
	}
}

func (w *DiscoveryWrapper) CreateApp(ctx context.Context, app types.Application) error {
	return w.NoCache.CreateApp(ctx, app)
}

func (w *DiscoveryWrapper) UpdateApp(ctx context.Context, app types.Application) error {
	return w.NoCache.UpdateApp(ctx, app)
}

func (w *DiscoveryWrapper) DeleteApp(ctx context.Context, name string) error {
	return w.NoCache.DeleteApp(ctx, name)
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (w *DiscoveryWrapper) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	return w.NoCache.CreateKubernetesCluster(ctx, cluster)
}

// UpdateKubernetesCluster updates existing kubernetes cluster resource.
func (w *DiscoveryWrapper) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	return w.NoCache.UpdateKubernetesCluster(ctx, cluster)
}

// DeleteKubernetesCluster deletes specified kubernetes cluster resource.
func (w *DiscoveryWrapper) DeleteKubernetesCluster(ctx context.Context, name string) error {
	return w.NoCache.DeleteKubernetesCluster(ctx, name)
}

// CreateDatabase creates a new database resource.
func (w *DiscoveryWrapper) CreateDatabase(ctx context.Context, database types.Database) error {
	return w.NoCache.CreateDatabase(ctx, database)
}

// UpdateDatabase updates an existing database resource.
func (w *DiscoveryWrapper) UpdateDatabase(ctx context.Context, database types.Database) error {
	return w.NoCache.UpdateDatabase(ctx, database)
}

// DeleteDatabase deletes a database resource.
func (w *DiscoveryWrapper) DeleteDatabase(ctx context.Context, name string) error {
	return w.NoCache.DeleteDatabase(ctx, name)
}

// UpsertServerInfo upserts a server info resource.
func (w *DiscoveryWrapper) UpsertServerInfo(ctx context.Context, si types.ServerInfo) error {
	return w.NoCache.UpsertServerInfo(ctx, si)
}

// SubmitUsageEvent submits an external usage event.
func (w *DiscoveryWrapper) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	return w.NoCache.SubmitUsageEvent(ctx, req)
}

// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
func (w *DiscoveryWrapper) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	return w.NoCache.GenerateAWSOIDCToken(ctx, integration)
}

// GenerateAzureOIDCToken generates a token to be used to execute an Azure OIDC Integration action.
func (w *DiscoveryWrapper) GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error) {
	return w.NoCache.GenerateAzureOIDCToken(ctx, integration)
}

// EnrollEKSClusters enrolls EKS clusters into Teleport by installing teleport-kube-agent chart on the clusters.
func (w *DiscoveryWrapper) EnrollEKSClusters(ctx context.Context, req *integrationpb.EnrollEKSClustersRequest, _ ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
	return w.NoCache.EnrollEKSClusters(ctx, req)
}

// Ping gets basic info about the auth server.
func (w *DiscoveryWrapper) Ping(ctx context.Context) (proto.PingResponse, error) {
	return w.NoCache.Ping(ctx)
}

// UpdateDiscoveryConfigStatus updates the status of a discovery config.
func (w *DiscoveryWrapper) UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error) {
	return w.NoCache.UpdateDiscoveryConfigStatus(ctx, name, status)
}

// UpserUserTask creates or updates an User Task.
func (w *DiscoveryWrapper) UpsertUserTask(ctx context.Context, req *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	return w.NoCache.UpsertUserTask(ctx, req)
}

// Close closes all associated resources
func (w *DiscoveryWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadDiscoveryAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

type OktaWrapper struct {
	ReadOktaAccessPoint
	accessPoint
	NoCache OktaAccessPoint
}

func NewOktaWrapper(base OktaAccessPoint, cache ReadOktaAccessPoint) OktaAccessPoint {
	return &OktaWrapper{
		NoCache:             base,
		accessPoint:         base,
		ReadOktaAccessPoint: cache,
	}
}

// CreateUser creates a new user in the cluster
func (w *OktaWrapper) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	return w.NoCache.CreateUser(ctx, user)
}

// UpdateUser updates a user in the cluster
func (w *OktaWrapper) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	return w.NoCache.UpdateUser(ctx, user)
}

// DeleteUser removes a user from the cluster
func (w *OktaWrapper) DeleteUser(ctx context.Context, user string) error {
	return w.NoCache.DeleteUser(ctx, user)
}

// CreateUserGroup creates a new user group resource.
func (w *OktaWrapper) CreateUserGroup(ctx context.Context, userGroup types.UserGroup) error {
	return w.NoCache.CreateUserGroup(ctx, userGroup)
}

// UpdateUserGroup updates an existing user group resource.
func (w *OktaWrapper) UpdateUserGroup(ctx context.Context, userGroup types.UserGroup) error {
	return w.NoCache.UpdateUserGroup(ctx, userGroup)
}

// DeleteUserGroup removes the specified user group resource.
func (w *OktaWrapper) DeleteUserGroup(ctx context.Context, name string) error {
	return w.NoCache.DeleteUserGroup(ctx, name)
}

// CreateOktaImportRule creates a new Okta import rule resource.
func (w *OktaWrapper) CreateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	return w.NoCache.CreateOktaImportRule(ctx, importRule)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (w *OktaWrapper) UpdateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	return w.NoCache.UpdateOktaImportRule(ctx, importRule)
}

// DeleteOktaImportRule removes the specified Okta import rule resource.
func (w *OktaWrapper) DeleteOktaImportRule(ctx context.Context, name string) error {
	return w.NoCache.DeleteOktaImportRule(ctx, name)
}

// CreateOktaAssignment creates a new Okta assignment resource.
func (w *OktaWrapper) CreateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	return w.NoCache.CreateOktaAssignment(ctx, assignment)
}

// UpdateOktaAssignment updates an existing Okta assignment resource.
func (w *OktaWrapper) UpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	return w.NoCache.UpdateOktaAssignment(ctx, assignment)
}

// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
// since the last transition.
func (w *OktaWrapper) UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error {
	return w.NoCache.UpdateOktaAssignmentStatus(ctx, name, status, timeHasPassed)
}

// DeleteOktaAssignment removes the specified Okta assignment resource.
func (w *OktaWrapper) DeleteOktaAssignment(ctx context.Context, name string) error {
	return w.NoCache.DeleteOktaAssignment(ctx, name)
}

// DeleteApplicationServer removes specified application server.
func (w *OktaWrapper) DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error {
	return w.NoCache.DeleteApplicationServer(ctx, namespace, hostID, name)
}

// GetLocks fetches locks that target a given set of resources
func (w *OktaWrapper) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	return w.NoCache.GetLocks(ctx, inForceOnly, targets...)
}

// UpsertLock creates and/or updates lock resources
func (w *OktaWrapper) UpsertLock(ctx context.Context, lock types.Lock) error {
	return w.NoCache.UpsertLock(ctx, lock)
}

// DeleteLock deletes a lock by name
func (w *OktaWrapper) DeleteLock(ctx context.Context, name string) error {
	return w.NoCache.DeleteLock(ctx, name)
}

// Close closes all associated resources
func (w *OktaWrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadOktaAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

// NewRemoteProxyCachingAccessPoint returns new caching access point using
// access point policy
type NewRemoteProxyCachingAccessPoint func(clt ClientI, cacheName []string) (RemoteProxyAccessPoint, error)
