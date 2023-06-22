/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
	// DeviceTrust defines access to device trust.
	DeviceTrust ResourceAccess `json:"deviceTrust"`
	// Locks defines access to locking resources.
	Locks ResourceAccess `json:"lock"`
	// Assist defines access to assist feature.
	Assist ResourceAccess `json:"assist"`
	// SAMLIdpServiceProvider defines access to `saml_idp_service_provider` objects.
	SAMLIdpServiceProvider ResourceAccess `json:"samlIdpServiceProvider"`
}

func hasAccess(roleSet RoleSet, ctx *Context, kind string, verbs ...string) bool {
	for _, verb := range verbs {
		// Since this check occurs often and does not imply the caller is trying to
		// ResourceAccess any resource, silence any logging done on the proxy.
		if err := roleSet.GuessIfAccessIsPossible(ctx, apidefaults.Namespace, kind, verb, true); err != nil {
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

func NewUserACL(user types.User, userRoles RoleSet, features proto.Features, desktopRecordingEnabled bool) UserACL {
	ctx := &Context{User: user}
	recordedSessionAccess := newAccess(userRoles, ctx, types.KindSession)
	activeSessionAccess := newAccess(userRoles, ctx, types.KindSSHSession)
	roleAccess := newAccess(userRoles, ctx, types.KindRole)
	authConnectors := newAccess(userRoles, ctx, types.KindAuthConnector)
	trustedClusterAccess := newAccess(userRoles, ctx, types.KindTrustedCluster)
	eventAccess := newAccess(userRoles, ctx, types.KindEvent)
	userAccess := newAccess(userRoles, ctx, types.KindUser)
	tokenAccess := newAccess(userRoles, ctx, types.KindToken)
	nodeAccess := newAccess(userRoles, ctx, types.KindNode)
	appServerAccess := newAccess(userRoles, ctx, types.KindAppServer)
	dbServerAccess := newAccess(userRoles, ctx, types.KindDatabaseServer)
	dbAccess := newAccess(userRoles, ctx, types.KindDatabase)
	kubeServerAccess := newAccess(userRoles, ctx, types.KindKubeServer)
	requestAccess := newAccess(userRoles, ctx, types.KindAccessRequest)
	desktopAccess := newAccess(userRoles, ctx, types.KindWindowsDesktop)
	cnDiagnosticAccess := newAccess(userRoles, ctx, types.KindConnectionDiagnostic)
	samlIdpServiceProviderAccess := newAccess(userRoles, ctx, types.KindSAMLIdPServiceProvider)

	var assistAccess ResourceAccess
	if features.Assist {
		assistAccess = newAccess(userRoles, ctx, types.KindAssistant)
	}

	var billingAccess ResourceAccess
	if features.Cloud {
		billingAccess = newAccess(userRoles, ctx, types.KindBilling)
	}

	var pluginsAccess ResourceAccess
	if features.Plugins {
		pluginsAccess = newAccess(userRoles, ctx, types.KindPlugin)
	}

	clipboard := userRoles.DesktopClipboard()
	desktopSessionRecording := desktopRecordingEnabled && userRoles.RecordDesktopSession()
	directorySharing := userRoles.DesktopDirectorySharing()
	download := newAccess(userRoles, ctx, types.KindDownload)
	license := newAccess(userRoles, ctx, types.KindLicense)
	deviceTrust := newAccess(userRoles, ctx, types.KindDevice)
	integrationsAccess := newAccess(userRoles, ctx, types.KindIntegration)
	lockAccess := newAccess(userRoles, ctx, types.KindLock)

	return UserACL{
		AccessRequests:          requestAccess,
		AppServers:              appServerAccess,
		DBServers:               dbServerAccess,
		DB:                      dbAccess,
		KubeServers:             kubeServerAccess,
		Desktops:                desktopAccess,
		AuthConnectors:          authConnectors,
		TrustedClusters:         trustedClusterAccess,
		RecordedSessions:        recordedSessionAccess,
		ActiveSessions:          activeSessionAccess,
		Roles:                   roleAccess,
		Events:                  eventAccess,
		Users:                   userAccess,
		Tokens:                  tokenAccess,
		Nodes:                   nodeAccess,
		Billing:                 billingAccess,
		ConnectionDiagnostic:    cnDiagnosticAccess,
		Clipboard:               clipboard,
		DesktopSessionRecording: desktopSessionRecording,
		DirectorySharing:        directorySharing,
		Download:                download,
		License:                 license,
		Plugins:                 pluginsAccess,
		Integrations:            integrationsAccess,
		DeviceTrust:             deviceTrust,
		Locks:                   lockAccess,
		Assist:                  assistAccess,
		SAMLIdpServiceProvider:  samlIdpServiceProviderAccess,
	}
}
