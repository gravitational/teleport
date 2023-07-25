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
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
)

// collection is responsible for managing collection
// of resources updates
type collection interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// processEvent processes event
	processEvent(ctx context.Context, e types.Event) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// executor[T, R] is a specific way to run the collector operations that we need
// for the genericCollector for a generic resource type T and its reader type R.
type executor[T types.Resource, R any] interface {
	// getAll returns all of the target resources from the auth server.
	// For singleton objects, this should be a size-1 slice.
	getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]T, error)

	// upsert will create or update a target resource in the cache.
	upsert(ctx context.Context, cache *Cache, value T) error

	// deleteAll will delete all target resources of the type in the cache.
	deleteAll(ctx context.Context, cache *Cache) error

	// delete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to deleteAll.
	delete(ctx context.Context, cache *Cache, resource types.Resource) error

	// isSingleton will return true if the target resource is a singleton.
	isSingleton() bool

	// getReader returns the appropriate reader type R based on the health status of the cache.
	// Reader type R provides getter methods related to the collection, e.g. GetNodes(), GetRoles().
	// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
	getReader(c *Cache, cacheOK bool) R
}

// genericCollection is a generic collection implementation for resource type T with collection-specific logic
// encapsulated in executor type E. Type R provides getter methods related to the collection, e.g. GetNodes(),
// GetRoles().
type genericCollection[T types.Resource, R any, E executor[T, R]] struct {
	cache *Cache
	watch types.WatchKind
	exec  E
}

