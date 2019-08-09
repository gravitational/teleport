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

package auth

import (
	"context"

	"github.com/gravitational/teleport/lib/services"
)

// Announcer specifies interface responsible for announcing presence
type Announcer interface {
	// UpsertNode registers node presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(s services.Server) (*services.KeepAlive, error)

	// UpsertProxy registers proxy presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(s services.Server) error

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(s services.Server) error

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (services.KeepAliver, error)
}

// ReadAccessPoint is an API interface implemented by a certificate authority (CA)
type ReadAccessPoint interface {
	// GetReverseTunnels returns  a list of reverse tunnels
	GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error)

	// GetClusterConfig returns cluster level configuration.
	GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error)

	// GetNamespaces returns a list of namespaces
	GetNamespaces() ([]services.Namespace, error)

	// GetNamespace returns namespace by name
	GetNamespace(name string) (*services.Namespace, error)

	// GetServers returns a list of registered servers
	GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error)

	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]services.Server, error)

	// GetAuthServers returns a list of auth servers registered in the cluster
	GetAuthServers() ([]services.Server, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType services.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (services.User, error)

	// GetUsers returns a list of local users registered with this domain
	GetUsers(withSecrets bool) ([]services.User, error)

	// GetRole returns role by name
	GetRole(name string) (services.Role, error)

	// GetRoles returns a list of roles
	GetRoles() ([]services.Role, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...services.MarshalOption) ([]services.TunnelConnection, error)

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error)
}

// AccessPoint is an API interface implemented by a certificate authority (CA)
type AccessPoint interface {
	// ReadAccessPoint provides methods to read data
	ReadAccessPoint
	// Announcer adds methods used to announce presence
	Announcer

	// UpsertTunnelConnection upserts tunnel connection
	UpsertTunnelConnection(conn services.TunnelConnection) error

	// DeleteTunnelConnection deletes tunnel connection
	DeleteTunnelConnection(clusterName, connName string) error
}

// AccessCache is a subset of the interface working on the certificate authorities
type AccessCache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType services.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error)

	// GetClusterConfig returns cluster level configuration.
	GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error)
}

// AuthCache is a subset of the auth interface hanlding
// access to the discovery API and static tokens
type AuthCache interface {
	ReadAccessPoint

	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() (services.StaticTokens, error)

	// GetTokens returns all active (non-expired) provisioning tokens
	GetTokens(opts ...services.MarshalOption) ([]services.ProvisionToken, error)

	// GetToken finds and returns token by ID
	GetToken(token string) (services.ProvisionToken, error)

	// NewWatcher returns a new event watcher
	NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error)
}

// NewWrapper returns new access point wrapper
func NewWrapper(writer AccessPoint, cache ReadAccessPoint) AccessPoint {
	return &Wrapper{
		Write:           writer,
		ReadAccessPoint: cache,
	}
}

// Wrapper wraps access point and auth cache in one client
// so that update operations are going through access point
// and read operations are going though cache
type Wrapper struct {
	ReadAccessPoint
	Write AccessPoint
}

// UpsertNode is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertNode(s services.Server) (*services.KeepAlive, error) {
	return w.Write.UpsertNode(s)
}

// UpsertAuthServer is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertAuthServer(s services.Server) error {
	return w.Write.UpsertAuthServer(s)
}

// NewKeepAliver returns a new instance of keep aliver
func (w *Wrapper) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
	return w.Write.NewKeepAliver(ctx)
}

// UpsertProxy is part of auth.AccessPoint implementation
func (w *Wrapper) UpsertProxy(s services.Server) error {
	return w.Write.UpsertProxy(s)
}

// UpsertTunnelConnection is a part of auth.AccessPoint implementation
func (w *Wrapper) UpsertTunnelConnection(conn services.TunnelConnection) error {
	return w.Write.UpsertTunnelConnection(conn)
}

// DeleteTunnelConnection is a part of auth.AccessPoint implementation
func (w *Wrapper) DeleteTunnelConnection(clusterName, connName string) error {
	return w.Write.DeleteTunnelConnection(clusterName, connName)
}

// NewCachingAcessPoint returns new caching access point using
// access point policy
type NewCachingAccessPoint func(clt ClientI, cacheName []string) (AccessPoint, error)

// NoCache is a no cache used for access point
func NoCache(clt ClientI, cacheName []string) (AccessPoint, error) {
	return clt, nil
}
