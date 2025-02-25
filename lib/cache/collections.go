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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/defaults"
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

type crownjewelsGetter interface {
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
	GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error)
}

type userTasksGetter interface {
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error)
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// cacheCollections is a registry of resource collections used by Cache.
type cacheCollections struct {
	// byKind is a map of registered collections by resource Kind/SubKind
	byKind map[resourceKind]collection

	auditQueries                       collectionReader[services.SecurityAuditQueryGetter]
	secReports                         collectionReader[services.SecurityReportGetter]
	secReportsStates                   collectionReader[services.SecurityReportStateGetter]
	accessLists                        collectionReader[accessListsGetter]
	accessListMembers                  collectionReader[accessListMembersGetter]
	accessListReviews                  collectionReader[accessListReviewsGetter]
	apps                               collectionReader[services.AppGetter]
	nodes                              collectionReader[nodeGetter]
	tunnelConnections                  collectionReader[tunnelConnectionGetter]
	appSessions                        collectionReader[appSessionGetter]
	appServers                         collectionReader[appServerGetter]
	authPreferences                    collectionReader[authPreferenceGetter]
	authServers                        collectionReader[authServerGetter]
	certAuthorities                    collectionReader[services.AuthorityGetter]
	clusterAuditConfigs                collectionReader[clusterAuditConfigGetter]
	clusterNames                       collectionReader[clusterNameGetter]
	clusterNetworkingConfigs           collectionReader[clusterNetworkingConfigGetter]
	databases                          collectionReader[services.DatabaseGetter]
	databaseObjects                    collectionReader[services.DatabaseObjectsGetter]
	databaseServers                    collectionReader[databaseServerGetter]
	discoveryConfigs                   collectionReader[services.DiscoveryConfigsGetter]
	installers                         collectionReader[installerGetter]
	integrations                       collectionReader[services.IntegrationsGetter]
	userTasks                          collectionReader[userTasksGetter]
	crownJewels                        collectionReader[crownjewelsGetter]
	kubeClusters                       collectionReader[kubernetesClusterGetter]
	kubeWaitingContainers              collectionReader[kubernetesWaitingContainerGetter]
	staticHostUsers                    collectionReader[staticHostUserGetter]
	kubeServers                        collectionReader[kubeServerGetter]
	locks                              collectionReader[services.LockGetter]
	networkRestrictions                collectionReader[networkRestrictionGetter]
	oktaAssignments                    collectionReader[oktaAssignmentGetter]
	oktaImportRules                    collectionReader[oktaImportRuleGetter]
	proxies                            collectionReader[services.ProxyGetter]
	remoteClusters                     collectionReader[remoteClusterGetter]
	reverseTunnels                     collectionReader[reverseTunnelGetter]
	roles                              collectionReader[roleGetter]
	samlIdPServiceProviders            collectionReader[samlIdPServiceProviderGetter]
	samlIdPSessions                    collectionReader[samlIdPSessionGetter]
	sessionRecordingConfigs            collectionReader[sessionRecordingConfigGetter]
	snowflakeSessions                  collectionReader[snowflakeSessionGetter]
	staticTokens                       collectionReader[staticTokensGetter]
	tokens                             collectionReader[tokenGetter]
	uiConfigs                          collectionReader[uiConfigGetter]
	users                              collectionReader[userGetter]
	userGroups                         collectionReader[userGroupGetter]
	userLoginStates                    collectionReader[services.UserLoginStatesGetter]
	webSessions                        collectionReader[webSessionGetter]
	webTokens                          collectionReader[webTokenGetter]
	windowsDesktops                    collectionReader[windowsDesktopsGetter]
	dynamicWindowsDesktops             collectionReader[dynamicWindowsDesktopsGetter]
	windowsDesktopServices             collectionReader[windowsDesktopServiceGetter]
	userNotifications                  collectionReader[notificationGetter]
	accessGraphSettings                collectionReader[accessGraphSettingsGetter]
	globalNotifications                collectionReader[notificationGetter]
	accessMonitoringRules              collectionReader[accessMonitoringRuleGetter]
	spiffeFederations                  collectionReader[SPIFFEFederationReader]
	autoUpdateConfigs                  collectionReader[autoUpdateConfigGetter]
	autoUpdateVersions                 collectionReader[autoUpdateVersionGetter]
	autoUpdateAgentRollouts            collectionReader[autoUpdateAgentRolloutGetter]
	provisioningStates                 collectionReader[provisioningStateGetter]
	identityCenterAccounts             collectionReader[identityCenterAccountGetter]
	identityCenterPrincipalAssignments collectionReader[identityCenterPrincipalAssignmentGetter]
	identityCenterAccountAssignments   collectionReader[identityCenterAccountAssignmentGetter]
	pluginStaticCredentials            collectionReader[pluginStaticCredentialsGetter]
	gitServers                         collectionReader[services.GitServerGetter]
	workloadIdentity                   collectionReader[WorkloadIdentityReader]
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
			collections.byKind[resourceKind] = &genericCollection[types.AccessRequest, noReader, accessRequestExecutor]{cache: c, watch: watch}
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
			collections.byKind[resourceKind] = &genericCollection[types.DatabaseService, noReader, databaseServiceExecutor]{cache: c, watch: watch}
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
		case types.KindDatabaseObject:
			if c.DatabaseObjects == nil {
				return nil, trace.BadParameter("missing parameter DatabaseObject")
			}
			collections.databaseObjects = &genericCollection[*dbobjectv1.DatabaseObject, services.DatabaseObjectsGetter, databaseObjectExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.databaseObjects
		case types.KindKubernetesCluster:
			if c.Kubernetes == nil {
				return nil, trace.BadParameter("missing parameter Kubernetes")
			}
			collections.kubeClusters = &genericCollection[types.KubeCluster, kubernetesClusterGetter, kubeClusterExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.kubeClusters
		case types.KindCrownJewel:
			if c.CrownJewels == nil {
				return nil, trace.BadParameter("missing parameter crownjewels")
			}
			collections.crownJewels = &genericCollection[*crownjewelv1.CrownJewel, crownjewelsGetter, crownJewelsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.crownJewels
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
		case types.KindDynamicWindowsDesktop:
			if c.WindowsDesktops == nil {
				return nil, trace.BadParameter("missing parameter DynamicWindowsDesktops")
			}
			collections.dynamicWindowsDesktops = &genericCollection[types.DynamicWindowsDesktop, dynamicWindowsDesktopsGetter, dynamicWindowsDesktopsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.dynamicWindowsDesktops
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
		case types.KindAccessList:
			if c.AccessLists == nil {
				return nil, trace.BadParameter("missing parameter AccessLists")
			}
			collections.accessLists = &genericCollection[*accesslist.AccessList, accessListsGetter, accessListExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessLists
		case types.KindAccessListMember:
			if c.AccessLists == nil {
				return nil, trace.BadParameter("missing parameter AccessLists")
			}
			collections.accessListMembers = &genericCollection[*accesslist.AccessListMember, accessListMembersGetter, accessListMemberExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessListMembers
		case types.KindAccessListReview:
			if c.AccessLists == nil {
				return nil, trace.BadParameter("missing parameter AccessLists")
			}
			collections.accessListReviews = &genericCollection[*accesslist.Review, accessListReviewsGetter, accessListReviewExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessListReviews
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
		case types.KindNotification:
			if c.Notifications == nil {
				return nil, trace.BadParameter("missing parameter Notifications")
			}
			collections.userNotifications = &genericCollection[*notificationsv1.Notification, notificationGetter, userNotificationExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.userNotifications
		case types.KindGlobalNotification:
			if c.Notifications == nil {
				return nil, trace.BadParameter("missing parameter Notifications")
			}
			collections.globalNotifications = &genericCollection[*notificationsv1.GlobalNotification, notificationGetter, globalNotificationExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.globalNotifications
		case types.KindAccessMonitoringRule:
			if c.AccessMonitoringRules == nil {
				return nil, trace.BadParameter("missing parameter AccessMonitoringRule")
			}
			collections.accessMonitoringRules = &genericCollection[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleGetter, accessMonitoringRulesExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.accessMonitoringRules
		case types.KindAccessGraphSettings:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections.accessGraphSettings = &genericCollection[*clusterconfigpb.AccessGraphSettings, accessGraphSettingsGetter, accessGraphSettingsExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.accessGraphSettings
		case types.KindSPIFFEFederation:
			if c.Config.SPIFFEFederations == nil {
				return nil, trace.BadParameter("missing parameter SPIFFEFederations")
			}
			collections.spiffeFederations = &genericCollection[*machineidv1.SPIFFEFederation, SPIFFEFederationReader, spiffeFederationExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.spiffeFederations
		case types.KindWorkloadIdentity:
			if c.Config.WorkloadIdentity == nil {
				return nil, trace.BadParameter("missing parameter WorkloadIdentity")
			}
			collections.workloadIdentity = &genericCollection[*workloadidentityv1pb.WorkloadIdentity, WorkloadIdentityReader, workloadIdentityExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.workloadIdentity
		case types.KindAutoUpdateConfig:
			if c.AutoUpdateService == nil {
				return nil, trace.BadParameter("missing parameter AutoUpdateService")
			}
			collections.autoUpdateConfigs = &genericCollection[*autoupdate.AutoUpdateConfig, autoUpdateConfigGetter, autoUpdateConfigExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.autoUpdateConfigs
		case types.KindAutoUpdateVersion:
			if c.AutoUpdateService == nil {
				return nil, trace.BadParameter("missing parameter AutoUpdateService")
			}
			collections.autoUpdateVersions = &genericCollection[*autoupdate.AutoUpdateVersion, autoUpdateVersionGetter, autoUpdateVersionExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.autoUpdateVersions
		case types.KindAutoUpdateAgentRollout:
			if c.AutoUpdateService == nil {
				return nil, trace.BadParameter("missing parameter AutoUpdateService")
			}
			collections.autoUpdateAgentRollouts = &genericCollection[*autoupdate.AutoUpdateAgentRollout, autoUpdateAgentRolloutGetter, autoUpdateAgentRolloutExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.autoUpdateAgentRollouts

		case types.KindProvisioningPrincipalState:
			if c.ProvisioningStates == nil {
				return nil, trace.BadParameter("missing parameter KindProvisioningState")
			}
			collections.provisioningStates = &genericCollection[*provisioningv1.PrincipalState, provisioningStateGetter, provisioningStateExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.provisioningStates

		case types.KindIdentityCenterAccount:
			if c.IdentityCenter == nil {
				return nil, trace.BadParameter("missing upstream IdentityCenter collection")
			}
			collections.identityCenterAccounts = &genericCollection[
				services.IdentityCenterAccount,
				identityCenterAccountGetter,
				identityCenterAccountExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.identityCenterAccounts

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

		case types.KindIdentityCenterAccountAssignment:
			if c.IdentityCenter == nil {
				return nil, trace.BadParameter("missing parameter IdentityCenter")
			}
			collections.identityCenterAccountAssignments = &genericCollection[
				services.IdentityCenterAccountAssignment,
				identityCenterAccountAssignmentGetter,
				identityCenterAccountAssignmentExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.identityCenterAccountAssignments

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
		if err != nil {
			if !(types.IsUnsupportedAuthorityErr(err) && caType.NewlyAdded()) {
				return nil, trace.Wrap(err)
			}
			continue
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

type autoUpdateConfigExecutor struct{}

func (autoUpdateConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*autoupdate.AutoUpdateConfig, error) {
	config, err := cache.AutoUpdateService.GetAutoUpdateConfig(ctx)
	return []*autoupdate.AutoUpdateConfig{config}, trace.Wrap(err)
}

func (autoUpdateConfigExecutor) upsert(ctx context.Context, cache *Cache, resource *autoupdate.AutoUpdateConfig) error {
	_, err := cache.autoUpdateCache.UpsertAutoUpdateConfig(ctx, resource)
	return trace.Wrap(err)
}

func (autoUpdateConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.autoUpdateCache.DeleteAutoUpdateConfig(ctx)
}

func (autoUpdateConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.autoUpdateCache.DeleteAutoUpdateConfig(ctx)
}

func (autoUpdateConfigExecutor) isSingleton() bool { return true }

func (autoUpdateConfigExecutor) getReader(cache *Cache, cacheOK bool) autoUpdateConfigGetter {
	if cacheOK {
		return cache.autoUpdateCache
	}
	return cache.Config.AutoUpdateService
}

type autoUpdateConfigGetter interface {
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)
}

var _ executor[*autoupdate.AutoUpdateConfig, autoUpdateConfigGetter] = autoUpdateConfigExecutor{}

type autoUpdateVersionExecutor struct{}

func (autoUpdateVersionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*autoupdate.AutoUpdateVersion, error) {
	version, err := cache.AutoUpdateService.GetAutoUpdateVersion(ctx)
	return []*autoupdate.AutoUpdateVersion{version}, trace.Wrap(err)
}

func (autoUpdateVersionExecutor) upsert(ctx context.Context, cache *Cache, resource *autoupdate.AutoUpdateVersion) error {
	_, err := cache.autoUpdateCache.UpsertAutoUpdateVersion(ctx, resource)
	return trace.Wrap(err)
}

func (autoUpdateVersionExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.autoUpdateCache.DeleteAutoUpdateVersion(ctx)
}

func (autoUpdateVersionExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.autoUpdateCache.DeleteAutoUpdateVersion(ctx)
}

func (autoUpdateVersionExecutor) isSingleton() bool { return true }

func (autoUpdateVersionExecutor) getReader(cache *Cache, cacheOK bool) autoUpdateVersionGetter {
	if cacheOK {
		return cache.autoUpdateCache
	}
	return cache.Config.AutoUpdateService
}

type autoUpdateVersionGetter interface {
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)
}

var _ executor[*autoupdate.AutoUpdateVersion, autoUpdateVersionGetter] = autoUpdateVersionExecutor{}

type autoUpdateAgentRolloutExecutor struct{}

func (autoUpdateAgentRolloutExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*autoupdate.AutoUpdateAgentRollout, error) {
	plan, err := cache.AutoUpdateService.GetAutoUpdateAgentRollout(ctx)
	return []*autoupdate.AutoUpdateAgentRollout{plan}, trace.Wrap(err)
}

func (autoUpdateAgentRolloutExecutor) upsert(ctx context.Context, cache *Cache, resource *autoupdate.AutoUpdateAgentRollout) error {
	_, err := cache.autoUpdateCache.UpsertAutoUpdateAgentRollout(ctx, resource)
	return trace.Wrap(err)
}

func (autoUpdateAgentRolloutExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.autoUpdateCache.DeleteAutoUpdateAgentRollout(ctx)
}

func (autoUpdateAgentRolloutExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.autoUpdateCache.DeleteAutoUpdateAgentRollout(ctx)
}

func (autoUpdateAgentRolloutExecutor) isSingleton() bool { return true }

func (autoUpdateAgentRolloutExecutor) getReader(cache *Cache, cacheOK bool) autoUpdateAgentRolloutGetter {
	if cacheOK {
		return cache.autoUpdateCache
	}
	return cache.Config.AutoUpdateService
}

type autoUpdateAgentRolloutGetter interface {
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)
}

var _ executor[*autoupdate.AutoUpdateAgentRollout, autoUpdateAgentRolloutGetter] = autoUpdateAgentRolloutExecutor{}

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

type roleExecutor struct{}

func (roleExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.Role, error) {
	return cache.Access.GetRoles(ctx)
}

func (roleExecutor) upsert(ctx context.Context, cache *Cache, resource types.Role) error {
	_, err := cache.accessCache.UpsertRole(ctx, resource)
	return err
}

func (roleExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessCache.DeleteAllRoles(ctx)
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
	ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
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

func (databaseServiceExecutor) getReader(_ *Cache, _ bool) noReader {
	return noReader{}
}

var _ executor[types.DatabaseService, noReader] = databaseServiceExecutor{}

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

		if !loadSecrets {
			for i := 0; i < len(webSessions); i++ {
				webSessions[i] = webSessions[i].WithoutSecrets()
			}
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
	ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error)
}

var _ executor[types.WebSession, appSessionGetter] = appSessionExecutor{}

type snowflakeSessionExecutor struct{}

func (snowflakeSessionExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.WebSession, error) {
	webSessions, err := cache.SnowflakeSession.GetSnowflakeSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !loadSecrets {
		for i := 0; i < len(webSessions); i++ {
			webSessions[i] = webSessions[i].WithoutSecrets()
		}
	}

	return webSessions, nil
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

		if !loadSecrets {
			for i := 0; i < len(webSessions); i++ {
				webSessions[i] = webSessions[i].WithoutSecrets()
			}
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
	webSessions, err := cache.WebSession.List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !loadSecrets {
		for i := 0; i < len(webSessions); i++ {
			webSessions[i] = webSessions[i].WithoutSecrets()
		}
	}

	return webSessions, nil
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
	_, err := cache.clusterConfigCache.UpsertAuthPreference(ctx, resource)
	return trace.Wrap(err)
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
	GetClusterAuditConfig(context.Context) (types.ClusterAuditConfig, error)
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
	_, err := cache.clusterConfigCache.UpsertClusterNetworkingConfig(ctx, resource)
	return trace.Wrap(err)
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
	GetClusterNetworkingConfig(context.Context) (types.ClusterNetworkingConfig, error)
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
	_, err := cache.clusterConfigCache.UpsertSessionRecordingConfig(ctx, resource)
	return trace.Wrap(err)
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
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
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
	case types.Resource153Unwrapper:
		switch wc := r.Unwrap().(type) {
		case *kubewaitingcontainerpb.KubernetesWaitingContainer:
			err := cache.kubeWaitingContsCache.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
				Username:      wc.Spec.Username,
				Cluster:       wc.Spec.Cluster,
				Namespace:     wc.Spec.Namespace,
				PodName:       wc.Spec.PodName,
				ContainerName: wc.Spec.ContainerName,
			})
			return trace.Wrap(err)
		}
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

type crownJewelsExecutor struct{}

func (crownJewelsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*crownjewelv1.CrownJewel, error) {
	var resources []*crownjewelv1.CrownJewel
	var nextToken string
	for {
		var page []*crownjewelv1.CrownJewel
		var err error
		page, nextToken, err = cache.CrownJewels.ListCrownJewels(ctx, 0 /* page size */, nextToken)
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

func (crownJewelsExecutor) upsert(ctx context.Context, cache *Cache, resource *crownjewelv1.CrownJewel) error {
	_, err := cache.crownJewelsCache.UpsertCrownJewel(ctx, resource)
	return trace.Wrap(err)
}

func (crownJewelsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.crownJewelsCache.DeleteAllCrownJewels(ctx)
}

func (crownJewelsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.crownJewelsCache.DeleteCrownJewel(ctx, resource.GetName())
}

func (crownJewelsExecutor) isSingleton() bool { return false }

func (crownJewelsExecutor) getReader(cache *Cache, cacheOK bool) crownjewelsGetter {
	if cacheOK {
		return cache.crownJewelsCache
	}
	return cache.Config.CrownJewels
}

var _ executor[*crownjewelv1.CrownJewel, crownjewelsGetter] = crownJewelsExecutor{}

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

type accessListExecutor struct{}

func (accessListExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*accesslist.AccessList, error) {
	var resources []*accesslist.AccessList
	var nextToken string
	for {
		var page []*accesslist.AccessList
		var err error
		page, nextToken, err = cache.AccessLists.ListAccessLists(ctx, 0 /* page size */, nextToken)
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

func (accessListExecutor) upsert(ctx context.Context, cache *Cache, resource *accesslist.AccessList) error {
	_, err := cache.accessListCache.UnconditionalUpsertAccessList(ctx, resource)
	return trace.Wrap(err)
}

func (accessListExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessListCache.DeleteAllAccessLists(ctx)
}

func (accessListExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessListCache.UnconditionalDeleteAccessList(ctx, resource.GetName())
}

func (accessListExecutor) isSingleton() bool { return false }

func (accessListExecutor) getReader(cache *Cache, cacheOK bool) accessListsGetter {
	if cacheOK {
		return cache.accessListCache
	}
	return cache.Config.AccessLists
}

type accessListsGetter interface {
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
}

type accessListMemberExecutor struct{}

func (accessListMemberExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*accesslist.AccessListMember, error) {
	var resources []*accesslist.AccessListMember
	var nextToken string
	for {
		var page []*accesslist.AccessListMember
		var err error
		page, nextToken, err = cache.AccessLists.ListAllAccessListMembers(ctx, 0 /* page size */, nextToken)
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

func (accessListMemberExecutor) upsert(ctx context.Context, cache *Cache, resource *accesslist.AccessListMember) error {
	_, err := cache.accessListCache.UnconditionalUpsertAccessListMember(ctx, resource)
	return trace.Wrap(err)
}

func (accessListMemberExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessListCache.DeleteAllAccessListMembers(ctx)
}

func (accessListMemberExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessListCache.UnconditionalDeleteAccessListMember(ctx,
		resource.GetMetadata().Description, // Cache passes access  ID via description field.
		resource.GetName())
}

func (accessListMemberExecutor) isSingleton() bool { return false }

func (accessListMemberExecutor) getReader(cache *Cache, cacheOK bool) accessListMembersGetter {
	if cacheOK {
		return cache.accessListCache
	}
	return cache.Config.AccessLists
}

type accessListMembersGetter interface {
	CountAccessListMembers(ctx context.Context, accessListName string) (uint32, uint32, error)
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error)
}

type accessListReviewExecutor struct{}

func (accessListReviewExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*accesslist.Review, error) {
	var resources []*accesslist.Review
	var nextToken string
	for {
		var page []*accesslist.Review
		var err error
		page, nextToken, err = cache.AccessLists.ListAllAccessListReviews(ctx, 0 /* page size */, nextToken)
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

func (accessListReviewExecutor) upsert(ctx context.Context, cache *Cache, resource *accesslist.Review) error {
	if _, _, err := cache.accessListCache.CreateAccessListReview(ctx, resource); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		if err := cache.accessListCache.DeleteAccessListReview(ctx, resource.Spec.AccessList, resource.GetName()); err != nil {
			return trace.Wrap(err)
		}

		if _, _, err := cache.accessListCache.CreateAccessListReview(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (accessListReviewExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.accessListCache.DeleteAllAccessListReviews(ctx)
}

func (accessListReviewExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.accessListCache.DeleteAccessListReview(ctx,
		resource.GetMetadata().Description, // Cache passes access  ID via description field.
		resource.GetName())
}

func (accessListReviewExecutor) isSingleton() bool { return false }

func (accessListReviewExecutor) getReader(cache *Cache, cacheOK bool) accessListReviewsGetter {
	if cacheOK {
		return cache.accessListCache
	}
	return cache.Config.AccessLists
}

type accessListReviewsGetter interface {
	ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error)
}

type notificationGetter interface {
	ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error)
	ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error)
}

type userNotificationExecutor struct{}

func (userNotificationExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*notificationsv1.Notification, error) {
	var notifications []*notificationsv1.Notification
	var startKey string
	for {
		notifs, nextKey, err := cache.notificationsCache.ListUserNotifications(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		notifications = append(notifications, notifs...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	return notifications, nil
}

func (userNotificationExecutor) upsert(ctx context.Context, cache *Cache, notification *notificationsv1.Notification) error {
	_, err := cache.notificationsCache.UpsertUserNotification(ctx, notification)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (userNotificationExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.notificationsCache.DeleteAllUserNotifications(ctx)
}

func (userNotificationExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	r, ok := resource.(types.Resource153Unwrapper)
	if !ok {
		return trace.BadParameter("unknown resource type, expected types.Resource153Unwrapper, got %T", resource)
	}

	notification, ok := r.Unwrap().(*notificationsv1.Notification)
	if !ok {
		return trace.BadParameter("unknown Notification type, expected *notificationsv1.Notification, got %T", resource)
	}

	username := notification.GetSpec().GetUsername()
	notificationId := notification.GetMetadata().GetName()

	err := cache.notificationsCache.DeleteUserNotification(ctx, username, notificationId)
	return trace.Wrap(err)
}

func (userNotificationExecutor) isSingleton() bool { return false }

func (userNotificationExecutor) getReader(cache *Cache, cacheOK bool) notificationGetter {
	if cacheOK {
		return cache.notificationsCache
	}
	return cache.Config.Notifications
}

var _ executor[*notificationsv1.Notification, notificationGetter] = userNotificationExecutor{}

type globalNotificationExecutor struct{}

func (globalNotificationExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*notificationsv1.GlobalNotification, error) {
	var notifications []*notificationsv1.GlobalNotification
	var startKey string
	for {
		notifs, nextKey, err := cache.notificationsCache.ListGlobalNotifications(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		notifications = append(notifications, notifs...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	return notifications, nil
}

func (globalNotificationExecutor) upsert(ctx context.Context, cache *Cache, notification *notificationsv1.GlobalNotification) error {
	if _, err := cache.notificationsCache.UpsertGlobalNotification(ctx, notification); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (globalNotificationExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.notificationsCache.DeleteAllGlobalNotifications(ctx)
}

func (globalNotificationExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {

	r, ok := resource.(types.Resource153Unwrapper)
	if !ok {
		return trace.BadParameter("unknown resource type, expected types.Resource153Unwrapper, got %T", resource)
	}

	globalNotification, ok := r.Unwrap().(*notificationsv1.GlobalNotification)
	if !ok {
		return trace.BadParameter("unknown Notification type, expected *notificationsv1.GlobalNotification, got %T", resource)
	}

	notificationId := globalNotification.GetMetadata().GetName()

	err := cache.notificationsCache.DeleteGlobalNotification(ctx, notificationId)
	return trace.Wrap(err)
}

func (globalNotificationExecutor) isSingleton() bool { return false }

func (globalNotificationExecutor) getReader(cache *Cache, cacheOK bool) notificationGetter {
	if cacheOK {
		return cache.notificationsCache
	}
	return cache.Config.Notifications
}

var _ executor[*notificationsv1.GlobalNotification, notificationGetter] = globalNotificationExecutor{}

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
	ListAccessMonitoringRulesWithFilter(ctx context.Context, pageSize int, nextToken string, subjects []string, notificationName string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}

type accessGraphSettingsExecutor struct{}

func (accessGraphSettingsExecutor) getAll(ctx context.Context, cache *Cache, _ bool) ([]*clusterconfigpb.AccessGraphSettings, error) {
	set, err := cache.ClusterConfig.GetAccessGraphSettings(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []*clusterconfigpb.AccessGraphSettings{set}, nil
}

func (accessGraphSettingsExecutor) upsert(ctx context.Context, cache *Cache, resource *clusterconfigpb.AccessGraphSettings) error {
	_, err := cache.clusterConfigCache.UpsertAccessGraphSettings(ctx, resource)
	return trace.Wrap(err)
}

func (accessGraphSettingsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.clusterConfigCache.DeleteAccessGraphSettings(ctx))
}

func (accessGraphSettingsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.clusterConfigCache.DeleteAccessGraphSettings(ctx))
}

func (accessGraphSettingsExecutor) isSingleton() bool { return false }

func (accessGraphSettingsExecutor) getReader(cache *Cache, cacheOK bool) accessGraphSettingsGetter {
	if cacheOK {
		return cache.clusterConfigCache
	}
	return cache.Config.ClusterConfig
}

type accessGraphSettingsGetter interface {
	GetAccessGraphSettings(context.Context) (*clusterconfigpb.AccessGraphSettings, error)
}

var _ executor[*clusterconfigpb.AccessGraphSettings, accessGraphSettingsGetter] = accessGraphSettingsExecutor{}
