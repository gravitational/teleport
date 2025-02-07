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
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

type ResourceAccess struct {
	List   bool `json:"list"`
	Read   bool `json:"read"`
	Edit   bool `json:"edit"`
	Create bool `json:"create"`
	Delete bool `json:"remove"`
	Use    bool `json:"use"`
}

// UserACL is derived from a user's role set and includes
// information as to what features the user is allowed to use.
type UserACL struct {
	// RecordedSessions defines access to recorded sessions.
	RecordedSessions ResourceAccess `json:"recordedSessions"`
	// ActiveSessions defines access to active sessions.
	ActiveSessions ResourceAccess `json:"activeSessions"`
	// AuthConnectors defines access to auth.connectors.
	AuthConnectors ResourceAccess `json:"authConnectors"`
	// Roles defines access to roles.
	Roles ResourceAccess `json:"roles"`
	// Users defines access to users.
	Users ResourceAccess `json:"users"`
	// TrustedClusters defines access to trusted clusters.
	TrustedClusters ResourceAccess `json:"trustedClusters"`
	// ClusterMaintenanceConfig defines access to update cluster maintenance config.
	ClusterMaintenanceConfig ResourceAccess `json:"clusterMaintenanceConfig"`
	// Events defines access to audit logs.
	Events ResourceAccess `json:"events"`
	// Tokens defines access to tokens.
	Tokens ResourceAccess `json:"tokens"`
	// Nodes defines access to nodes.
	Nodes ResourceAccess `json:"nodes"`
	// AppServers defines access to application servers
	AppServers ResourceAccess `json:"appServers"`
	// DBServers defines access to database servers.
	DBServers ResourceAccess `json:"dbServers"`
	// DB defines access to database resource.
	DB ResourceAccess `json:"db"`
	// KubeServers defines access to kubernetes servers.
	KubeServers ResourceAccess `json:"kubeServers"`
	// Desktops defines access to desktops.
	Desktops ResourceAccess `json:"desktops"`
	// AccessRequests defines access to access requests.
	AccessRequests ResourceAccess `json:"accessRequests"`
	// Billing defines access to billing information.
	Billing ResourceAccess `json:"billing"`
	// ConnectionDiagnostic defines access to connection diagnostics.
	ConnectionDiagnostic ResourceAccess `json:"connectionDiagnostic"`
	// Clipboard defines whether the user can use a shared clipboard during windows desktop sessions.
	Clipboard bool `json:"clipboard"`
	// DesktopSessionRecording defines whether the user's desktop sessions are being recorded.
	DesktopSessionRecording bool `json:"desktopSessionRecording"`
	// DirectorySharing defines whether a user is permitted to share a directory during windows desktop sessions.
	DirectorySharing bool `json:"directorySharing"`
	// Download defines whether the user has access to download Teleport Enterprise Binaries
	Download ResourceAccess `json:"download"`
	// Download defines whether the user has access to download the license
	License ResourceAccess `json:"license"`
	// Plugins defines whether the user has access to manage hosted plugin instances
	Plugins ResourceAccess `json:"plugins"`
	// Integrations defines whether the user has access to manage integrations.
	Integrations ResourceAccess `json:"integrations"`
	// UserTasks defines whether the user has access to manage UserTasks.
	UserTasks ResourceAccess `json:"userTasks"`
	// DeviceTrust defines access to device trust.
	DeviceTrust ResourceAccess `json:"deviceTrust"`
	// Locks defines access to locking resources.
	Locks ResourceAccess `json:"lock"`
	// SAMLIdpServiceProvider defines access to `saml_idp_service_provider` objects.
	SAMLIdpServiceProvider ResourceAccess `json:"samlIdpServiceProvider"`
	// AccessList defines access to access list management.
	AccessList ResourceAccess `json:"accessList"`
	// DiscoveryConfig defines whether the user has access to manage DiscoveryConfigs.
	DiscoveryConfig ResourceAccess `json:"discoverConfigs"`
	// AuditQuery defines access to audit query management.
	AuditQuery ResourceAccess `json:"auditQuery"`
	// SecurityReport defines access to security reports.
	SecurityReport ResourceAccess `json:"securityReport"`
	// ExternalAuditStorage defines access to manage ExternalAuditStorage
	ExternalAuditStorage ResourceAccess `json:"externalAuditStorage"`
	// AccessGraph defines access to access graph.
	AccessGraph ResourceAccess `json:"accessGraph"`
	// Bots defines access to manage Bots.
	Bots ResourceAccess `json:"bots"`
	// BotInstances defines access to manage bot instances
	BotInstances ResourceAccess `json:"botInstances"`
	// AccessMonitoringRule defines access to manage access monitoring rule resources.
	AccessMonitoringRule ResourceAccess `json:"accessMonitoringRule"`
	// CrownJewel defines access to manage CrownJewel resources.
	CrownJewel ResourceAccess `json:"crownJewel"`
	// AccessGraphSettings defines access to manage access graph settings.
	AccessGraphSettings ResourceAccess `json:"accessGraphSettings"`
	// ReviewRequests defines the ability to review requests
	ReviewRequests bool `json:"reviewRequests"`
	// Contact defines the ability to manage contacts
	Contact ResourceAccess `json:"contact"`
	// FileTransferAccess defines the ability to perform remote file operations via SCP or SFTP
	FileTransferAccess bool `json:"fileTransferAccess"`
	// GitServers defines access to Git servers.
	GitServers ResourceAccess `json:"gitServers"`
}