// fetch implements collection
func (g *genericCollection[T, R, _]) fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error) {
	// Singleton objects will only get deleted or updated, not both
	deleteSingleton := false

	var resources []T
	if cacheOK {
		resources, err = g.exec.getAll(ctx, g.cache, g.watch.LoadSecrets)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			deleteSingleton = true
		}
	}

	return func(ctx context.Context) error {
		// Always perform the delete if this is not a singleton, otherwise
		// only perform the delete if the singleton wasn't found
		// or the resource kind isn't cached in the current generation.
		if !g.exec.isSingleton() || deleteSingleton || !cacheOK {
			if err := g.exec.deleteAll(ctx, g.cache); err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		}
		// If this is a singleton and we performed a deletion, return here
		// because we only want to update or delete a singleton, not both.
		// Also don't continue if the resource kind isn't cached in the current generation.
		if g.exec.isSingleton() && deleteSingleton || !cacheOK {
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
func (g *genericCollection[T, R, _]) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		if err := g.exec.delete(ctx, g.cache, event.Resource); err != nil {
			if !trace.IsNotFound(err) {
				g.cache.Logger.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(T)
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
func (g *genericCollection[T, R, _]) watchKind() types.WatchKind {
	return g.watch
}

var _ collection = (*genericCollection[types.Resource, any, executor[types.Resource, any]])(nil)

// genericCollection obtains the reader object from the executor based on the provided health status of the cache.
// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
func (c *genericCollection[T, R, _]) getReader(cacheOK bool) R {
	return c.exec.getReader(c.cache, cacheOK)
}

var _ collectionReader[any] = (*genericCollection[types.Resource, any, executor[types.Resource, any]])(nil)

// cacheCollections is a registry of resource collections used by Cache.
type cacheCollections struct {
	// byKind is a map of registered collections by resource Kind/SubKind
	byKind map[resourceKind]collection

	accessLists              collectionReader[services.AccessListsGetter]
	apps                     collectionReader[services.AppGetter]
	nodes                    collectionReader[nodeGetter]
	tunnelConnections        collectionReader[tunnelConnectionGetter]
	appSessions              collectionReader[appSessionGetter]
	appServers               collectionReader[appServerGetter]
	authPreferences          collectionReader[authPreferenceGetter]
	authServers              collectionReader[authServerGetter]
	certAuthorities          collectionReader[services.AuthorityGetter]
	clusterAuditConfigs      collectionReader[clusterAuditConfigGetter]
	clusterNames             collectionReader[clusterNameGetter]
	clusterNetworkingConfigs collectionReader[clusterNetworkingConfigGetter]
	databases                collectionReader[services.DatabaseGetter]
	databaseServers          collectionReader[databaseServerGetter]
	installers               collectionReader[installerGetter]
	integrations             collectionReader[services.IntegrationsGetter]
	kubeClusters             collectionReader[kubernetesClusterGetter]
	kubeServers              collectionReader[kubeServerGetter]
	locks                    collectionReader[services.LockGetter]
	namespaces               collectionReader[namespaceGetter]
	networkRestrictions      collectionReader[networkRestrictionGetter]
	oktaAssignments          collectionReader[oktaAssignmentGetter]
	oktaImportRules          collectionReader[oktaImportRuleGetter]
	proxies                  collectionReader[services.ProxyGetter]
	remoteClusters           collectionReader[remoteClusterGetter]
	reverseTunnels           collectionReader[reverseTunnelGetter]
	roles                    collectionReader[roleGetter]
	samlIdPServiceProviders  collectionReader[samlIdPServiceProviderGetter]
	samlIdPSessions          collectionReader[samlIdPSessionGetter]
	sessionRecordingConfigs  collectionReader[sessionRecordingConfigGetter]
	snowflakeSessions        collectionReader[snowflakeSessionGetter]
	staticTokens             collectionReader[staticTokensGetter]
	tokens                   collectionReader[tokenGetter]
	uiConfigs                collectionReader[uiConfigGetter]
	users                    collectionReader[userGetter]
	userGroups               collectionReader[userGroupGetter]
	webSessions              collectionReader[webSessionGetter]
	webTokens                collectionReader[webTokenGetter]
	windowsDesktops          collectionReader[windowsDesktopsGetter]
	windowsDesktopServices   collectionReader[windowsDesktopServiceGetter]
}

// setupCollections returns a registry of collections.
func setupCollections(c *Cache, watches []types.WatchKind) (*cacheCollections, error) {
	collections := &cacheCollections{
		byKind: make(map[resourceKind]collection, len(watches)),
	}
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindCertAuthority:
			if c.Trust == nil {
				return nil, trace.BadParameter("missing parameter Trust")
			}
			var filter types.CertAuthorityFilter
			filter.FromMap(watch.Filter)

			collections.certAuthorities = &genericCollection[types.CertAuthority, services.AuthorityGetter, certAuthorityExecutor]{
				cache: c,
				exec:  certAuthorityExecutor{filter: filter},
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.certAuthorities
		case types.KindStaticTokens:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.staticTokens = &genericCollection[types.StaticTokens, staticTokensGetter, staticTokensExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.staticTokens
		case types.KindToken:
			if c.Provisioner == nil {
				return nil, trace.BadParameter("missing parameter Provisioner")
			}
			collections.tokens = &genericCollection[types.ProvisionToken, tokenGetter, provisionTokenExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.tokens
		case types.KindClusterName:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.clusterNames = &genericCollection[types.ClusterName, clusterNameGetter, clusterNameExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.clusterNames
		case types.KindClusterAuditConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.clusterAuditConfigs = &genericCollection[types.ClusterAuditConfig, clusterAuditConfigGetter, clusterAuditConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.clusterAuditConfigs
		case types.KindClusterNetworkingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.clusterNetworkingConfigs = &genericCollection[types.ClusterNetworkingConfig, clusterNetworkingConfigGetter, clusterNetworkingConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.clusterNetworkingConfigs
		case types.KindClusterAuthPreference:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.authPreferences = &genericCollection[types.AuthPreference, authPreferenceGetter, authPreferenceExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.authPreferences
		case types.KindSessionRecordingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.sessionRecordingConfigs = &genericCollection[types.SessionRecordingConfig, sessionRecordingConfigGetter, sessionRecordingConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.sessionRecordingConfigs
		case types.KindInstaller:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.installers = &genericCollection[types.Installer, installerGetter, installerConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.installers
		case types.KindUIConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.uiConfigs = &genericCollection[types.UIConfig, uiConfigGetter, uiConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.uiConfigs
		case types.KindUser:
			if c.Users == nil {
				return nil, trace.BadParameter("missing parameter Users")
			}
			collections.users = &genericCollection[types.User, userGetter, userExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.users
		case types.KindRole:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections.roles = &genericCollection[types.Role, roleGetter, roleExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.roles
		case types.KindNamespace:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.namespaces = &genericCollection[*types.Namespace, namespaceGetter, namespaceExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.namespaces
		case types.KindNode:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.nodes = &genericCollection[types.Server, nodeGetter, nodeExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.nodes
		case types.KindProxy:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.proxies = &genericCollection[types.Server, services.ProxyGetter, proxyExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.proxies
		case types.KindAuthServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.authServers = &genericCollection[types.Server, authServerGetter, authServerExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.authServers
		case types.KindReverseTunnel:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.reverseTunnels = &genericCollection[types.ReverseTunnel, reverseTunnelGetter, reverseTunnelExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.reverseTunnels
		case types.KindTunnelConnection:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.tunnelConnections = &genericCollection[types.TunnelConnection, tunnelConnectionGetter, tunnelConnectionExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.tunnelConnections
		case types.KindRemoteCluster:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.remoteClusters = &genericCollection[types.RemoteCluster, remoteClusterGetter, remoteClusterExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.remoteClusters
		case types.KindAccessRequest:
			if c.DynamicAccess == nil {
				return nil, trace.BadParameter("missing parameter DynamicAccess")
			}
			// access request resources aren't directly used by Cache so there's no associated reader type
			collections.byKind[resourceKind] = &genericCollection[types.AccessRequest, any, accessRequestExecutor]{cache: c, watch: watch}
		case types.KindAppServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.appServers = &genericCollection[types.AppServer, appServerGetter, appServerExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.appServers
		case types.KindWebSession:
			switch watch.SubKind {
			case types.KindAppSession:
				if c.AppSession == nil {
					return nil, trace.BadParameter("missing parameter AppSession")
				}
				collections.appSessions = &genericCollection[types.WebSession, appSessionGetter, appSessionExecutor]{
					cache: c,
					watch: watch,
				}
				collections.byKind[resourceKind] = collections.appSessions
			case types.KindSnowflakeSession:
				if c.SnowflakeSession == nil {
					return nil, trace.BadParameter("missing parameter SnowflakeSession")
				}
				collections.snowflakeSessions = &genericCollection[types.WebSession, snowflakeSessionGetter, snowflakeSessionExecutor]{
					cache: c,
					watch: watch,
				}
				collections.byKind[resourceKind] = collections.snowflakeSessions
			case types.KindSAMLIdPSession:
				if c.SAMLIdPSession == nil {
					return nil, trace.BadParameter("missing parameter SAMLIdPSession")
				}
				collections.samlIdPSessions = &genericCollection[types.WebSession, samlIdPSessionGetter, samlIdPSessionExecutor]{
					cache: c,
					watch: watch,
				}
				collections.byKind[resourceKind] = collections.samlIdPSessions
			case types.KindWebSession:
				if c.WebSession == nil {
					return nil, trace.BadParameter("missing parameter WebSession")
				}
				collections.webSessions = &genericCollection[types.WebSession, webSessionGetter, webSessionExecutor]{
					cache: c,
					watch: watch,
				}
				collections.byKind[resourceKind] = collections.webSessions
			}
		case types.KindWebToken:
			if c.WebToken == nil {
				return nil, trace.BadParameter("missing parameter WebToken")
			}
			collections.webTokens = &genericCollection[types.WebToken, webTokenGetter, webTokenExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.webTokens
		case types.KindKubeServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.kubeServers = &genericCollection[types.KubeServer, kubeServerGetter, kubeServerExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.kubeServers
		case types.KindDatabaseServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.databaseServers = &genericCollection[types.DatabaseServer, databaseServerGetter, databaseServerExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.databaseServers
		case types.KindDatabaseService:
			if c.DatabaseServices == nil {
				return nil, trace.BadParameter("missing parameter DatabaseServices")
			}
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			// database service resources aren't directly used by Cache so there's no associated reader type
			collections.byKind[resourceKind] = &genericCollection[types.DatabaseService, any, databaseServiceExecutor]{cache: c, watch: watch}
		case types.KindApp:
			if c.Apps == nil {
				return nil, trace.BadParameter("missing parameter Apps")
			}
			collections.apps = &genericCollection[types.Application, services.AppGetter, appExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.apps
		case types.KindDatabase:
			if c.Databases == nil {
				return nil, trace.BadParameter("missing parameter Databases")
			}
			collections.databases = &genericCollection[types.Database, services.DatabaseGetter, databaseExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.databases
		case types.KindKubernetesCluster:
			if c.Kubernetes == nil {
				return nil, trace.BadParameter("missing parameter Kubernetes")
			}
			collections.kubeClusters = &genericCollection[types.KubeCluster, kubernetesClusterGetter, kubeClusterExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.kubeClusters
		case types.KindNetworkRestrictions:
			if c.Restrictions == nil {
				return nil, trace.BadParameter("missing parameter Restrictions")
			}
			collections.networkRestrictions = &genericCollection[types.NetworkRestrictions, networkRestrictionGetter, networkRestrictionsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.networkRestrictions
		case types.KindLock:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections.locks = &genericCollection[types.Lock, services.LockGetter, lockExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.locks
		case types.KindWindowsDesktopService:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections.windowsDesktopServices = &genericCollection[types.WindowsDesktopService, windowsDesktopServiceGetter, windowsDesktopServicesExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.windowsDesktopServices
		case types.KindWindowsDesktop:
			if c.WindowsDesktops == nil {
				return nil, trace.BadParameter("missing parameter WindowsDesktops")
			}
			collections.windowsDesktops = &genericCollection[types.WindowsDesktop, windowsDesktopsGetter, windowsDesktopsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.windowsDesktops
		case types.KindSAMLIdPServiceProvider:
			if c.SAMLIdPServiceProviders == nil {
				return nil, trace.BadParameter("missing parameter SAMLIdPServiceProviders")
			}
			collections.samlIdPServiceProviders = &genericCollection[types.SAMLIdPServiceProvider, samlIdPServiceProviderGetter, samlIdPServiceProvidersExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.samlIdPServiceProviders
		case types.KindUserGroup:
			if c.UserGroups == nil {
				return nil, trace.BadParameter("missing parameter UserGroups")
			}
			collections.userGroups = &genericCollection[types.UserGroup, userGroupGetter, userGroupsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.userGroups
		case types.KindOktaImportRule:
			if c.Okta == nil {
				return nil, trace.BadParameter("missing parameter Okta")
			}
			collections.oktaImportRules = &genericCollection[types.OktaImportRule, oktaImportRuleGetter, oktaImportRulesExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.oktaImportRules
		case types.KindOktaAssignment:
			if c.Okta == nil {
				return nil, trace.BadParameter("missing parameter Okta")
			}
			collections.oktaAssignments = &genericCollection[types.OktaAssignment, oktaAssignmentGetter, oktaAssignmentsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.oktaAssignments
		case types.KindIntegration:
			if c.Integrations == nil {
				return nil, trace.BadParameter("missing parameter Integrations")
			}
			collections.integrations = &genericCollection[types.Integration, services.IntegrationsGetter, integrationsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.integrations
		case types.KindAccessList:
			if c.AccessLists == nil {
				return nil, trace.BadParameter("missing parameter AccessLists")
			}
			collections.accessLists = &genericCollection[*accesslist.AccessList, services.AccessListsGetter, accessListsExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessLists
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
		}
	}
	return resourceKind{
		kind: wk.Kind,
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
	}
	return resourceKind{
		kind: res.GetKind(),
	}
}

type resourceKind struct {
	kind    string
	subkind string
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

func (accessRequestExecutor) getReader(_ *Cache, _ bool) any {
	// access request resources aren't directly used by Cache so there's no associated reader type
	return nil
}

var _ executor[types.AccessRequest, any] = accessRequestExecutor{}

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

func (tunnelConnectionExecutor) getReader(cache *Cache, cacheOK bool) tunnelConnectionGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type tunnelConnectionGetter interface {
	GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error)
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)
}

var _ executor[types.TunnelConnection, tunnelConnectionGetter] = tunnelConnectionExecutor{}

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

func (remoteClusterExecutor) getReader(cache *Cache, cacheOK bool) remoteClusterGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type remoteClusterGetter interface {
	GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error)
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)
}

var _ executor[types.RemoteCluster, remoteClusterGetter] = remoteClusterExecutor{}

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

func (reverseTunnelExecutor) getReader(cache *Cache, cacheOK bool) reverseTunnelGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type reverseTunnelGetter interface {
	GetReverseTunnels(ctx context.Context, opts ...services.MarshalOption) ([]types.ReverseTunnel, error)
}

var _ executor[types.ReverseTunnel, reverseTunnelGetter] = reverseTunnelExecutor{}

type proxyExecutor struct{}

func (proxyExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Server, error) {
	return cache.Presence.GetProxies()
}

func (proxyExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	return cache.presenceCache.UpsertProxy(ctx, resource)
}

func (proxyExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllProxies()
}

func (proxyExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteProxy(ctx, resource.GetName())
}

func (proxyExecutor) isSingleton() bool { return false }

func (proxyExecutor) getReader(cache *Cache, cacheOK bool) services.ProxyGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

var _ executor[types.Server, services.ProxyGetter] = proxyExecutor{}

type authServerExecutor struct{}

func (authServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Server, error) {
	return cache.Presence.GetAuthServers()
}

func (authServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	return cache.presenceCache.UpsertAuthServer(ctx, resource)
}

func (authServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllAuthServers()
}

func (authServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteAuthServer(resource.GetName())
}

func (authServerExecutor) isSingleton() bool { return false }

func (authServerExecutor) getReader(cache *Cache, cacheOK bool) authServerGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type authServerGetter interface {
	GetAuthServers() ([]types.Server, error)
}

var _ executor[types.Server, authServerGetter] = authServerExecutor{}

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

func (nodeExecutor) getReader(cache *Cache, cacheOK bool) nodeGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type nodeGetter interface {
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
}

var _ executor[types.Server, nodeGetter] = nodeExecutor{}

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

func (namespaceExecutor) getReader(cache *Cache, cacheOK bool) namespaceGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type namespaceGetter interface {
	GetNamespaces() ([]types.Namespace, error)
	GetNamespace(name string) (*types.Namespace, error)
}

var _ executor[*types.Namespace, namespaceGetter] = namespaceExecutor{}

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

func (certAuthorityExecutor) getReader(cache *Cache, cacheOK bool) services.AuthorityGetter {
	if cacheOK {
		return cache.trustCache
	}
	return cache.Config.Trust
}

var _ executor[types.CertAuthority, services.AuthorityGetter] = certAuthorityExecutor{}

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

func (staticTokensExecutor) getReader(cache *Cache, cacheOK bool) staticTokensGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type staticTokensGetter interface {
	GetStaticTokens() (types.StaticTokens, error)
}

var _ executor[types.StaticTokens, staticTokensGetter] = staticTokensExecutor{}

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

func (provisionTokenExecutor) getReader(cache *Cache, cacheOK bool) tokenGetter {
	if cacheOK {
		return cache.provisionerCache
	}
	return cache.Config.Provisioner
}

type tokenGetter interface {
	GetTokens(ctx context.Context) ([]types.ProvisionToken, error)
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)
}

var _ executor[types.ProvisionToken, tokenGetter] = provisionTokenExecutor{}

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

func (clusterNameExecutor) getReader(cache *Cache, cacheOK bool) clusterNameGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type clusterNameGetter interface {
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

var _ executor[types.ClusterName, clusterNameGetter] = clusterNameExecutor{}

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

func (userExecutor) getReader(cache *Cache, cacheOK bool) userGetter {
	if cacheOK {
		return cache.usersCache
	}
	return cache.Config.Users
}

type userGetter interface {
	GetUser(user string, withSecrets bool) (types.User, error)
	GetUsers(withSecrets bool) ([]types.User, error)
}

var _ executor[types.User, userGetter] = userExecutor{}

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

func (roleExecutor) getReader(cache *Cache, cacheOK bool) roleGetter {
	if cacheOK {
		return cache.accessCache
	}
	return cache.Config.Access
}

type roleGetter interface {
	GetRoles(ctx context.Context) ([]types.Role, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
}

var _ executor[types.Role, roleGetter] = roleExecutor{}

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

func (databaseServerExecutor) getReader(cache *Cache, cacheOK bool) databaseServerGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type databaseServerGetter interface {
	GetDatabaseServers(context.Context, string, ...services.MarshalOption) ([]types.DatabaseServer, error)
}

var _ executor[types.DatabaseServer, databaseServerGetter] = databaseServerExecutor{}

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

func (databaseServiceExecutor) getReader(_ *Cache, _ bool) any {
	// database service resources aren't directly used by Cache so there's no associated reader
	return nil
}

var _ executor[types.DatabaseService, any] = databaseServiceExecutor{}

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

func (databaseExecutor) getReader(cache *Cache, cacheOK bool) services.DatabaseGetter {
	if cacheOK {
		return cache.databasesCache
	}
	return cache.Config.Databases
}

var _ executor[types.Database, services.DatabaseGetter] = databaseExecutor{}

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

func (appExecutor) getReader(cache *Cache, cacheOK bool) services.AppGetter {
	if cacheOK {
		return cache.appsCache
	}
	return cache.Apps
}

func (appExecutor) isSingleton() bool { return false }

var _ executor[types.Application, services.AppGetter] = appExecutor{}

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

func (appServerExecutor) getReader(cache *Cache, cacheOK bool) appServerGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type appServerGetter interface {
	GetApplicationServers(context.Context, string) ([]types.AppServer, error)
}

var _ executor[types.AppServer, appServerGetter] = appServerExecutor{}

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

func (appSessionExecutor) getReader(cache *Cache, cacheOK bool) appSessionGetter {
	if cacheOK {
		return cache.appSessionCache
	}
	return cache.Config.AppSession
}

type appSessionGetter interface {
	GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error)
}

var _ executor[types.WebSession, appSessionGetter] = appSessionExecutor{}

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

func (snowflakeSessionExecutor) getReader(cache *Cache, cacheOK bool) snowflakeSessionGetter {
	if cacheOK {
		return cache.snowflakeSessionCache
	}
	return cache.Config.SnowflakeSession
}

type snowflakeSessionGetter interface {
	GetSnowflakeSession(context.Context, types.GetSnowflakeSessionRequest) (types.WebSession, error)
}

var _ executor[types.WebSession, snowflakeSessionGetter] = snowflakeSessionExecutor{}

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

func (samlIdPSessionExecutor) getReader(cache *Cache, cacheOK bool) samlIdPSessionGetter {
	if cacheOK {
		return cache.samlIdPSessionCache
	}
	return cache.Config.SAMLIdPSession
}

type samlIdPSessionGetter interface {
	GetSAMLIdPSession(context.Context, types.GetSAMLIdPSessionRequest) (types.WebSession, error)
}

var _ executor[types.WebSession, samlIdPSessionGetter] = samlIdPSessionExecutor{}

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

func (webSessionExecutor) getReader(cache *Cache, cacheOK bool) webSessionGetter {
	if cacheOK {
		return cache.webSessionCache
	}
	return cache.Config.WebSession
}

type webSessionGetter interface {
	Get(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error)
}

var _ executor[types.WebSession, webSessionGetter] = webSessionExecutor{}

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

func (webTokenExecutor) getReader(cache *Cache, cacheOK bool) webTokenGetter {
	if cacheOK {
		return cache.webTokenCache
	}
	return cache.Config.WebToken
}

type webTokenGetter interface {
	Get(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error)
}

var _ executor[types.WebToken, webTokenGetter] = webTokenExecutor{}

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

func (kubeServerExecutor) getReader(cache *Cache, cacheOK bool) kubeServerGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}

type kubeServerGetter interface {
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)
}

var _ executor[types.KubeServer, kubeServerGetter] = kubeServerExecutor{}

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

func (authPreferenceExecutor) getReader(cache *Cache, cacheOK bool) authPreferenceGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type authPreferenceGetter interface {
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
}

var _ executor[types.AuthPreference, authPreferenceGetter] = authPreferenceExecutor{}

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

func (clusterAuditConfigExecutor) getReader(cache *Cache, cacheOK bool) clusterAuditConfigGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type clusterAuditConfigGetter interface {
	GetClusterAuditConfig(context.Context, ...services.MarshalOption) (types.ClusterAuditConfig, error)
}

var _ executor[types.ClusterAuditConfig, clusterAuditConfigGetter] = clusterAuditConfigExecutor{}

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

func (clusterNetworkingConfigExecutor) getReader(cache *Cache, cacheOK bool) clusterNetworkingConfigGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type clusterNetworkingConfigGetter interface {
	GetClusterNetworkingConfig(context.Context, ...services.MarshalOption) (types.ClusterNetworkingConfig, error)
}

var _ executor[types.ClusterNetworkingConfig, clusterNetworkingConfigGetter] = clusterNetworkingConfigExecutor{}

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

func (uiConfigExecutor) getReader(cache *Cache, cacheOK bool) uiConfigGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type uiConfigGetter interface {
	GetUIConfig(context.Context) (types.UIConfig, error)
}

var _ executor[types.UIConfig, uiConfigGetter] = uiConfigExecutor{}

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

func (sessionRecordingConfigExecutor) getReader(cache *Cache, cacheOK bool) sessionRecordingConfigGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type sessionRecordingConfigGetter interface {
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)
}

var _ executor[types.SessionRecordingConfig, sessionRecordingConfigGetter] = sessionRecordingConfigExecutor{}

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

func (installerConfigExecutor) getReader(cache *Cache, cacheOK bool) installerGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type installerGetter interface {
	GetInstallers(context.Context) ([]types.Installer, error)
	GetInstaller(ctx context.Context, name string) (types.Installer, error)
}

var _ executor[types.Installer, installerGetter] = installerConfigExecutor{}

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

func (networkRestrictionsExecutor) getReader(cache *Cache, cacheOK bool) networkRestrictionGetter {
	if cacheOK {
		return cache.restrictionsCache
	}
	return cache.Config.Restrictions
}

type networkRestrictionGetter interface {
	GetNetworkRestrictions(context.Context) (types.NetworkRestrictions, error)
}

var _ executor[types.NetworkRestrictions, networkRestrictionGetter] = networkRestrictionsExecutor{}

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

func (lockExecutor) getReader(cache *Cache, cacheOK bool) services.LockGetter {
	if cacheOK {
		return cache.accessCache
	}
	return cache.Config.Access
}

var _ executor[types.Lock, services.LockGetter] = lockExecutor{}

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

func (windowsDesktopServicesExecutor) getReader(cache *Cache, cacheOK bool) windowsDesktopServiceGetter {
	if cacheOK {
		return windowsDesktopServiceAggregate{
			Presence:        cache.presenceCache,
			WindowsDesktops: cache.windowsDesktopsCache,
		}
	}
	return windowsDesktopServiceAggregate{
		Presence:        cache.Config.Presence,
		WindowsDesktops: cache.Config.WindowsDesktops,
	}
}

type windowsDesktopServiceAggregate struct {
	services.Presence
	services.WindowsDesktops
}

type windowsDesktopServiceGetter interface {
	GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error)
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)
	ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error)
}

var _ executor[types.WindowsDesktopService, windowsDesktopServiceGetter] = windowsDesktopServicesExecutor{}

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

func (windowsDesktopsExecutor) getReader(cache *Cache, cacheOK bool) windowsDesktopsGetter {
	if cacheOK {
		return cache.windowsDesktopsCache
	}
	return cache.Config.WindowsDesktops
}

type windowsDesktopsGetter interface {
	GetWindowsDesktops(context.Context, types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)
	ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error)
}

var _ executor[types.WindowsDesktop, windowsDesktopsGetter] = windowsDesktopsExecutor{}

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

func (kubeClusterExecutor) getReader(cache *Cache, cacheOK bool) kubernetesClusterGetter {
	if cacheOK {
		return cache.kubernetesCache
	}
	return cache.Config.Kubernetes
}

type kubernetesClusterGetter interface {
	GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error)
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
}

var _ executor[types.KubeCluster, kubernetesClusterGetter] = kubeClusterExecutor{}

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

func (samlIdPServiceProvidersExecutor) getReader(cache *Cache, cacheOK bool) samlIdPServiceProviderGetter {
	if cacheOK {
		return cache.samlIdPServiceProvidersCache
	}
	return cache.Config.SAMLIdPServiceProviders
}

type samlIdPServiceProviderGetter interface {
	ListSAMLIdPServiceProviders(context.Context, int, string) ([]types.SAMLIdPServiceProvider, string, error)
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)
}

var _ executor[types.SAMLIdPServiceProvider, samlIdPServiceProviderGetter] = samlIdPServiceProvidersExecutor{}

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

func (userGroupsExecutor) getReader(cache *Cache, cacheOK bool) userGroupGetter {
	if cacheOK {
		return cache.userGroupsCache
	}
	return cache.Config.UserGroups
}

type userGroupGetter interface {
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)
	ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error)
}

var _ executor[types.UserGroup, userGroupGetter] = userGroupsExecutor{}

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

func (oktaImportRulesExecutor) getReader(cache *Cache, cacheOK bool) oktaImportRuleGetter {
	if cacheOK {
		return cache.oktaCache
	}
	return cache.Config.Okta
}

type oktaImportRuleGetter interface {
	ListOktaImportRules(context.Context, int, string) ([]types.OktaImportRule, string, error)
	GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error)
}

var _ executor[types.OktaImportRule, oktaImportRuleGetter] = oktaImportRulesExecutor{}

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

func (oktaAssignmentsExecutor) getReader(cache *Cache, cacheOK bool) oktaAssignmentGetter {
	if cacheOK {
		return cache.oktaCache
	}
	return cache.Config.Okta
}

type oktaAssignmentGetter interface {
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
}

var _ executor[types.OktaAssignment, oktaAssignmentGetter] = oktaAssignmentsExecutor{}

// collectionReader extends the collection interface, adding routing capabilities.
type collectionReader[R any] interface {
	collection

	// getReader returns the appropriate reader type T based on the health status of the cache.
	// Reader type R provides getter methods related to the collection, e.g. GetNodes(), GetRoles().
	// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
	getReader(cacheOK bool) R
}

type resourceGetter interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

type integrationsExecutor struct{}

func (integrationsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Integration, error) {
	var (
		startKey  string
		resources []types.Integration
	)
	for {
		var igs []types.Integration
		var err error
		igs, startKey, err = cache.Integrations.ListIntegrations(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, igs...)

		if startKey == "" {
			break
		}
	}

	return resources, nil
}

func (integrationsExecutor) upsert(ctx context.Context, cache *Cache, resource types.Integration) error {
	_, err := cache.integrationsCache.CreateIntegration(ctx, resource)
	if trace.IsAlreadyExists(err) {
		_, err = cache.integrationsCache.UpdateIntegration(ctx, resource)
	}
	return trace.Wrap(err)
}

func (integrationsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.integrationsCache.DeleteAllIntegrations(ctx)
}

func (integrationsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.integrationsCache.DeleteIntegration(ctx, resource.GetName())
}

func (integrationsExecutor) isSingleton() bool { return false }

func (integrationsExecutor) getReader(cache *Cache, cacheOK bool) services.IntegrationsGetter {
	if cacheOK {
		return cache.integrationsCache
	}
	return cache.Config.Integrations
}

var _ executor[types.Integration, services.IntegrationsGetter] = integrationsExecutor{}

type accessListsExecutor struct{}

func (accessListsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*accesslist.AccessList, error) {
	resources, err := cache.accessListsCache.GetAccessLists(ctx)
	return resources, trace.Wrap(err)
}

func (accessListsExecutor) upsert(ctx context.Context, cache *Cache, resource *accesslist.AccessList) error {
	_, err := cache.accessListsCache.UpsertAccessList(ctx, resource)
	return trace.Wrap(err)
}

func (accessListsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessListsCache.DeleteAllAccessLists(ctx)
}

func (accessListsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessListsCache.DeleteAccessList(ctx, resource.GetName())
}

func (accessListsExecutor) isSingleton() bool { return false }

func (accessListsExecutor) getReader(cache *Cache, cacheOK bool) services.AccessListsGetter {
	if cacheOK {
		return cache.accessListsCache
	}
	return cache.Config.AccessLists
}

var _ executor[*accesslist.AccessList, services.AccessListsGetter] = accessListsExecutor{}
