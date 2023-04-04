/*
Copyright 2018-2019 Gravitational, Inc.

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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

// collection is responsible for managing collection
// of resources updates
type collection interface {
	// fetch fetches resources and returns a function which
	// will apply said resources to the cache.  fetch *must*
	// not mutate cache state outside of the apply function.
	fetch(ctx context.Context) (apply func(ctx context.Context) error, err error)
	// processEvent processes event
	processEvent(ctx context.Context, e types.Event) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// executor[R] is a specific way to run the collector operations that we need
// for the genericCollector for a generic resource type R.
type executor[R types.Resource] interface {
	// getAll returns all of the target resources from the auth server.
	// For singleton objects, this should be a size-1 slice.
	getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]R, error)

	// upsert will create or update a target resource in the cache.
	upsert(ctx context.Context, cache *Cache, value R) error

	// deleteAll will delete all target resources of the type in the cache.
	deleteAll(ctx context.Context, cache *Cache) error

	// delete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to deleteAll.
	delete(ctx context.Context, cache *Cache, resource types.Resource) error

	// isSingleton will return true if the target resource is a singleton.
	isSingleton() bool
}

type genericCollection[R types.Resource, E executor[R]] struct {
	cache *Cache
	watch types.WatchKind
	exec  E
}

// fetch implements collection
func (g *genericCollection[_, _]) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	// Singleton objects will only get deleted or updated, not both
	deleteSingleton := false
	resources, err := g.exec.getAll(ctx, g.cache, g.watch.LoadSecrets)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		deleteSingleton = true
	}
	return func(ctx context.Context) error {
		// Always perform the delete if this is not a singleton, otherwise
		// only perform the delete if the singleton wasn't found.
		if !g.exec.isSingleton() || deleteSingleton {
			if err := g.exec.deleteAll(ctx, g.cache); err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		}
		// If this is a singleton and we performed a deletion, return here
		// because we only want to update or delete a singleton, not both.
		if g.exec.isSingleton() && deleteSingleton {
			return nil
		}
		for _, resource := range resources {
			if err := g.exec.upsert(ctx, g.cache, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

// processEvent implements collection
func (g *genericCollection[R, _]) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		if err := g.exec.delete(ctx, g.cache, event.Resource); err != nil {
			if !trace.IsNotFound(err) {
				g.cache.Logger.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(R)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := g.exec.upsert(ctx, g.cache, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		g.cache.Logger.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

// watchKind implements collection
func (g *genericCollection[_, _]) watchKind() types.WatchKind {
	return g.watch
}

var _ collection = (*genericCollection[types.Resource, executor[types.Resource]])(nil)

// setupCollections returns a mapping of collections
func setupCollections(c *Cache, watches []types.WatchKind) (map[resourceKind]collection, error) {
	collections := make(map[resourceKind]collection, len(watches))
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindCertAuthority:
			if c.Trust == nil {
				return nil, trace.BadParameter("missing parameter Trust")
			}
			var filter types.CertAuthorityFilter
			filter.FromMap(watch.Filter)
			collections[resourceKind] = &genericCollection[types.CertAuthority, certAuthorityExecutor]{
				cache: c,
				watch: watch,
				exec:  certAuthorityExecutor{filter: filter},
			}
		case types.KindStaticTokens:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.StaticTokens, staticTokensExecutor]{cache: c, watch: watch}
		case types.KindToken:
			if c.Provisioner == nil {
				return nil, trace.BadParameter("missing parameter Provisioner")
			}
			collections[resourceKind] = &genericCollection[types.ProvisionToken, provisionTokenExecutor]{cache: c, watch: watch}
		case types.KindClusterName:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.ClusterName, clusterNameExecutor]{cache: c, watch: watch}
		case types.KindClusterAuditConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.ClusterAuditConfig, clusterAuditConfigExecutor]{cache: c, watch: watch}
		case types.KindClusterNetworkingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.ClusterNetworkingConfig, clusterNetworkingConfigExecutor]{cache: c, watch: watch}
		case types.KindClusterAuthPreference:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.AuthPreference, authPreferenceExecutor]{cache: c, watch: watch}
		case types.KindSessionRecordingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.SessionRecordingConfig, sessionRecordingConfigExecutor]{cache: c, watch: watch}
		case types.KindInstaller:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.Installer, installerConfigExecutor]{cache: c, watch: watch}
		case types.KindUIConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &genericCollection[types.UIConfig, uiConfigExecutor]{cache: c, watch: watch}
		case types.KindUser:
			if c.Users == nil {
				return nil, trace.BadParameter("missing parameter Users")
			}
			collections[resourceKind] = &genericCollection[types.User, userExecutor]{cache: c, watch: watch}
		case types.KindRole:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[resourceKind] = &genericCollection[types.Role, roleExecutor]{cache: c, watch: watch}
		case types.KindNamespace:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[*types.Namespace, namespaceExecutor]{cache: c, watch: watch}
		case types.KindNode:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.Server, nodeExecutor]{cache: c, watch: watch}
		case types.KindProxy:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.Server, proxyExecutor]{cache: c, watch: watch}
		case types.KindAuthServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.Server, authServerExecutor]{cache: c, watch: watch}
		case types.KindReverseTunnel:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.ReverseTunnel, reverseTunnelExecutor]{cache: c, watch: watch}
		case types.KindTunnelConnection:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.TunnelConnection, tunnelConnectionExecutor]{cache: c, watch: watch}
		case types.KindRemoteCluster:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.RemoteCluster, remoteClusterExecutor]{cache: c, watch: watch}
		case types.KindAccessRequest:
			if c.DynamicAccess == nil {
				return nil, trace.BadParameter("missing parameter DynamicAccess")
			}
			collections[resourceKind] = &genericCollection[types.AccessRequest, accessRequestExecutor]{cache: c, watch: watch}
		case types.KindAppServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.AppServer, appServerExecutor]{cache: c, watch: watch}
		case types.KindWebSession:
			switch watch.SubKind {
			case types.KindAppSession:
				if c.AppSession == nil {
					return nil, trace.BadParameter("missing parameter AppSession")
				}
				collections[resourceKind] = &genericCollection[types.WebSession, appSessionExecutor]{cache: c, watch: watch}
			case types.KindSnowflakeSession:
				if c.SnowflakeSession == nil {
					return nil, trace.BadParameter("missing parameter SnowflakeSession")
				}
				collections[resourceKind] = &genericCollection[types.WebSession, snowflakeSessionExecutor]{cache: c, watch: watch}
			case types.KindSAMLIdPSession:
				if c.SAMLIdPSession == nil {
					return nil, trace.BadParameter("missing parameter SAMLIdPSession")
				}
				collections[resourceKind] = &genericCollection[types.WebSession, samlIdPSessionExecutor]{cache: c, watch: watch}
			case types.KindWebSession:
				if c.WebSession == nil {
					return nil, trace.BadParameter("missing parameter WebSession")
				}
				collections[resourceKind] = &genericCollection[types.WebSession, webSessionExecutor]{cache: c, watch: watch}
			}
		case types.KindWebToken:
			if c.WebToken == nil {
				return nil, trace.BadParameter("missing parameter WebToken")
			}
			collections[resourceKind] = &genericCollection[types.WebToken, webTokenExecutor]{cache: c, watch: watch}
		case types.KindKubeServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.KubeServer, kubeServerExecutor]{cache: c, watch: watch}
		case types.KindDatabaseServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.DatabaseServer, databaseServerExecutor]{cache: c, watch: watch}
		case types.KindDatabaseService:
			if c.DatabaseServices == nil {
				return nil, trace.BadParameter("missing parameter DatabaseServices")
			}
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.DatabaseService, databaseServiceExecutor]{cache: c, watch: watch}
		case types.KindApp:
			if c.Apps == nil {
				return nil, trace.BadParameter("missing parameter Apps")
			}
			collections[resourceKind] = &genericCollection[types.Application, appExecutor]{cache: c, watch: watch}
		case types.KindDatabase:
			if c.Databases == nil {
				return nil, trace.BadParameter("missing parameter Databases")
			}
			collections[resourceKind] = &genericCollection[types.Database, databaseExecutor]{cache: c, watch: watch}
		case types.KindKubernetesCluster:
			if c.Kubernetes == nil {
				return nil, trace.BadParameter("missing parameter Kubernetes")
			}
			collections[resourceKind] = &genericCollection[types.KubeCluster, kubeClusterExecutor]{cache: c, watch: watch}
		case types.KindNetworkRestrictions:
			if c.Restrictions == nil {
				return nil, trace.BadParameter("missing parameter Restrictions")
			}
			collections[resourceKind] = &genericCollection[types.NetworkRestrictions, networkRestrictionsExecutor]{cache: c, watch: watch}
		case types.KindLock:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[resourceKind] = &genericCollection[types.Lock, lockExecutor]{cache: c, watch: watch}
		case types.KindWindowsDesktopService:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &genericCollection[types.WindowsDesktopService, windowsDesktopServicesExecutor]{cache: c, watch: watch}
		case types.KindWindowsDesktop:
			if c.WindowsDesktops == nil {
				return nil, trace.BadParameter("missing parameter WindowsDesktops")
			}
			collections[resourceKind] = &genericCollection[types.WindowsDesktop, windowsDesktopsExecutor]{cache: c, watch: watch}
		case types.KindSAMLIdPServiceProvider:
			if c.SAMLIdPServiceProviders == nil {
				return nil, trace.BadParameter("missing parameter SAMLIdPServiceProviders")
			}
			collections[resourceKind] = &genericCollection[types.SAMLIdPServiceProvider, samlIdPServiceProvidersExecutor]{cache: c, watch: watch}
		case types.KindUserGroup:
			if c.UserGroups == nil {
				return nil, trace.BadParameter("missing parameter UserGroups")
			}
			collections[resourceKind] = &genericCollection[types.UserGroup, userGroupsExecutor]{cache: c, watch: watch}
		case types.KindOktaImportRule:
			if c.Okta == nil {
				return nil, trace.BadParameter("missing parameter Okta")
			}
			collections[resourceKind] = &genericCollection[types.OktaImportRule, oktaImportRulesExecutor]{cache: c, watch: watch}
		case types.KindOktaAssignment:
			if c.Okta == nil {
				return nil, trace.BadParameter("missing parameter Okta")
			}
			collections[resourceKind] = &genericCollection[types.OktaAssignment, oktaAssignmentsExecutor]{cache: c, watch: watch}
		default:
			return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
		}
	}
	return collections, nil
}

func resourceKindFromWatchKind(wk types.WatchKind) resourceKind {
	switch wk.Kind {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    wk.Kind,
			subkind: wk.SubKind,
			version: wk.Version,
		}
	}
	return resourceKind{
		kind:    wk.Kind,
		version: wk.Version,
	}
}

func resourceKindFromResource(res types.Resource) resourceKind {
	switch res.GetKind() {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    res.GetKind(),
			subkind: res.GetSubKind(),
		}
	case types.KindAppServer:
		// DELETE IN 9.0.
		switch res.GetVersion() {
		case types.V2:
			return resourceKind{
				kind:    res.GetKind(),
				version: res.GetVersion(),
			}
		}
	}
	return resourceKind{
		kind: res.GetKind(),
	}
}

type resourceKind struct {
	kind    string
	subkind string
	version string
}

func (r resourceKind) String() string {
	if r.subkind == "" {
		return r.kind
	}
	return fmt.Sprintf("%s/%s", r.kind, r.subkind)
}

type accessRequestExecutor struct{}

func (accessRequestExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.AccessRequest, error) {
	return cache.DynamicAccess.GetAccessRequests(ctx, types.AccessRequestFilter{})
}

func (accessRequestExecutor) upsert(ctx context.Context, cache *Cache, resource types.AccessRequest) error {
	return cache.dynamicAccessCache.UpsertAccessRequest(ctx, resource)
}

func (accessRequestExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.dynamicAccessCache.DeleteAllAccessRequests(ctx)
}

func (accessRequestExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.dynamicAccessCache.DeleteAccessRequest(ctx, resource.GetName())
}

func (accessRequestExecutor) isSingleton() bool { return false }

var _ executor[types.AccessRequest] = accessRequestExecutor{}

type tunnelConnectionExecutor struct{}

func (tunnelConnectionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.TunnelConnection, error) {
	return cache.Presence.GetAllTunnelConnections()
}

func (tunnelConnectionExecutor) upsert(ctx context.Context, cache *Cache, resource types.TunnelConnection) error {
	return cache.presenceCache.UpsertTunnelConnection(resource)
}

func (tunnelConnectionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllTunnelConnections()
}

func (tunnelConnectionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteTunnelConnection(resource.GetSubKind(), resource.GetName())
}

func (tunnelConnectionExecutor) isSingleton() bool { return false }

var _ executor[types.TunnelConnection] = tunnelConnectionExecutor{}

type remoteClusterExecutor struct{}

func (remoteClusterExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.RemoteCluster, error) {
	return cache.Presence.GetRemoteClusters()
}

func (remoteClusterExecutor) upsert(ctx context.Context, cache *Cache, resource types.RemoteCluster) error {
	err := cache.presenceCache.DeleteRemoteCluster(ctx, resource.GetName())
	if err != nil {
		if !trace.IsNotFound(err) {
			cache.Logger.WithError(err).Warnf("Failed to delete remote cluster %v.", resource.GetName())
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(cache.presenceCache.CreateRemoteCluster(resource))
}

func (remoteClusterExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllRemoteClusters()
}

func (remoteClusterExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteRemoteCluster(ctx, resource.GetName())
}

func (remoteClusterExecutor) isSingleton() bool { return false }

var _ executor[types.RemoteCluster] = remoteClusterExecutor{}

type reverseTunnelExecutor struct{}

func (reverseTunnelExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ReverseTunnel, error) {
	return cache.Presence.GetReverseTunnels(ctx)
}

func (reverseTunnelExecutor) upsert(ctx context.Context, cache *Cache, resource types.ReverseTunnel) error {
	return cache.presenceCache.UpsertReverseTunnel(resource)
}

func (reverseTunnelExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllReverseTunnels()
}

func (reverseTunnelExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteReverseTunnel(resource.GetName())
}

func (reverseTunnelExecutor) isSingleton() bool { return false }

var _ executor[types.ReverseTunnel] = reverseTunnelExecutor{}

type proxyExecutor struct{}

func (proxyExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Server, error) {
	return cache.Presence.GetProxies()
}

func (proxyExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	return cache.presenceCache.UpsertProxy(resource)
}

func (proxyExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllProxies()
}

func (proxyExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteProxy(resource.GetName())
}

func (proxyExecutor) isSingleton() bool { return false }

var _ executor[types.Server] = proxyExecutor{}

type authServerExecutor struct{}

func (authServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Server, error) {
	return cache.Presence.GetAuthServers()
}

func (authServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	return cache.presenceCache.UpsertAuthServer(resource)
}

func (authServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllAuthServers()
}

func (authServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteAuthServer(resource.GetName())
}

func (authServerExecutor) isSingleton() bool { return false }

var _ executor[types.Server] = authServerExecutor{}

type nodeExecutor struct{}

func (nodeExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Server, error) {
	return cache.Presence.GetNodes(ctx, apidefaults.Namespace)
}

func (nodeExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	_, err := cache.presenceCache.UpsertNode(ctx, resource)
	return trace.Wrap(err)
}

func (nodeExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllNodes(ctx, apidefaults.Namespace)
}

func (nodeExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteNode(ctx, resource.GetMetadata().Namespace, resource.GetName())
}

func (nodeExecutor) isSingleton() bool { return false }

var _ executor[types.Server] = nodeExecutor{}

type namespaceExecutor struct{}

func (namespaceExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*types.Namespace, error) {
	namespaces, err := cache.Presence.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	derefNamespaces := make([]*types.Namespace, len(namespaces))
	for i, namespace := range namespaces {
		derefNamespaces[i] = &namespace
	}
	return derefNamespaces, nil
}

func (namespaceExecutor) upsert(ctx context.Context, cache *Cache, resource *types.Namespace) error {
	return cache.presenceCache.UpsertNamespace(*resource)
}

func (namespaceExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllNamespaces()
}

func (namespaceExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteNamespace(resource.GetName())
}

func (namespaceExecutor) isSingleton() bool { return false }

var _ executor[*types.Namespace] = namespaceExecutor{}

type certAuthorityExecutor struct {
	// extracted from watch.Filter, to avoid rebuilding on every event
	filter types.CertAuthorityFilter
}

// delete implements executor[types.CertAuthority]
func (certAuthorityExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	err := cache.trustCache.DeleteCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(resource.GetSubKind()),
		DomainName: resource.GetName(),
	})
	return trace.Wrap(err)
}

// deleteAll implements executor[types.CertAuthority]
func (certAuthorityExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	for _, caType := range types.CertAuthTypes {
		if err := cache.trustCache.DeleteAllCertAuthorities(caType); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// getAll implements executor[types.CertAuthority]
func (e certAuthorityExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.CertAuthority, error) {
	var authorities []types.CertAuthority
	for _, caType := range types.CertAuthTypes {
		cas, err := cache.Trust.GetCertAuthorities(ctx, caType, loadSecrets)
		// if caType was added in this major version we might get a BadParameter
		// error if we're connecting to an older upstream that doesn't know about it
		if err != nil && !(caType.NewlyAdded() && trace.IsBadParameter(err)) {
			return nil, trace.Wrap(err)
		}

		// this can be removed once we get the ability to fetch CAs with a filter,
		// but it should be harmless, and it could be kept as additional safety
		if !e.filter.IsEmpty() {
			filtered := cas[:0]
			for _, ca := range cas {
				if e.filter.Match(ca) {
					filtered = append(filtered, ca)
				}
			}
			cas = filtered
		}

		authorities = append(authorities, cas...)
	}

	return authorities, nil
}

// upsert implements executor[types.CertAuthority]
func (e certAuthorityExecutor) upsert(ctx context.Context, cache *Cache, value types.CertAuthority) error {
	if !e.filter.Match(value) {
		return nil
	}

	return cache.trustCache.UpsertCertAuthority(ctx, value)
}

func (certAuthorityExecutor) isSingleton() bool { return false }

var _ executor[types.CertAuthority] = certAuthorityExecutor{}

type staticTokensExecutor struct{}

func (staticTokensExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.StaticTokens, error) {
	token, err := cache.ClusterConfig.GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.StaticTokens{token}, nil
}

func (staticTokensExecutor) upsert(ctx context.Context, cache *Cache, resource types.StaticTokens) error {
	return cache.clusterConfigCache.SetStaticTokens(resource)
}

func (staticTokensExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteStaticTokens()
}

func (staticTokensExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteStaticTokens()
}

func (staticTokensExecutor) isSingleton() bool { return true }

var _ executor[types.StaticTokens] = staticTokensExecutor{}

type provisionTokenExecutor struct{}

func (provisionTokenExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ProvisionToken, error) {
	return cache.Provisioner.GetTokens(ctx)
}

func (provisionTokenExecutor) upsert(ctx context.Context, cache *Cache, resource types.ProvisionToken) error {
	return cache.provisionerCache.UpsertToken(ctx, resource)
}

func (provisionTokenExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.provisionerCache.DeleteAllTokens()
}

func (provisionTokenExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.provisionerCache.DeleteToken(ctx, resource.GetName())
}

func (provisionTokenExecutor) isSingleton() bool { return false }

var _ executor[types.ProvisionToken] = provisionTokenExecutor{}

type clusterNameExecutor struct{}

func (clusterNameExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ClusterName, error) {
	name, err := cache.ClusterConfig.GetClusterName()
	return []types.ClusterName{name}, trace.Wrap(err)
}

func (clusterNameExecutor) upsert(ctx context.Context, cache *Cache, resource types.ClusterName) error {
	return cache.clusterConfigCache.UpsertClusterName(resource)
}

func (clusterNameExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteClusterName()
}

func (clusterNameExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteClusterName()
}

func (clusterNameExecutor) isSingleton() bool { return true }

var _ executor[types.ClusterName] = clusterNameExecutor{}

type userExecutor struct{}

func (userExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.User, error) {
	return cache.Users.GetUsers(loadSecrets)
}

func (userExecutor) upsert(ctx context.Context, cache *Cache, resource types.User) error {
	return cache.usersCache.UpsertUser(resource)
}

func (userExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.usersCache.DeleteAllUsers()
}

func (userExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.usersCache.DeleteUser(ctx, resource.GetName())
}

func (userExecutor) isSingleton() bool { return false }

var _ executor[types.User] = userExecutor{}

type roleExecutor struct{}

func (roleExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Role, error) {
	return cache.Access.GetRoles(ctx)
}

func (roleExecutor) upsert(ctx context.Context, cache *Cache, resource types.Role) error {
	return cache.accessCache.UpsertRole(ctx, resource)
}

func (roleExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessCache.DeleteAllRoles()
}

func (roleExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessCache.DeleteRole(ctx, resource.GetName())
}

func (roleExecutor) isSingleton() bool { return false }

var _ executor[types.Role] = roleExecutor{}

type databaseServerExecutor struct{}

func (databaseServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.DatabaseServer, error) {
	return cache.Presence.GetDatabaseServers(ctx, apidefaults.Namespace)
}

func (databaseServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.DatabaseServer) error {
	_, err := cache.presenceCache.UpsertDatabaseServer(ctx, resource)
	return trace.Wrap(err)
}

func (databaseServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
}

func (databaseServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteDatabaseServer(ctx,
		resource.GetMetadata().Namespace,
		resource.GetMetadata().Description, // Cache passes host ID via description field.
		resource.GetName())
}

func (databaseServerExecutor) isSingleton() bool { return false }

var _ executor[types.DatabaseServer] = databaseServerExecutor{}

type databaseServiceExecutor struct{}

func (databaseServiceExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.DatabaseService, error) {
	resources, err := client.GetResourcesWithFilters(ctx, cache.Presence, proto.ListResourcesRequest{ResourceType: types.KindDatabaseService})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbsvcs := make([]types.DatabaseService, len(resources))
	for i, resource := range resources {
		dbsvc, ok := resource.(types.DatabaseService)
		if !ok {
			return nil, trace.BadParameter("unexpected resource %T", resource)
		}
		dbsvcs[i] = dbsvc
	}

	return dbsvcs, nil
}

func (databaseServiceExecutor) upsert(ctx context.Context, cache *Cache, resource types.DatabaseService) error {
	_, err := cache.databaseServicesCache.UpsertDatabaseService(ctx, resource)
	return trace.Wrap(err)
}

func (databaseServiceExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.databaseServicesCache.DeleteAllDatabaseServices(ctx)
}

func (databaseServiceExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.databaseServicesCache.DeleteDatabaseService(ctx, resource.GetName())
}

func (databaseServiceExecutor) isSingleton() bool { return false }

var _ executor[types.DatabaseService] = databaseServiceExecutor{}

type databaseExecutor struct{}

func (databaseExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Database, error) {
	return cache.Databases.GetDatabases(ctx)
}

func (databaseExecutor) upsert(ctx context.Context, cache *Cache, resource types.Database) error {
	if err := cache.databasesCache.CreateDatabase(ctx, resource); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		return trace.Wrap(cache.databasesCache.UpdateDatabase(ctx, resource))
	}

	return nil
}

func (databaseExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.databasesCache.DeleteAllDatabases(ctx)
}

func (databaseExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.databasesCache.DeleteDatabase(ctx, resource.GetName())
}

func (databaseExecutor) isSingleton() bool { return false }

var _ executor[types.Database] = databaseExecutor{}

type appExecutor struct{}

func (appExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Application, error) {
	return cache.Apps.GetApps(ctx)
}

func (appExecutor) upsert(ctx context.Context, cache *Cache, resource types.Application) error {
	if err := cache.appsCache.CreateApp(ctx, resource); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		return trace.Wrap(cache.appsCache.UpdateApp(ctx, resource))
	}

	return nil
}

func (appExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.appsCache.DeleteAllApps(ctx)
}

func (appExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.appsCache.DeleteApp(ctx, resource.GetName())
}

func (appExecutor) isSingleton() bool { return false }

var _ executor[types.Application] = appExecutor{}

type appServerExecutor struct{}

func (appServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.AppServer, error) {
	return cache.Presence.GetApplicationServers(ctx, apidefaults.Namespace)
}

func (appServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.AppServer) error {
	_, err := cache.presenceCache.UpsertApplicationServer(ctx, resource)
	return trace.Wrap(err)
}

func (appServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
}

func (appServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteApplicationServer(ctx,
		resource.GetMetadata().Namespace,
		resource.GetMetadata().Description, // Cache passes host ID via description field.
		resource.GetName())
}

func (appServerExecutor) isSingleton() bool { return false }

var _ executor[types.AppServer] = appServerExecutor{}

type appSessionExecutor struct{}

func (appSessionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebSession, error) {
	var (
		startKey string
		sessions []types.WebSession
	)
	for {
		webSessions, nextKey, err := cache.AppSession.ListAppSessions(ctx, 0, startKey, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sessions = append(sessions, webSessions...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return sessions, nil
}

func (appSessionExecutor) upsert(ctx context.Context, cache *Cache, resource types.WebSession) error {
	return cache.appSessionCache.UpsertAppSession(ctx, resource)
}

func (appSessionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.appSessionCache.DeleteAllAppSessions(ctx)
}

func (appSessionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.appSessionCache.DeleteAppSession(ctx, types.DeleteAppSessionRequest{
		SessionID: resource.GetName(),
	})
}

func (appSessionExecutor) isSingleton() bool { return false }

var _ executor[types.WebSession] = appSessionExecutor{}

type snowflakeSessionExecutor struct{}

func (snowflakeSessionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebSession, error) {
	return cache.SnowflakeSession.GetSnowflakeSessions(ctx)
}

func (snowflakeSessionExecutor) upsert(ctx context.Context, cache *Cache, resource types.WebSession) error {
	return cache.snowflakeSessionCache.UpsertSnowflakeSession(ctx, resource)
}

func (snowflakeSessionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.snowflakeSessionCache.DeleteAllSnowflakeSessions(ctx)
}

func (snowflakeSessionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.snowflakeSessionCache.DeleteSnowflakeSession(ctx, types.DeleteSnowflakeSessionRequest{
		SessionID: resource.GetName(),
	})
}

func (snowflakeSessionExecutor) isSingleton() bool { return false }

var _ executor[types.WebSession] = snowflakeSessionExecutor{}

//nolint:revive // Because we want this to be IdP.
type samlIdPSessionExecutor struct{}

func (samlIdPSessionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebSession, error) {
	var (
		startKey string
		sessions []types.WebSession
	)
	for {
		webSessions, nextKey, err := cache.SAMLIdPSession.ListSAMLIdPSessions(ctx, 0, startKey, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sessions = append(sessions, webSessions...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return sessions, nil
}

func (samlIdPSessionExecutor) upsert(ctx context.Context, cache *Cache, resource types.WebSession) error {
	return cache.samlIdPSessionCache.UpsertSAMLIdPSession(ctx, resource)
}

func (samlIdPSessionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.samlIdPSessionCache.DeleteAllSAMLIdPSessions(ctx)
}

func (samlIdPSessionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.samlIdPSessionCache.DeleteSAMLIdPSession(ctx, types.DeleteSAMLIdPSessionRequest{
		SessionID: resource.GetName(),
	})
}

func (samlIdPSessionExecutor) isSingleton() bool { return false }

var _ executor[types.WebSession] = samlIdPSessionExecutor{}

type webSessionExecutor struct{}

func (webSessionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebSession, error) {
	return cache.WebSession.List(ctx)
}

func (webSessionExecutor) upsert(ctx context.Context, cache *Cache, resource types.WebSession) error {
	return cache.webSessionCache.Upsert(ctx, resource)
}

func (webSessionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.webSessionCache.DeleteAll(ctx)
}

func (webSessionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.webSessionCache.Delete(ctx, types.DeleteWebSessionRequest{
		SessionID: resource.GetName(),
	})
}

func (webSessionExecutor) isSingleton() bool { return false }

var _ executor[types.WebSession] = webSessionExecutor{}

type webTokenExecutor struct{}

func (webTokenExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebToken, error) {
	return cache.WebToken.List(ctx)
}

func (webTokenExecutor) upsert(ctx context.Context, cache *Cache, resource types.WebToken) error {
	return cache.webTokenCache.Upsert(ctx, resource)
}

func (webTokenExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.webTokenCache.DeleteAll(ctx)
}

func (webTokenExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.webTokenCache.Delete(ctx, types.DeleteWebTokenRequest{
		Token: resource.GetName(),
	})
}

func (webTokenExecutor) isSingleton() bool { return false }

var _ executor[types.WebToken] = webTokenExecutor{}

type kubeServerExecutor struct{}

func (kubeServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.KubeServer, error) {
	return cache.Presence.GetKubernetesServers(ctx)
}

func (kubeServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.KubeServer) error {
	_, err := cache.presenceCache.UpsertKubernetesServer(ctx, resource)
	return trace.Wrap(err)
}

func (kubeServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllKubernetesServers(ctx)
}

func (kubeServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteKubernetesServer(
		ctx,
		resource.GetMetadata().Description, // Cache passes host ID via description field.
		resource.GetName(),
	)
}

func (kubeServerExecutor) isSingleton() bool { return false }

var _ executor[types.KubeServer] = kubeServerExecutor{}

type authPreferenceExecutor struct{}

func (authPreferenceExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.AuthPreference, error) {
	authPref, err := cache.ClusterConfig.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.AuthPreference{authPref}, nil
}

func (authPreferenceExecutor) upsert(ctx context.Context, cache *Cache, resource types.AuthPreference) error {
	return cache.clusterConfigCache.SetAuthPreference(ctx, resource)
}

func (authPreferenceExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteAuthPreference(ctx)
}

func (authPreferenceExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteAuthPreference(ctx)
}

func (authPreferenceExecutor) isSingleton() bool { return true }

var _ executor[types.AuthPreference] = authPreferenceExecutor{}

type clusterAuditConfigExecutor struct{}

func (clusterAuditConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ClusterAuditConfig, error) {
	auditConfig, err := cache.ClusterConfig.GetClusterAuditConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.ClusterAuditConfig{auditConfig}, nil
}

func (clusterAuditConfigExecutor) upsert(ctx context.Context, cache *Cache, resource types.ClusterAuditConfig) error {
	return cache.clusterConfigCache.SetClusterAuditConfig(ctx, resource)
}

func (clusterAuditConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteClusterAuditConfig(ctx)
}

func (clusterAuditConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteClusterAuditConfig(ctx)
}

func (clusterAuditConfigExecutor) isSingleton() bool { return true }

var _ executor[types.ClusterAuditConfig] = clusterAuditConfigExecutor{}

type clusterNetworkingConfigExecutor struct{}

func (clusterNetworkingConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ClusterNetworkingConfig, error) {
	networkingConfig, err := cache.ClusterConfig.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.ClusterNetworkingConfig{networkingConfig}, nil
}

func (clusterNetworkingConfigExecutor) upsert(ctx context.Context, cache *Cache, resource types.ClusterNetworkingConfig) error {
	return cache.clusterConfigCache.SetClusterNetworkingConfig(ctx, resource)
}

func (clusterNetworkingConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteClusterNetworkingConfig(ctx)
}

func (clusterNetworkingConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteClusterNetworkingConfig(ctx)
}

func (clusterNetworkingConfigExecutor) isSingleton() bool { return true }

var _ executor[types.ClusterNetworkingConfig] = clusterNetworkingConfigExecutor{}

type uiConfigExecutor struct{}

func (uiConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.UIConfig, error) {
	uiConfig, err := cache.ClusterConfig.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.UIConfig{uiConfig}, nil
}

func (uiConfigExecutor) upsert(ctx context.Context, cache *Cache, resource types.UIConfig) error {
	return cache.clusterConfigCache.SetUIConfig(ctx, resource)
}

func (uiConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteUIConfig(ctx)
}

func (uiConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteUIConfig(ctx)
}

func (uiConfigExecutor) isSingleton() bool { return true }

var _ executor[types.UIConfig] = uiConfigExecutor{}

type sessionRecordingConfigExecutor struct{}

func (sessionRecordingConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.SessionRecordingConfig, error) {
	sessionRecordingConfig, err := cache.ClusterConfig.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.SessionRecordingConfig{sessionRecordingConfig}, nil
}

func (sessionRecordingConfigExecutor) upsert(ctx context.Context, cache *Cache, resource types.SessionRecordingConfig) error {
	return cache.clusterConfigCache.SetSessionRecordingConfig(ctx, resource)
}

func (sessionRecordingConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteSessionRecordingConfig(ctx)
}

func (sessionRecordingConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteSessionRecordingConfig(ctx)
}

func (sessionRecordingConfigExecutor) isSingleton() bool { return true }

var _ executor[types.SessionRecordingConfig] = sessionRecordingConfigExecutor{}

type installerConfigExecutor struct{}

func (installerConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Installer, error) {
	return cache.ClusterConfig.GetInstallers(ctx)
}

func (installerConfigExecutor) upsert(ctx context.Context, cache *Cache, resource types.Installer) error {
	return cache.clusterConfigCache.SetInstaller(ctx, resource)
}

func (installerConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.clusterConfigCache.DeleteAllInstallers(ctx)
}

func (installerConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.clusterConfigCache.DeleteInstaller(ctx, resource.GetName())
}

func (installerConfigExecutor) isSingleton() bool { return false }

var _ executor[types.Installer] = installerConfigExecutor{}

type networkRestrictionsExecutor struct{}

func (networkRestrictionsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.NetworkRestrictions, error) {
	restrictions, err := cache.Restrictions.GetNetworkRestrictions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.NetworkRestrictions{restrictions}, nil
}

func (networkRestrictionsExecutor) upsert(ctx context.Context, cache *Cache, resource types.NetworkRestrictions) error {
	return cache.restrictionsCache.SetNetworkRestrictions(ctx, resource)
}

func (networkRestrictionsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.restrictionsCache.DeleteNetworkRestrictions(ctx)
}

func (networkRestrictionsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.restrictionsCache.DeleteNetworkRestrictions(ctx)
}

func (networkRestrictionsExecutor) isSingleton() bool { return true }

var _ executor[types.NetworkRestrictions] = networkRestrictionsExecutor{}

type lockExecutor struct{}

func (lockExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Lock, error) {
	return cache.Access.GetLocks(ctx, false)
}

func (lockExecutor) upsert(ctx context.Context, cache *Cache, resource types.Lock) error {
	return cache.accessCache.UpsertLock(ctx, resource)
}

func (lockExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessCache.DeleteAllLocks(ctx)
}

func (lockExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessCache.DeleteLock(ctx, resource.GetName())
}

func (lockExecutor) isSingleton() bool { return false }

var _ executor[types.Lock] = lockExecutor{}

type windowsDesktopServicesExecutor struct{}

func (windowsDesktopServicesExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WindowsDesktopService, error) {
	return cache.Presence.GetWindowsDesktopServices(ctx)
}

func (windowsDesktopServicesExecutor) upsert(ctx context.Context, cache *Cache, resource types.WindowsDesktopService) error {
	_, err := cache.presenceCache.UpsertWindowsDesktopService(ctx, resource)
	return trace.Wrap(err)
}

func (windowsDesktopServicesExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllWindowsDesktopServices(ctx)
}

func (windowsDesktopServicesExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteWindowsDesktopService(ctx, resource.GetName())
}

func (windowsDesktopServicesExecutor) isSingleton() bool { return false }

var _ executor[types.WindowsDesktopService] = windowsDesktopServicesExecutor{}

type windowsDesktopsExecutor struct{}

func (windowsDesktopsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WindowsDesktop, error) {
	return cache.WindowsDesktops.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
}

func (windowsDesktopsExecutor) upsert(ctx context.Context, cache *Cache, resource types.WindowsDesktop) error {
	return cache.windowsDesktopsCache.UpsertWindowsDesktop(ctx, resource)
}

func (windowsDesktopsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.windowsDesktopsCache.DeleteAllWindowsDesktops(ctx)
}

func (windowsDesktopsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.windowsDesktopsCache.DeleteWindowsDesktop(ctx,
		resource.GetMetadata().Description, // Cache passes host ID via description field.
		resource.GetName(),
	)
}

func (windowsDesktopsExecutor) isSingleton() bool { return false }

var _ executor[types.WindowsDesktop] = windowsDesktopsExecutor{}

type kubeClusterExecutor struct{}

func (kubeClusterExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.KubeCluster, error) {
	return cache.Kubernetes.GetKubernetesClusters(ctx)
}

func (kubeClusterExecutor) upsert(ctx context.Context, cache *Cache, resource types.KubeCluster) error {
	if err := cache.kubernetesCache.CreateKubernetesCluster(ctx, resource); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		return trace.Wrap(cache.kubernetesCache.UpdateKubernetesCluster(ctx, resource))
	}

	return nil
}

func (kubeClusterExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.kubernetesCache.DeleteAllKubernetesClusters(ctx)
}

func (kubeClusterExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.kubernetesCache.DeleteKubernetesCluster(ctx, resource.GetName())
}

func (kubeClusterExecutor) isSingleton() bool { return false }

var _ executor[types.KubeCluster] = kubeClusterExecutor{}

//nolint:revive // Because we want this to be IdP.
type samlIdPServiceProvidersExecutor struct{}

func (samlIdPServiceProvidersExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.SAMLIdPServiceProvider, error) {
	var (
		startKey string
		sps      []types.SAMLIdPServiceProvider
	)
	for {
		var samlProviders []types.SAMLIdPServiceProvider
		var err error
		samlProviders, startKey, err = cache.SAMLIdPServiceProviders.ListSAMLIdPServiceProviders(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sps = append(sps, samlProviders...)

		if startKey == "" {
			break
		}
	}

	return sps, nil
}

func (samlIdPServiceProvidersExecutor) upsert(ctx context.Context, cache *Cache, resource types.SAMLIdPServiceProvider) error {
	err := cache.samlIdPServiceProvidersCache.CreateSAMLIdPServiceProvider(ctx, resource)
	if trace.IsAlreadyExists(err) {
		err = cache.samlIdPServiceProvidersCache.UpdateSAMLIdPServiceProvider(ctx, resource)
	}
	return trace.Wrap(err)
}

func (samlIdPServiceProvidersExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.samlIdPServiceProvidersCache.DeleteAllSAMLIdPServiceProviders(ctx)
}

func (samlIdPServiceProvidersExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.samlIdPServiceProvidersCache.DeleteSAMLIdPServiceProvider(ctx, resource.GetName())
}

func (samlIdPServiceProvidersExecutor) isSingleton() bool { return false }

var _ executor[types.SAMLIdPServiceProvider] = samlIdPServiceProvidersExecutor{}

type userGroupsExecutor struct{}

func (userGroupsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.UserGroup, error) {
	var (
		startKey  string
		resources []types.UserGroup
	)
	for {
		var userGroups []types.UserGroup
		var err error
		userGroups, startKey, err = cache.UserGroups.ListUserGroups(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, userGroups...)

		if startKey == "" {
			break
		}
	}

	return resources, nil
}

func (userGroupsExecutor) upsert(ctx context.Context, cache *Cache, resource types.UserGroup) error {
	err := cache.userGroupsCache.CreateUserGroup(ctx, resource)
	if trace.IsAlreadyExists(err) {
		err = cache.userGroupsCache.UpdateUserGroup(ctx, resource)
	}
	return trace.Wrap(err)
}

func (userGroupsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.userGroupsCache.DeleteAllUserGroups(ctx)
}

func (userGroupsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.userGroupsCache.DeleteUserGroup(ctx, resource.GetName())
}

func (userGroupsExecutor) isSingleton() bool { return false }

var _ executor[types.UserGroup] = userGroupsExecutor{}

type oktaImportRulesExecutor struct{}

func (oktaImportRulesExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.OktaImportRule, error) {
	var (
		startKey  string
		resources []types.OktaImportRule
	)
	for {
		var importRules []types.OktaImportRule
		var err error
		importRules, startKey, err = cache.Okta.ListOktaImportRules(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, importRules...)

		if startKey == "" {
			break
		}
	}

	return resources, nil
}

func (oktaImportRulesExecutor) upsert(ctx context.Context, cache *Cache, resource types.OktaImportRule) error {
	_, err := cache.oktaCache.CreateOktaImportRule(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.oktaCache.UpdateOktaImportRule(ctx, resource)
	}
	return trace.Wrap(err)
}

func (oktaImportRulesExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.oktaCache.DeleteAllOktaImportRules(ctx)
}

func (oktaImportRulesExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.oktaCache.DeleteOktaImportRule(ctx, resource.GetName())
}

func (oktaImportRulesExecutor) isSingleton() bool { return false }

var _ executor[types.OktaImportRule] = oktaImportRulesExecutor{}

type oktaAssignmentsExecutor struct{}

func (oktaAssignmentsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.OktaAssignment, error) {
	var (
		startKey  string
		resources []types.OktaAssignment
	)
	for {
		var assignments []types.OktaAssignment
		var err error
		assignments, startKey, err = cache.Okta.ListOktaAssignments(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, assignments...)

		if startKey == "" {
			break
		}
	}

	return resources, nil
}

func (oktaAssignmentsExecutor) upsert(ctx context.Context, cache *Cache, resource types.OktaAssignment) error {
	_, err := cache.oktaCache.CreateOktaAssignment(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.oktaCache.UpdateOktaAssignment(ctx, resource)
	}
	return trace.Wrap(err)
}

func (oktaAssignmentsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.oktaCache.DeleteAllOktaAssignments(ctx)
}

func (oktaAssignmentsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.oktaCache.DeleteOktaAssignment(ctx, resource.GetName())
}

func (oktaAssignmentsExecutor) isSingleton() bool { return false }

var _ executor[types.OktaAssignment] = oktaAssignmentsExecutor{}
