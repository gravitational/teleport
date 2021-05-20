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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

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
	UpsertAppServer(context.Context, types.Server) (*types.KeepAlive, error)

	// UpsertDatabaseServer registers a database proxy server.
	UpsertDatabaseServer(context.Context, types.DatabaseServer) (*types.KeepAlive, error)
}

// ReadAccessPoint is an API interface implemented by a certificate authority (CA)
type ReadAccessPoint interface {
	// Closer closes all the resources
	io.Closer

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterConfig returns cluster level configuration.
	GetClusterConfig(opts ...services.MarshalOption) (types.ClusterConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference() (types.AuthPreference, error)

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
	GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)

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
}

// AccessPoint is an API interface implemented by a certificate authority (CA)
type AccessPoint interface {
	// ReadAccessPoint provides methods to read data
	ReadAccessPoint
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
}

// AccessCache is a subset of the interface working on the certificate authorities
type AccessCache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterConfig returns cluster level configuration.
	GetClusterConfig(opts ...services.MarshalOption) (types.ClusterConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// Cache is a subset of the auth interface hanlding
// access to the discovery API and static tokens
type Cache interface {
	ReadAccessPoint

	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() (types.StaticTokens, error)

	// GetTokens returns all active (non-expired) provisioning tokens
	GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]types.ProvisionToken, error)

	// GetToken finds and returns token by ID
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// NewWatcher returns a new event watcher
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// NewWrapper returns new access point wrapper
func NewWrapper(base AccessPoint, cache ReadAccessPoint) AccessPoint {
	return &Wrapper{
		NoCache:         base,
		ReadAccessPoint: cache,
	}
}

// Wrapper wraps access point and auth cache in one client
// so that reads of cached values can be intercepted.
type Wrapper struct {
	ReadAccessPoint
	NoCache AccessPoint
}

// ResumeAuditStream resumes existing audit stream
func (w *Wrapper) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	return w.NoCache.ResumeAuditStream(ctx, sid, uploadID)
}

// CreateAuditStream creates new audit stream
func (w *Wrapper) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	return w.NoCache.CreateAuditStream(ctx, sid)
}

// Close closes all associated resources
func (w *Wrapper) Close() error {
	err := w.NoCache.Close()
	err2 := w.ReadAccessPoint.Close()
	return trace.NewAggregate(err, err2)
}

// UpsertNode is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	return w.NoCache.UpsertNode(ctx, s)
}

// UpsertAuthServer is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertAuthServer(s types.Server) error {
	return w.NoCache.UpsertAuthServer(s)
}

// NewKeepAliver returns a new instance of keep aliver
func (w *Wrapper) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	return w.NoCache.NewKeepAliver(ctx)
}

// UpsertProxy is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertProxy(s types.Server) error {
	return w.NoCache.UpsertProxy(s)
}

// UpsertTunnelConnection is a part of auth.AccessPoint implementation
func (w *Wrapper) UpsertTunnelConnection(conn types.TunnelConnection) error {
	return w.NoCache.UpsertTunnelConnection(conn)
}

// DeleteTunnelConnection is a part of auth.AccessPoint implementation
func (w *Wrapper) DeleteTunnelConnection(clusterName, connName string) error {
	return w.NoCache.DeleteTunnelConnection(clusterName, connName)
}

// AcquireSemaphore acquires lease with requested resources from semaphore
func (w *Wrapper) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return w.NoCache.AcquireSemaphore(ctx, params)
}

// KeepAliveSemaphoreLease updates semaphore lease
func (w *Wrapper) KeepAliveSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return w.NoCache.KeepAliveSemaphoreLease(ctx, lease)
}

// CancelSemaphoreLease cancels semaphore lease early
func (w *Wrapper) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return w.NoCache.CancelSemaphoreLease(ctx, lease)
}

// GetSemaphores returns a list of semaphores matching supplied filter.
func (w *Wrapper) GetSemaphores(ctx context.Context, filter types.SemaphoreFilter) ([]types.Semaphore, error) {
	return w.NoCache.GetSemaphores(ctx, filter)
}

// DeleteSemaphore deletes a semaphore matching supplied filter.
func (w *Wrapper) DeleteSemaphore(ctx context.Context, filter types.SemaphoreFilter) error {
	return w.NoCache.DeleteSemaphore(ctx, filter)
}

// UpsertKubeService is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertKubeService(ctx context.Context, s types.Server) error {
	return w.NoCache.UpsertKubeService(ctx, s)
}

// UpsertAppServer adds an application server.
func (w *Wrapper) UpsertAppServer(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	return w.NoCache.UpsertAppServer(ctx, server)
}

// UpsertDatabaseServer registers a database proxy server.
func (w *Wrapper) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	return w.NoCache.UpsertDatabaseServer(ctx, server)
}

// NewCachingAcessPoint returns new caching access point using
// access point policy
type NewCachingAccessPoint func(clt ClientI, cacheName []string) (AccessPoint, error)

// NoCache is a no cache used for access point
func NoCache(clt ClientI, cacheName []string) (AccessPoint, error) {
	return clt, nil
}

// notImplementedMessage is the message to return for endpoints that are not
// implemented. This is due to how service interfaces are used with Teleport.
const notImplementedMessage = "not implemented: can only be called by auth locally"
