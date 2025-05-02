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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// legacyCollection is responsible for managing collection
// of resources updates
type legacyCollection interface {
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
type executor[T any, R any] interface {
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

// noReader is returned by getReader for resources which aren't directly used by the cache, and therefore have no associated reader.
type noReader struct{}

type userTasksGetter interface {
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error)
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// legacyCollections is a registry of resource collections used by Cache.
type legacyCollections struct {
	// byKind is a map of registered collections by resource Kind/SubKind
	byKind map[resourceKind]legacyCollection

	auditQueries                       collectionReader[services.SecurityAuditQueryGetter]
	secReports                         collectionReader[services.SecurityReportGetter]
	secReportsStates                   collectionReader[services.SecurityReportStateGetter]
	tunnelConnections                  collectionReader[tunnelConnectionGetter]
	databaseObjects                    collectionReader[services.DatabaseObjectsGetter]
	discoveryConfigs                   collectionReader[services.DiscoveryConfigsGetter]
	installers                         collectionReader[installerGetter]
	integrations                       collectionReader[services.IntegrationsGetter]
	userTasks                          collectionReader[userTasksGetter]
	kubeWaitingContainers              collectionReader[kubernetesWaitingContainerGetter]
	staticHostUsers                    collectionReader[staticHostUserGetter]
	locks                              collectionReader[services.LockGetter]
	networkRestrictions                collectionReader[networkRestrictionGetter]
	proxies                            collectionReader[services.ProxyGetter]
	remoteClusters                     collectionReader[remoteClusterGetter]
	uiConfigs                          collectionReader[uiConfigGetter]
	userLoginStates                    collectionReader[services.UserLoginStatesGetter]
	webTokens                          collectionReader[webTokenGetter]
	dynamicWindowsDesktops             collectionReader[dynamicWindowsDesktopsGetter]
	accessMonitoringRules              collectionReader[accessMonitoringRuleGetter]
	provisioningStates                 collectionReader[provisioningStateGetter]
	identityCenterPrincipalAssignments collectionReader[identityCenterPrincipalAssignmentGetter]
	pluginStaticCredentials            collectionReader[pluginStaticCredentialsGetter]
	gitServers                         collectionReader[services.GitServerGetter]
}

// setupLegacyCollections returns a registry of legacyCollections.
func setupLegacyCollections(c *Cache, watches []types.WatchKind) (*legacyCollections, error) {
	collections := &legacyCollections{
		byKind: make(map[resourceKind]legacyCollection, len(watches)),
	}
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
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
			collections.byKind[resourceKind] = &genericCollection[types.AccessRequest, noReader, accessRequestExecutor]{cache: c, watch: watch}

		case types.KindWebToken:
			if c.WebToken == nil {
				return nil, trace.BadParameter("missing parameter WebToken")
			}
			collections.webTokens = &genericCollection[types.WebToken, webTokenGetter, webTokenExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.webTokens
		case types.KindDatabaseObject:
			if c.DatabaseObjects == nil {
				return nil, trace.BadParameter("missing parameter DatabaseObject")
			}
			collections.databaseObjects = &genericCollection[*dbobjectv1.DatabaseObject, services.DatabaseObjectsGetter, databaseObjectExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.databaseObjects
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
		case types.KindDynamicWindowsDesktop:
			if c.WindowsDesktops == nil {
				return nil, trace.BadParameter("missing parameter DynamicWindowsDesktops")
			}
			collections.dynamicWindowsDesktops = &genericCollection[types.DynamicWindowsDesktop, dynamicWindowsDesktopsGetter, dynamicWindowsDesktopsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.dynamicWindowsDesktops
		case types.KindIntegration:
			if c.Integrations == nil {
				return nil, trace.BadParameter("missing parameter Integrations")
			}
			collections.integrations = &genericCollection[types.Integration, services.IntegrationsGetter, integrationsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.integrations
		case types.KindUserTask:
			if c.UserTasks == nil {
				return nil, trace.BadParameter("missing parameter user tasks")
			}
			collections.userTasks = &genericCollection[*usertasksv1.UserTask, userTasksGetter, userTasksExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.userTasks
		case types.KindDiscoveryConfig:
			if c.DiscoveryConfigs == nil {
				return nil, trace.BadParameter("missing parameter DiscoveryConfigs")
			}
			collections.discoveryConfigs = &genericCollection[*discoveryconfig.DiscoveryConfig, services.DiscoveryConfigsGetter, discoveryConfigExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.discoveryConfigs
		case types.KindHeadlessAuthentication:
			// For headless authentications, we need only process events. We don't need to keep the cache up to date.
			collections.byKind[resourceKind] = &genericCollection[*types.HeadlessAuthentication, noReader, noopExecutor]{cache: c, watch: watch}
		case types.KindAuditQuery:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter SecReports")
			}
			collections.auditQueries = &genericCollection[*secreports.AuditQuery, services.SecurityAuditQueryGetter, auditQueryExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.auditQueries
		case types.KindSecurityReport:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter KindSecurityReport")
			}
			collections.secReports = &genericCollection[*secreports.Report, services.SecurityReportGetter, secReportExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.secReports
		case types.KindSecurityReportState:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter KindSecurityReport")
			}
			collections.secReportsStates = &genericCollection[*secreports.ReportState, services.SecurityReportStateGetter, secReportStateExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.secReportsStates
		case types.KindUserLoginState:
			if c.UserLoginStates == nil {
				return nil, trace.BadParameter("missing parameter UserLoginStates")
			}
			collections.userLoginStates = &genericCollection[*userloginstate.UserLoginState, services.UserLoginStatesGetter, userLoginStateExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.userLoginStates
		case types.KindKubeWaitingContainer:
			if c.KubeWaitingContainers == nil {
				return nil, trace.BadParameter("missing parameter KubeWaitingContainers")
			}
			collections.kubeWaitingContainers = &genericCollection[*kubewaitingcontainerpb.KubernetesWaitingContainer, kubernetesWaitingContainerGetter, kubeWaitingContainerExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.kubeWaitingContainers
		case types.KindStaticHostUser:
			if c.StaticHostUsers == nil {
				return nil, trace.BadParameter("missing parameter StaticHostUsers")
			}
			collections.staticHostUsers = &genericCollection[*userprovisioningpb.StaticHostUser, staticHostUserGetter, staticHostUserExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.staticHostUsers
		case types.KindAccessMonitoringRule:
			if c.AccessMonitoringRules == nil {
				return nil, trace.BadParameter("missing parameter AccessMonitoringRule")
			}
			collections.accessMonitoringRules = &genericCollection[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleGetter, accessMonitoringRulesExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessMonitoringRules

		case types.KindProvisioningPrincipalState:
			if c.ProvisioningStates == nil {
				return nil, trace.BadParameter("missing parameter KindProvisioningState")
			}
			collections.provisioningStates = &genericCollection[*provisioningv1.PrincipalState, provisioningStateGetter, provisioningStateExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.provisioningStates
		case types.KindIdentityCenterPrincipalAssignment:
			if c.IdentityCenter == nil {
				return nil, trace.BadParameter("missing parameter IdentityCenter")
			}
			collections.identityCenterPrincipalAssignments = &genericCollection[
				*identitycenterv1.PrincipalAssignment,
				identityCenterPrincipalAssignmentGetter,
				identityCenterPrincipalAssignmentExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.identityCenterPrincipalAssignments

		case types.KindPluginStaticCredentials:
			if c.PluginStaticCredentials == nil {
				return nil, trace.BadParameter("missing parameter PluginStaticCredentials")
			}
			collections.pluginStaticCredentials = &genericCollection[
				types.PluginStaticCredentials,
				pluginStaticCredentialsGetter,
				pluginStaticCredentialsExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.pluginStaticCredentials

		case types.KindGitServer:
			if c.GitServers == nil {
				return nil, trace.BadParameter("missing parameter GitServers")
			}
			collections.gitServers = &genericCollection[
				types.Server,
				services.GitServerGetter,
				gitServerExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.gitServers
		default:
			if _, ok := c.collections.byKind[resourceKind]; !ok {
				return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
			}
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

func (accessRequestExecutor) getReader(_ *Cache, _ bool) noReader {
	return noReader{}
}

var _ executor[types.AccessRequest, noReader] = accessRequestExecutor{}

type tunnelConnectionExecutor struct{}

func (tunnelConnectionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.TunnelConnection, error) {
	return cache.Trust.GetAllTunnelConnections()
}

func (tunnelConnectionExecutor) upsert(ctx context.Context, cache *Cache, resource types.TunnelConnection) error {
	return cache.trustCache.UpsertTunnelConnection(resource)
}

func (tunnelConnectionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.trustCache.DeleteAllTunnelConnections()
}

func (tunnelConnectionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.trustCache.DeleteTunnelConnection(resource.GetSubKind(), resource.GetName())
}

func (tunnelConnectionExecutor) isSingleton() bool { return false }

func (tunnelConnectionExecutor) getReader(cache *Cache, cacheOK bool) tunnelConnectionGetter {
	if cacheOK {
		return cache.trustCache
	}
	return cache.Config.Trust
}

type tunnelConnectionGetter interface {
	GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error)
	GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error)
}

var _ executor[types.TunnelConnection, tunnelConnectionGetter] = tunnelConnectionExecutor{}

type remoteClusterExecutor struct{}

func (remoteClusterExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.RemoteCluster, error) {
	return cache.Trust.GetRemoteClusters(ctx)
}

func (remoteClusterExecutor) upsert(ctx context.Context, cache *Cache, resource types.RemoteCluster) error {
	err := cache.trustCache.DeleteRemoteCluster(ctx, resource.GetName())
	if err != nil {
		if !trace.IsNotFound(err) {
			cache.Logger.WarnContext(ctx, "Failed to delete remote cluster", "cluster", resource.GetName(), "error", err)
			return trace.Wrap(err)
		}
	}
	_, err = cache.trustCache.CreateRemoteCluster(ctx, resource)
	return trace.Wrap(err)
}

func (remoteClusterExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.trustCache.DeleteAllRemoteClusters(ctx)
}

func (remoteClusterExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.trustCache.DeleteRemoteCluster(ctx, resource.GetName())
}

func (remoteClusterExecutor) isSingleton() bool { return false }

func (remoteClusterExecutor) getReader(cache *Cache, cacheOK bool) remoteClusterGetter {
	if cacheOK {
		return cache.trustCache
	}
	return cache.Config.Trust
}

type remoteClusterGetter interface {
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
	ListRemoteClusters(ctx context.Context, pageSize int, pageToken string) ([]types.RemoteCluster, string, error)
}

var _ executor[types.RemoteCluster, remoteClusterGetter] = remoteClusterExecutor{}

type userExecutor struct{}

func (userExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.User, error) {
	return cache.Users.GetUsers(ctx, loadSecrets)
}

func (userExecutor) upsert(ctx context.Context, cache *Cache, resource types.User) error {
	_, err := cache.usersCache.UpsertUser(ctx, resource)
	return err
}

func (userExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.usersCache.DeleteAllUsers(ctx)
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
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}

var _ executor[types.User, userGetter] = userExecutor{}

type databaseObjectExecutor struct{}

func (databaseObjectExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*dbobjectv1.DatabaseObject, error) {
	var out []*dbobjectv1.DatabaseObject
	var nextToken string
	for {
		var page []*dbobjectv1.DatabaseObject
		var err error

		page, nextToken, err = cache.DatabaseObjects.ListDatabaseObjects(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (databaseObjectExecutor) upsert(ctx context.Context, cache *Cache, resource *dbobjectv1.DatabaseObject) error {
	_, err := cache.databaseObjectsCache.UpsertDatabaseObject(ctx, resource)
	return trace.Wrap(err)
}

func (databaseObjectExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.databaseObjectsCache.DeleteAllDatabaseObjects(ctx))
}

func (databaseObjectExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.databaseObjectsCache.DeleteDatabaseObject(ctx, resource.GetName()))
}

func (databaseObjectExecutor) isSingleton() bool { return false }

func (databaseObjectExecutor) getReader(cache *Cache, cacheOK bool) services.DatabaseObjectsGetter {
	if cacheOK {
		return cache.databaseObjectsCache
	}
	return cache.Config.DatabaseObjects
}

var _ executor[*dbobjectv1.DatabaseObject, services.DatabaseObjectsGetter] = databaseObjectExecutor{}

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

type dynamicWindowsDesktopsExecutor struct{}

func (dynamicWindowsDesktopsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.DynamicWindowsDesktop, error) {
	var desktops []types.DynamicWindowsDesktop
	next := ""
	for {
		d, token, err := cache.Config.DynamicWindowsDesktops.ListDynamicWindowsDesktops(ctx, defaults.MaxIterationLimit, next)
		if err != nil {
			return nil, err
		}
		desktops = append(desktops, d...)
		if token == "" {
			break
		}
		next = token
	}
	return desktops, nil
}

func (dynamicWindowsDesktopsExecutor) upsert(ctx context.Context, cache *Cache, resource types.DynamicWindowsDesktop) error {
	_, err := cache.dynamicWindowsDesktopsCache.UpsertDynamicWindowsDesktop(ctx, resource)
	return err
}

func (dynamicWindowsDesktopsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.dynamicWindowsDesktopsCache.DeleteAllDynamicWindowsDesktops(ctx)
}

func (dynamicWindowsDesktopsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.dynamicWindowsDesktopsCache.DeleteDynamicWindowsDesktop(ctx, resource.GetName())
}

func (dynamicWindowsDesktopsExecutor) isSingleton() bool { return false }

func (dynamicWindowsDesktopsExecutor) getReader(cache *Cache, cacheOK bool) dynamicWindowsDesktopsGetter {
	if cacheOK {
		return cache.dynamicWindowsDesktopsCache
	}
	return cache.Config.DynamicWindowsDesktops
}

type dynamicWindowsDesktopsGetter interface {
	GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error)
	ListDynamicWindowsDesktops(ctx context.Context, pageSize int, nextPage string) ([]types.DynamicWindowsDesktop, string, error)
}

var _ executor[types.DynamicWindowsDesktop, dynamicWindowsDesktopsGetter] = dynamicWindowsDesktopsExecutor{}

type kubeWaitingContainerExecutor struct{}

func (kubeWaitingContainerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	var (
		startKey string
		allConts []*kubewaitingcontainerpb.KubernetesWaitingContainer
	)
	for {
		conts, nextKey, err := cache.KubeWaitingContainers.ListKubernetesWaitingContainers(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allConts = append(allConts, conts...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return allConts, nil
}

func (kubeWaitingContainerExecutor) upsert(ctx context.Context, cache *Cache, resource *kubewaitingcontainerpb.KubernetesWaitingContainer) error {
	_, err := cache.kubeWaitingContsCache.UpsertKubernetesWaitingContainer(ctx, resource)
	return trace.Wrap(err)
}

func (kubeWaitingContainerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.kubeWaitingContsCache.DeleteAllKubernetesWaitingContainers(ctx))
}

func (kubeWaitingContainerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	switch r := resource.(type) {
	case types.Resource153UnwrapperT[*kubewaitingcontainerpb.KubernetesWaitingContainer]:
		wc := r.UnwrapT()
		err := cache.kubeWaitingContsCache.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
			Username:      wc.Spec.Username,
			Cluster:       wc.Spec.Cluster,
			Namespace:     wc.Spec.Namespace,
			PodName:       wc.Spec.PodName,
			ContainerName: wc.Spec.ContainerName,
		})
		return trace.Wrap(err)
	}

	return trace.BadParameter("unknown KubeWaitingContainer type, expected *kubewaitingcontainerpb.KubernetesWaitingContainer, got %T", resource)
}

func (kubeWaitingContainerExecutor) isSingleton() bool { return false }

func (kubeWaitingContainerExecutor) getReader(cache *Cache, cacheOK bool) kubernetesWaitingContainerGetter {
	if cacheOK {
		return cache.kubeWaitingContsCache
	}
	return cache.Config.KubeWaitingContainers
}

type kubernetesWaitingContainerGetter interface {
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)
}

var _ executor[*kubewaitingcontainerpb.KubernetesWaitingContainer, kubernetesWaitingContainerGetter] = kubeWaitingContainerExecutor{}

type staticHostUserExecutor struct{}

func (staticHostUserExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*userprovisioningpb.StaticHostUser, error) {
	var (
		startKey string
		allUsers []*userprovisioningpb.StaticHostUser
	)
	for {
		users, nextKey, err := cache.StaticHostUsers.ListStaticHostUsers(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allUsers = append(allUsers, users...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return allUsers, nil
}

func (staticHostUserExecutor) upsert(ctx context.Context, cache *Cache, resource *userprovisioningpb.StaticHostUser) error {
	_, err := cache.staticHostUsersCache.UpsertStaticHostUser(ctx, resource)
	return trace.Wrap(err)
}

func (staticHostUserExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.staticHostUsersCache.DeleteAllStaticHostUsers(ctx))
}

func (staticHostUserExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.staticHostUsersCache.DeleteStaticHostUser(ctx, resource.GetName()))
}

func (staticHostUserExecutor) isSingleton() bool { return false }

func (staticHostUserExecutor) getReader(cache *Cache, cacheOK bool) staticHostUserGetter {
	if cacheOK {
		return cache.staticHostUsersCache
	}
	return cache.Config.StaticHostUsers
}

type staticHostUserGetter interface {
	ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningpb.StaticHostUser, string, error)
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error)
}

type userTasksExecutor struct{}

func (userTasksExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*usertasksv1.UserTask, error) {
	var resources []*usertasksv1.UserTask
	var nextToken string
	for {
		var page []*usertasksv1.UserTask
		var err error
		page, nextToken, err = cache.UserTasks.ListUserTasks(ctx, 0 /* page size */, nextToken, &usertasksv1.ListUserTasksFilters{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, page...)

		if nextToken == "" {
			break
		}
	}
	return resources, nil
}

func (userTasksExecutor) upsert(ctx context.Context, cache *Cache, resource *usertasksv1.UserTask) error {
	_, err := cache.userTasksCache.UpsertUserTask(ctx, resource)
	return trace.Wrap(err)
}

func (userTasksExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.userTasksCache.DeleteAllUserTasks(ctx)
}

func (userTasksExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.userTasksCache.DeleteUserTask(ctx, resource.GetName())
}

func (userTasksExecutor) isSingleton() bool { return false }

func (userTasksExecutor) getReader(cache *Cache, cacheOK bool) userTasksGetter {
	if cacheOK {
		return cache.userTasksCache
	}
	return cache.Config.UserTasks
}

var _ executor[*usertasksv1.UserTask, userTasksGetter] = userTasksExecutor{}

// collectionReader extends the collection interface, adding routing capabilities.
type collectionReader[R any] interface {
	legacyCollection

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

type discoveryConfigExecutor struct{}

func (discoveryConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*discoveryconfig.DiscoveryConfig, error) {
	var discoveryConfigs []*discoveryconfig.DiscoveryConfig
	var nextToken string
	for {
		var page []*discoveryconfig.DiscoveryConfig
		var err error

		page, nextToken, err = cache.DiscoveryConfigs.ListDiscoveryConfigs(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		discoveryConfigs = append(discoveryConfigs, page...)

		if nextToken == "" {
			break
		}
	}
	return discoveryConfigs, nil
}

func (discoveryConfigExecutor) upsert(ctx context.Context, cache *Cache, resource *discoveryconfig.DiscoveryConfig) error {
	_, err := cache.discoveryConfigsCache.UpsertDiscoveryConfig(ctx, resource)
	return trace.Wrap(err)
}

func (discoveryConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.discoveryConfigsCache.DeleteAllDiscoveryConfigs(ctx)
}

func (discoveryConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.discoveryConfigsCache.DeleteDiscoveryConfig(ctx, resource.GetName())
}

func (discoveryConfigExecutor) isSingleton() bool { return false }

func (discoveryConfigExecutor) getReader(cache *Cache, cacheOK bool) services.DiscoveryConfigsGetter {
	if cacheOK {
		return cache.discoveryConfigsCache
	}
	return cache.Config.DiscoveryConfigs
}

var _ executor[*discoveryconfig.DiscoveryConfig, services.DiscoveryConfigsGetter] = discoveryConfigExecutor{}

type auditQueryExecutor struct{}

func (auditQueryExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.AuditQuery, error) {
	var out []*secreports.AuditQuery
	var nextToken string
	for {
		var page []*secreports.AuditQuery
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityAuditQueries(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (auditQueryExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.AuditQuery) error {
	err := cache.secReportsCache.UpsertSecurityAuditQuery(ctx, resource)
	return trace.Wrap(err)
}

func (auditQueryExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReports(ctx))
}

func (auditQueryExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityAuditQuery(ctx, resource.GetName()))
}

func (auditQueryExecutor) isSingleton() bool { return false }

func (auditQueryExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityAuditQueryGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.AuditQuery, services.SecurityAuditQueryGetter] = auditQueryExecutor{}

type secReportExecutor struct{}

func (secReportExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.Report, error) {
	var out []*secreports.Report
	var nextToken string
	for {
		var page []*secreports.Report
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityReports(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (secReportExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.Report) error {
	err := cache.secReportsCache.UpsertSecurityReport(ctx, resource)
	return trace.Wrap(err)
}

func (secReportExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReports(ctx))
}

func (secReportExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityReport(ctx, resource.GetName()))
}

func (secReportExecutor) isSingleton() bool { return false }

func (secReportExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityReportGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.Report, services.SecurityReportGetter] = secReportExecutor{}

type secReportStateExecutor struct{}

func (secReportStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.ReportState, error) {
	var out []*secreports.ReportState
	var nextToken string
	for {
		var page []*secreports.ReportState
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityReportsStates(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (secReportStateExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.ReportState) error {
	err := cache.secReportsCache.UpsertSecurityReportsState(ctx, resource)
	return trace.Wrap(err)
}

func (secReportStateExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReportsStates(ctx))
}

func (secReportStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityReportsState(ctx, resource.GetName()))
}

func (secReportStateExecutor) isSingleton() bool { return false }

func (secReportStateExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityReportStateGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.ReportState, services.SecurityReportStateGetter] = secReportStateExecutor{}

// noopExecutor can be used when a resource's events do not need to processed by
// the cache itself, only passed on to other watchers.
type noopExecutor struct{}

func (noopExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*types.HeadlessAuthentication, error) {
	return nil, nil
}

func (noopExecutor) upsert(ctx context.Context, cache *Cache, resource *types.HeadlessAuthentication) error {
	return nil
}

func (noopExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return nil
}

func (noopExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return nil
}

func (noopExecutor) isSingleton() bool { return false }

func (noopExecutor) getReader(_ *Cache, _ bool) noReader {
	return noReader{}
}

var _ executor[*types.HeadlessAuthentication, noReader] = noopExecutor{}

type userLoginStateExecutor struct{}

func (userLoginStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*userloginstate.UserLoginState, error) {
	resources, err := cache.UserLoginStates.GetUserLoginStates(ctx)
	return resources, trace.Wrap(err)
}

func (userLoginStateExecutor) upsert(ctx context.Context, cache *Cache, resource *userloginstate.UserLoginState) error {
	_, err := cache.userLoginStateCache.UpsertUserLoginState(ctx, resource)
	return trace.Wrap(err)
}

func (userLoginStateExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.userLoginStateCache.DeleteAllUserLoginStates(ctx)
}

func (userLoginStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.userLoginStateCache.DeleteUserLoginState(ctx, resource.GetName())
}

func (userLoginStateExecutor) isSingleton() bool { return false }

func (userLoginStateExecutor) getReader(cache *Cache, cacheOK bool) services.UserLoginStatesGetter {
	if cacheOK {
		return cache.userLoginStateCache
	}
	return cache.Config.UserLoginStates
}

var _ executor[*userloginstate.UserLoginState, services.UserLoginStatesGetter] = userLoginStateExecutor{}

type accessMonitoringRulesExecutor struct{}

func (accessMonitoringRulesExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	var resources []*accessmonitoringrulesv1.AccessMonitoringRule
	var nextToken string
	for {
		var page []*accessmonitoringrulesv1.AccessMonitoringRule
		var err error
		page, nextToken, err = cache.AccessMonitoringRules.ListAccessMonitoringRules(ctx, 0 /* page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, page...)

		if nextToken == "" {
			break
		}
	}
	return resources, nil
}

func (accessMonitoringRulesExecutor) upsert(ctx context.Context, cache *Cache, resource *accessmonitoringrulesv1.AccessMonitoringRule) error {
	_, err := cache.accessMontoringRuleCache.UpsertAccessMonitoringRule(ctx, resource)
	return trace.Wrap(err)
}

func (accessMonitoringRulesExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessMontoringRuleCache.DeleteAllAccessMonitoringRules(ctx)
}

func (accessMonitoringRulesExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessMontoringRuleCache.DeleteAccessMonitoringRule(ctx, resource.GetName())
}

func (accessMonitoringRulesExecutor) isSingleton() bool { return false }

func (accessMonitoringRulesExecutor) getReader(cache *Cache, cacheOK bool) accessMonitoringRuleGetter {
	if cacheOK {
		return cache.accessMontoringRuleCache
	}
	return cache.Config.AccessMonitoringRules
}

type accessMonitoringRuleGetter interface {
	GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}
