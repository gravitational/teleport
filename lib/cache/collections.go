// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	kubewaitingcontainerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/types/userloginstate"
	scopedrole "github.com/gravitational/teleport/lib/scopes/roles"
)

// collectionHandler is used by the [Cache] to seed the initial
// data and process events for a particular resource.
type collectionHandler interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// onDelete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to clear.
	onDelete(t types.Resource) error
	// onPut will update a single target resource from the cache
	onPut(t types.Resource) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// collections is the group of resource [collection]s
// that the [Cache] supports.
type collections struct {
	byKind map[resourceKind]collectionHandler

	provisionTokens                    *collection[types.ProvisionToken, provisionTokenIndex]
	staticTokens                       *collection[types.StaticTokens, staticTokensIndex]
	certAuthorities                    *collection[types.CertAuthority, certAuthorityIndex]
	users                              *collection[types.User, userIndex]
	roles                              *collection[types.Role, roleIndex]
	authServers                        *collection[types.Server, authServerIndex]
	proxyServers                       *collection[types.Server, proxyServerIndex]
	nodes                              *collection[types.Server, nodeIndex]
	apps                               *collection[types.Application, appIndex]
	appServers                         *collection[types.AppServer, appServerIndex]
	dbs                                *collection[types.Database, databaseIndex]
	dbServers                          *collection[types.DatabaseServer, databaseServerIndex]
	dbServices                         *collection[types.DatabaseService, databaseServiceIndex]
	kubeServers                        *collection[types.KubeServer, kubeServerIndex]
	kubeClusters                       *collection[types.KubeCluster, kubeClusterIndex]
	kubeWaitingContainers              *collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]
	windowsDesktops                    *collection[types.WindowsDesktop, windowsDesktopIndex]
	windowsDesktopServices             *collection[types.WindowsDesktopService, windowsDesktopServiceIndex]
	dynamicWindowsDesktops             *collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]
	userGroups                         *collection[types.UserGroup, userGroupIndex]
	identityCenterAccounts             *collection[*identitycenterv1.Account, identityCenterAccountIndex]
	identityCenterAccountAssignments   *collection[*identitycenterv1.AccountAssignment, identityCenterAccountAssignmentIndex]
	healthCheckConfig                  *collection[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]
	reverseTunnels                     *collection[types.ReverseTunnel, reverseTunnelIndex]
	spiffeFederations                  *collection[*machineidv1.SPIFFEFederation, spiffeFederationIndex]
	workloadIdentity                   *collection[*workloadidentityv1.WorkloadIdentity, workloadIdentityIndex]
	userNotifications                  *collection[*notificationsv1.Notification, userNotificationIndex]
	globalNotifications                *collection[*notificationsv1.GlobalNotification, globalNotificationIndex]
	clusterName                        *collection[types.ClusterName, clusterNameIndex]
	auditConfig                        *collection[types.ClusterAuditConfig, clusterAuditConfigIndex]
	networkingConfig                   *collection[types.ClusterNetworkingConfig, clusterNetworkingConfigIndex]
	authPreference                     *collection[types.AuthPreference, authPreferenceIndex]
	sessionRecordingConfig             *collection[types.SessionRecordingConfig, sessionRecordingConfigIndex]
	autoUpdateConfig                   *collection[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex]
	autoUpdateVerion                   *collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]
	autoUpdateRollout                  *collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]
	autoUpdateReports                  *collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]
	oktaImportRules                    *collection[types.OktaImportRule, oktaImportRuleIndex]
	oktaAssignments                    *collection[types.OktaAssignment, oktaAssignmentIndex]
	samlIdPServiceProviders            *collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]
	webSessions                        *collection[types.WebSession, webSessionIndex]
	appSessions                        *collection[types.WebSession, appSessionIndex]
	snowflakeSessions                  *collection[types.WebSession, snowflakeSessionIndex]
	accessLists                        *collection[*accesslist.AccessList, accessListIndex]
	accessListMembers                  *collection[*accesslist.AccessListMember, accessListMemberIndex]
	accessListReviews                  *collection[*accesslist.Review, accessListReviewIndex]
	crownJewels                        *collection[*crownjewelv1.CrownJewel, crownJewelIndex]
	accessGraphSettings                *collection[*clusterconfigv1.AccessGraphSettings, accessGraphSettingsIndex]
	integrations                       *collection[types.Integration, integrationIndex]
	pluginStaticCredentials            *collection[types.PluginStaticCredentials, pluginStaticCredentialsIndex]
	accessMonitoringRules              *collection[*accessmonitoringrulesv1.AccessMonitoringRule, accessMonitoringRuleIndex]
	webTokens                          *collection[types.WebToken, webTokenIndex]
	uiConfigs                          *collection[types.UIConfig, webUIConfigIndex]
	installers                         *collection[types.Installer, installerIndex]
	locks                              *collection[types.Lock, lockIndex]
	tunnelConnections                  *collection[types.TunnelConnection, tunnelConnectionIndex]
	remoteClusters                     *collection[types.RemoteCluster, remoteClusterIndex]
	userTasks                          *collection[*usertasksv1.UserTask, userTaskIndex]
	userLoginStates                    *collection[*userloginstate.UserLoginState, userLoginStateIndex]
	gitServers                         *collection[types.Server, gitServerIndex]
	databaseObjects                    *collection[*dbobjectv1.DatabaseObject, databaseObjectIndex]
	staticHostUsers                    *collection[*userprovisioningv2.StaticHostUser, staticHostUserIndex]
	networkRestrictions                *collection[types.NetworkRestrictions, networkingRestrictionIndex]
	discoveryConfigs                   *collection[*discoveryconfig.DiscoveryConfig, discoveryConfigIndex]
	provisioningStates                 *collection[*provisioningv1.PrincipalState, principalStateIndex]
	identityCenterPrincipalAssignments *collection[*identitycenterv1.PrincipalAssignment, identityCenterPrincipalAssignmentIndex]
	auditQueries                       *collection[*secreports.AuditQuery, auditQueryIndex]
	secReports                         *collection[*secreports.Report, securityReportIndex]
	secReportsStates                   *collection[*secreports.ReportState, securityReportStateIndex]
	relayServers                       *collection[*presencev1.RelayServer, relayServerIndex]
}