func hasAccess(roleSet RoleSet, ctx *Context, kind string, verbs ...string) bool {
	for _, verb := range verbs {
		// Since this check occurs often and does not imply the caller is trying to
		// ResourceAccess any resource, silence any logging done on the proxy.
		if err := roleSet.GuessIfAccessIsPossible(ctx, apidefaults.Namespace, kind, verb); err != nil {
			return false
		}
	}
	return true
}

func newAccess(roleSet RoleSet, ctx *Context, kind string) ResourceAccess {
	return ResourceAccess{
		List:   hasAccess(roleSet, ctx, kind, types.VerbList),
		Read:   hasAccess(roleSet, ctx, kind, types.VerbRead),
		Edit:   hasAccess(roleSet, ctx, kind, types.VerbUpdate),
		Create: hasAccess(roleSet, ctx, kind, types.VerbCreate),
		Delete: hasAccess(roleSet, ctx, kind, types.VerbDelete),
		Use:    hasAccess(roleSet, ctx, kind, types.VerbUse),
	}
}

// NewUserACL builds an ACL for a user based on their roles.
func NewUserACL(user types.User, userRoles RoleSet, features proto.Features, desktopRecordingEnabled, accessMonitoringEnabled bool) UserACL {
	ctx := &Context{User: user}
	recordedSessionAccess := newAccess(userRoles, ctx, types.KindSession)
	roleAccess := newAccess(userRoles, ctx, types.KindRole)
	authConnectors := newAccess(userRoles, ctx, types.KindAuthConnector)
	trustedClusterAccess := newAccess(userRoles, ctx, types.KindTrustedCluster)
	clusterMaintenanceConfig := newAccess(userRoles, ctx, types.KindClusterMaintenanceConfig)
	eventAccess := newAccess(userRoles, ctx, types.KindEvent)
	userAccess := newAccess(userRoles, ctx, types.KindUser)
	tokenAccess := newAccess(userRoles, ctx, types.KindToken)
	nodeAccess := newAccess(userRoles, ctx, types.KindNode)
	appServerAccess := newAccess(userRoles, ctx, types.KindAppServer)
	dbServerAccess := newAccess(userRoles, ctx, types.KindDatabaseServer)
	dbAccess := newAccess(userRoles, ctx, types.KindDatabase)
	kubeServerAccess := newAccess(userRoles, ctx, types.KindKubeServer)
	requestAccess := newAccess(userRoles, ctx, types.KindAccessRequest)
	accessMonitoringRules := newAccess(userRoles, ctx, types.KindAccessMonitoringRule)
	desktopAccess := newAccess(userRoles, ctx, types.KindWindowsDesktop)
	cnDiagnosticAccess := newAccess(userRoles, ctx, types.KindConnectionDiagnostic)
	samlIdpServiceProviderAccess := newAccess(userRoles, ctx, types.KindSAMLIdPServiceProvider)
	gitServersAccess := newAccess(userRoles, ctx, types.KindGitServer)

	// active sessions are a special case - if a user's role set has any join_sessions
	// policies then the ACL must permit showing active sessions
	activeSessionAccess := newAccess(userRoles, ctx, types.KindSSHSession)
	if userRoles.CanJoinSessions() {
		activeSessionAccess.List = true
		activeSessionAccess.Read = true
	}

	// The billing dashboards are available in: cloud clusters &
	// usage-based self-hosted non-stripe dashboards.
	var billingAccess ResourceAccess
	isDashboard := IsDashboard(features)
	isUsageBased := features.IsUsageBased
	isStripeManaged := features.IsStripeManaged

	if features.Cloud || (isDashboard && isUsageBased && !isStripeManaged) {
		billingAccess = newAccess(userRoles, ctx, types.KindBilling)
	}

	var pluginsAccess ResourceAccess
	if features.Plugins {
		pluginsAccess = newAccess(userRoles, ctx, types.KindPlugin)
	}

	var accessGraphAccess ResourceAccess
	var accessGraphSettings ResourceAccess
	if features.AccessGraph {
		accessGraphAccess = newAccess(userRoles, ctx, types.KindAccessGraph)
		accessGraphSettings = newAccess(userRoles, ctx, types.KindAccessGraphSettings)
	}

	clipboard := userRoles.DesktopClipboard()
	desktopSessionRecording := desktopRecordingEnabled && userRoles.RecordDesktopSession()
	directorySharing := userRoles.DesktopDirectorySharing()
	download := newAccess(userRoles, ctx, types.KindDownload)
	license := newAccess(userRoles, ctx, types.KindLicense)
	deviceTrust := newAccess(userRoles, ctx, types.KindDevice)
	integrationsAccess := newAccess(userRoles, ctx, types.KindIntegration)
	discoveryConfigsAccess := newAccess(userRoles, ctx, types.KindDiscoveryConfig)
	lockAccess := newAccess(userRoles, ctx, types.KindLock)
	accessListAccess := newAccess(userRoles, ctx, types.KindAccessList)
	externalAuditStorage := newAccess(userRoles, ctx, types.KindExternalAuditStorage)
	bots := newAccess(userRoles, ctx, types.KindBot)
	botInstances := newAccess(userRoles, ctx, types.KindBotInstance)
	crownJewelAccess := newAccess(userRoles, ctx, types.KindCrownJewel)
	userTasksAccess := newAccess(userRoles, ctx, types.KindUserTask)
	reviewRequests := userRoles.MaybeCanReviewRequests()
	fileTransferAccess := userRoles.CanCopyFiles()

	var auditQuery ResourceAccess
	var securityReports ResourceAccess
	if accessMonitoringEnabled {
		auditQuery = newAccess(userRoles, ctx, types.KindAuditQuery)
		securityReports = newAccess(userRoles, ctx, types.KindSecurityReport)
	}

	contact := newAccess(userRoles, ctx, types.KindContact)

	return UserACL{
		AccessRequests:           requestAccess,
		AppServers:               appServerAccess,
		DBServers:                dbServerAccess,
		DB:                       dbAccess,
		ReviewRequests:           reviewRequests,
		KubeServers:              kubeServerAccess,
		Desktops:                 desktopAccess,
		AuthConnectors:           authConnectors,
		TrustedClusters:          trustedClusterAccess,
		ClusterMaintenanceConfig: clusterMaintenanceConfig,
		RecordedSessions:         recordedSessionAccess,
		ActiveSessions:           activeSessionAccess,
		Roles:                    roleAccess,
		Events:                   eventAccess,
		Users:                    userAccess,
		Tokens:                   tokenAccess,
		Nodes:                    nodeAccess,
		Billing:                  billingAccess,
		ConnectionDiagnostic:     cnDiagnosticAccess,
		Clipboard:                clipboard,
		DesktopSessionRecording:  desktopSessionRecording,
		DirectorySharing:         directorySharing,
		Download:                 download,
		License:                  license,
		Plugins:                  pluginsAccess,
		Integrations:             integrationsAccess,
		UserTasks:                userTasksAccess,
		DiscoveryConfig:          discoveryConfigsAccess,
		DeviceTrust:              deviceTrust,
		Locks:                    lockAccess,
		SAMLIdpServiceProvider:   samlIdpServiceProviderAccess,
		AccessList:               accessListAccess,
		AuditQuery:               auditQuery,
		SecurityReport:           securityReports,
		ExternalAuditStorage:     externalAuditStorage,
		AccessGraph:              accessGraphAccess,
		Bots:                     bots,
		BotInstances:             botInstances,
		AccessMonitoringRule:     accessMonitoringRules,
		CrownJewel:               crownJewelAccess,
		AccessGraphSettings:      accessGraphSettings,
		Contact:                  contact,
		FileTransferAccess:       fileTransferAccess,
		GitServers:               gitServersAccess,
	}
}