// isKnownUncollectedKind is true if a resource kind is not stored in
// the cache itself but it's only configured in the cache so that the
// resources events can be processed by downstream watchers.
func isKnownUncollectedKind(kind string) bool {
	switch kind {
	case types.KindAccessRequest, types.KindHeadlessAuthentication, scopedrole.KindScopedRole, scopedrole.KindScopedRoleAssignment:
		return true
	default:
		return false
	}
}

// setupCollections ensures that the appropriate [collection] is
// initialized for all provided [types.WatcKind]s. An error is
// returned if a [types.WatchKind] has no associated [collection].
func setupCollections(c Config) (*collections, error) {
	out := &collections{
		byKind: make(map[resourceKind]collectionHandler, 1),
	}

	for _, watch := range c.Watches {
		if isKnownUncollectedKind(watch.Kind) {
			continue
		}

		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindToken:
			collect, err := newProvisionTokensCollection(c.Provisioner, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.provisionTokens = collect
			out.byKind[resourceKind] = out.provisionTokens
		case types.KindStaticTokens:
			collect, err := newStaticTokensCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.staticTokens = collect
			out.byKind[resourceKind] = out.staticTokens
		case types.KindCertAuthority:
			collect, err := newCertAuthorityCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.certAuthorities = collect
			out.byKind[resourceKind] = out.certAuthorities
		case types.KindUser:
			collect, err := newUserCollection(c.Users, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.users = collect
			out.byKind[resourceKind] = out.users
		case types.KindRole:
			collect, err := newRoleCollection(c.Access, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.roles = collect
			out.byKind[resourceKind] = out.roles
		case types.KindAuthServer:
			collect, err := newAuthServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.authServers = collect
			out.byKind[resourceKind] = out.authServers
		case types.KindProxy:
			collect, err := newProxyServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.proxyServers = collect
			out.byKind[resourceKind] = out.proxyServers
		case types.KindNode:
			collect, err := newNodeCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.nodes = collect
			out.byKind[resourceKind] = out.nodes
		case types.KindApp:
			collect, err := newAppCollection(c.Apps, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.apps = collect
			out.byKind[resourceKind] = out.apps
		case types.KindAppServer:
			collect, err := newAppServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.appServers = collect
			out.byKind[resourceKind] = out.appServers
		case types.KindDatabase:
			collect, err := newDatabaseCollection(c.Databases, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.dbs = collect
			out.byKind[resourceKind] = out.dbs
		case types.KindDatabaseServer:
			collect, err := newDatabaseServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.dbServers = collect
			out.byKind[resourceKind] = out.dbServers
		case types.KindDatabaseService:
			collect, err := newDatabaseServiceCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.dbServices = collect
			out.byKind[resourceKind] = out.dbServices
		case types.KindDatabaseObject:
			collect, err := newDatabaseObjectCollection(c.DatabaseObjects, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.databaseObjects = collect
			out.byKind[resourceKind] = out.databaseObjects
		case types.KindKubeServer:
			collect, err := newKubernetesServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.kubeServers = collect
			out.byKind[resourceKind] = out.kubeServers
		case types.KindKubernetesCluster:
			collect, err := newKubernetesClusterCollection(c.Kubernetes, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.kubeClusters = collect
			out.byKind[resourceKind] = out.kubeClusters
		case types.KindKubeWaitingContainer:
			collect, err := newKubernetesWaitingContainerCollection(c.KubeWaitingContainers, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.kubeWaitingContainers = collect
			out.byKind[resourceKind] = out.kubeWaitingContainers
		case types.KindWindowsDesktop:
			collect, err := newWindowsDesktopCollection(c.WindowsDesktops, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.windowsDesktops = collect
			out.byKind[resourceKind] = out.windowsDesktops
		case types.KindWindowsDesktopService:
			collect, err := newWindowsDesktopServiceCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.windowsDesktopServices = collect
			out.byKind[resourceKind] = out.windowsDesktopServices
		case types.KindDynamicWindowsDesktop:
			collect, err := newDynamicWindowsDesktopCollection(c.DynamicWindowsDesktops, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.dynamicWindowsDesktops = collect
			out.byKind[resourceKind] = out.dynamicWindowsDesktops
		case types.KindUserGroup:
			collect, err := newUserGroupCollection(c.UserGroups, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.userGroups = collect
			out.byKind[resourceKind] = out.userGroups
		case types.KindIdentityCenterAccount:
			collect, err := newIdentityCenterAccountCollection(c.IdentityCenter, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.identityCenterAccounts = collect
			out.byKind[resourceKind] = out.identityCenterAccounts
		case types.KindIdentityCenterAccountAssignment:
			collect, err := newIdentityCenterAccountAssignmentCollection(c.IdentityCenter, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.identityCenterAccountAssignments = collect
			out.byKind[resourceKind] = out.identityCenterAccountAssignments
		case types.KindHealthCheckConfig:
			collect, err := newHealthCheckConfigCollection(c.HealthCheckConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.healthCheckConfig = collect
			out.byKind[resourceKind] = out.healthCheckConfig
		case types.KindReverseTunnel:
			collect, err := newReverseTunnelCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.reverseTunnels = collect
			out.byKind[resourceKind] = out.reverseTunnels
		case types.KindSPIFFEFederation:
			collect, err := newSPIFFEFederationCollection(c.SPIFFEFederations, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.spiffeFederations = collect
			out.byKind[resourceKind] = out.spiffeFederations
		case types.KindWorkloadIdentity:
			collect, err := newWorkloadIdentityCollection(c.WorkloadIdentity, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.workloadIdentity = collect
			out.byKind[resourceKind] = out.workloadIdentity
		case types.KindNotification:
			collect, err := newUserNotificationCollection(c.Notifications, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.userNotifications = collect
			out.byKind[resourceKind] = out.userNotifications
		case types.KindGlobalNotification:
			collect, err := newGlobalNotificationCollection(c.Notifications, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.globalNotifications = collect
			out.byKind[resourceKind] = out.globalNotifications
		case types.KindClusterName:
			collect, err := newClusterNameCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.clusterName = collect
			out.byKind[resourceKind] = out.clusterName
		case types.KindClusterAuditConfig:
			collect, err := newClusterAuditConfigCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.auditConfig = collect
			out.byKind[resourceKind] = out.auditConfig
		case types.KindClusterNetworkingConfig:
			collect, err := newClusterNetworkingConfigCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.networkingConfig = collect
			out.byKind[resourceKind] = out.networkingConfig
		case types.KindClusterAuthPreference:
			collect, err := newAuthPreferenceCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.authPreference = collect
			out.byKind[resourceKind] = out.authPreference
		case types.KindSessionRecordingConfig:
			collect, err := newSessionRecordingConfigCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.sessionRecordingConfig = collect
			out.byKind[resourceKind] = out.sessionRecordingConfig
		case types.KindAutoUpdateConfig:
			collect, err := newAutoUpdateConfigCollection(c.AutoUpdateService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.autoUpdateConfig = collect
			out.byKind[resourceKind] = out.autoUpdateConfig
		case types.KindAutoUpdateVersion:
			collect, err := newAutoUpdateVersionCollection(c.AutoUpdateService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.autoUpdateVerion = collect
			out.byKind[resourceKind] = out.autoUpdateVerion
		case types.KindAutoUpdateAgentRollout:
			collect, err := newAutoUpdateRolloutCollection(c.AutoUpdateService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.autoUpdateRollout = collect
			out.byKind[resourceKind] = out.autoUpdateRollout
		case types.KindAutoUpdateAgentReport:
			collect, err := newAutoUpdateAgentReportCollection(c.AutoUpdateService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.autoUpdateReports = collect
			out.byKind[resourceKind] = out.autoUpdateReports
		case types.KindOktaImportRule:
			collect, err := newOktaImportRuleCollection(c.Okta, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.oktaImportRules = collect
			out.byKind[resourceKind] = out.oktaImportRules
		case types.KindOktaAssignment:
			collect, err := newOktaImportAssignmentCollection(c.Okta, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.oktaAssignments = collect
			out.byKind[resourceKind] = out.oktaAssignments
		case types.KindSAMLIdPServiceProvider:
			collect, err := newSAMLIdPServiceProviderCollection(c.SAMLIdPServiceProviders, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.samlIdPServiceProviders = collect
			out.byKind[resourceKind] = out.samlIdPServiceProviders
		case types.KindWebSession:
			switch watch.SubKind {
			case types.KindAppSession:
				collect, err := newAppSessionCollection(c.AppSession, watch)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				out.appSessions = collect
				out.byKind[resourceKind] = out.appSessions
			case types.KindSnowflakeSession:
				collect, err := newSnowflakeSessionCollection(c.SnowflakeSession, watch)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				out.snowflakeSessions = collect
				out.byKind[resourceKind] = out.snowflakeSessions
			case types.KindWebSession:
				collect, err := newWebSessionCollection(c.WebSession, watch)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				out.webSessions = collect
				out.byKind[resourceKind] = out.webSessions
			}
		case types.KindAccessList:
			collect, err := newAccessListCollection(c.AccessLists, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.accessLists = collect
			out.byKind[resourceKind] = out.accessLists
		case types.KindAccessListMember:
			collect, err := newAccessListMemberCollection(c.AccessLists, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.accessListMembers = collect
			out.byKind[resourceKind] = out.accessListMembers
		case types.KindAccessListReview:
			collect, err := newAccessListReviewCollection(c.AccessLists, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.accessListReviews = collect
			out.byKind[resourceKind] = out.accessListReviews
		case types.KindCrownJewel:
			collect, err := newCrownJewelCollection(c.CrownJewels, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.crownJewels = collect
			out.byKind[resourceKind] = out.crownJewels
		case types.KindAccessGraphSettings:
			collect, err := newAccessGraphSettingsCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.accessGraphSettings = collect
			out.byKind[resourceKind] = out.accessGraphSettings
		case types.KindIntegration:
			collect, err := newIntegrationCollection(c.Integrations, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.integrations = collect
			out.byKind[resourceKind] = out.integrations
		case types.KindPluginStaticCredentials:
			collect, err := newPluginStaticCredentialsCollection(c.PluginStaticCredentials, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.pluginStaticCredentials = collect
			out.byKind[resourceKind] = out.pluginStaticCredentials
		case types.KindAccessMonitoringRule:
			collect, err := newAccessMonitoringRuleCollection(c.AccessMonitoringRules, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.accessMonitoringRules = collect
			out.byKind[resourceKind] = out.accessMonitoringRules
		case types.KindUIConfig:
			collect, err := newWebUIConfigCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.uiConfigs = collect
			out.byKind[resourceKind] = out.uiConfigs
		case types.KindWebToken:
			collect, err := newWebTokenCollection(c.WebToken, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.webTokens = collect
			out.byKind[resourceKind] = out.webTokens
		case types.KindInstaller:
			collect, err := newInstallerCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.installers = collect
			out.byKind[resourceKind] = out.installers
		case types.KindLock:
			collect, err := newLockCollection(c.Access, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.locks = collect
			out.byKind[resourceKind] = out.locks
		case types.KindTunnelConnection:
			collect, err := newTunnelConnectionCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.tunnelConnections = collect
			out.byKind[resourceKind] = out.tunnelConnections
		case types.KindRemoteCluster:
			collect, err := newRemoteClusterCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.remoteClusters = collect
			out.byKind[resourceKind] = out.remoteClusters
		case types.KindUserTask:
			collect, err := newUserTaskCollection(c.UserTasks, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.userTasks = collect
			out.byKind[resourceKind] = out.userTasks
		case types.KindUserLoginState:
			collect, err := newUserLoginStateCollection(c.UserLoginStates, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.userLoginStates = collect
			out.byKind[resourceKind] = out.userLoginStates
		case types.KindGitServer:
			collect, err := newGitServerCollection(c.GitServers, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.gitServers = collect
			out.byKind[resourceKind] = out.gitServers
		case types.KindStaticHostUser:
			collect, err := newStaticHostUserCollection(c.StaticHostUsers, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.staticHostUsers = collect
			out.byKind[resourceKind] = out.staticHostUsers
		case types.KindNetworkRestrictions:
			collect, err := newNetworkingRestrictionCollection(c.Restrictions, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.networkRestrictions = collect
			out.byKind[resourceKind] = out.networkRestrictions
		case types.KindDiscoveryConfig:
			collect, err := newDiscoveryConfigCollection(c.DiscoveryConfigs, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.discoveryConfigs = collect
			out.byKind[resourceKind] = out.discoveryConfigs
		case types.KindProvisioningPrincipalState:

			collect, err := newPrincipalStateCollection(c.ProvisioningStates, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.provisioningStates = collect
			out.byKind[resourceKind] = out.provisioningStates
		case types.KindIdentityCenterPrincipalAssignment:
			collect, err := newIdentityCenterPrincipalAssignmentCollection(c.IdentityCenter, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.identityCenterPrincipalAssignments = collect
			out.byKind[resourceKind] = out.identityCenterPrincipalAssignments
		case types.KindAuditQuery:
			collect, err := newAuditQueryCollection(c.SecReports, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.auditQueries = collect
			out.byKind[resourceKind] = out.auditQueries
		case types.KindSecurityReport:
			collect, err := newSecurityReportCollection(c.SecReports, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.secReports = collect
			out.byKind[resourceKind] = out.secReports
		case types.KindSecurityReportState:
			collect, err := newSecurityReportStateCollection(c.SecReports, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.secReportsStates = collect
			out.byKind[resourceKind] = out.secReportsStates
		case types.KindRelayServer:
			collect, err := newRelayServerCollection(c.Presence, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out.relayServers = collect
			out.byKind[resourceKind] = out.relayServers
		default:
			if _, ok := out.byKind[resourceKind]; !ok {
				return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
			}
		}
	}

	return out, nil
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
