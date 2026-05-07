/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/subca"
)

// Handlers returns a map of Handler per kind.
// This map will be filled as we convert existing resources
// to the Handler format.
func Handlers() map[string]Handler {
	// When adding resources, please keep the map alphabetically ordered.
	m := map[string]Handler{
		types.KindAccessGraphSettings:                accessGraphSettingsHandler(),
		types.KindAccessList:                         accessListHandler(),
		types.KindAccessMonitoringRule:               accessMonitoringRuleHandler(),
		types.KindAccessRequest:                      accessRequestHandler(),
		types.KindApp:                                appHandler(),
		types.KindAppServer:                          appServerHandler(),
		types.KindAuditQuery:                         auditQueryHandler(),
		types.KindAuthServer:                         authHandler(),
		types.KindAutoUpdateAgentReport:              autoUpdateAgentReportHandler(),
		types.KindAutoUpdateAgentRollout:             autoUpdateAgentRolloutHandler(),
		types.KindAutoUpdateBotInstanceReport:        autoUpdateBotInstanceReportHandler(),
		types.KindAutoUpdateConfig:                   autoUpdateConfigHandler(),
		types.KindAutoUpdateVersion:                  autoUpdateVersionHandler(),
		types.KindBot:                                botHandler(),
		types.KindBotInstance:                        botInstanceHandler(),
		types.KindCertAuthority:                      certAuthorityHandler(),
		types.KindClusterAuthPreference:              authPreferenceHandler(),
		types.KindClusterMaintenanceConfig:           clusterMaintenanceConfigHandler(),
		types.KindClusterNetworkingConfig:            networkingConfigHandler(),
		types.KindConnectors:                         connectorsHandler(),
		types.KindDatabase:                           databaseHandler(),
		types.KindDatabaseObject:                     databaseObjectHandler(),
		types.KindDatabaseObjectImportRule:           databaseObjectImportRuleHandler(),
		types.KindDiscoveryConfig:                    discoveryConfigHandler(),
		types.KindDynamicWindowsDesktop:              dynamicWindowsDesktopHandler(),
		types.KindExternalAuditStorage:               externalAuditStorageHandler(),
		types.KindGithubConnector:                    githubConnectorHandler(),
		types.KindGitServer:                          gitServerHandler(),
		types.KindInferenceModel:                     inferenceModelHandler(),
		types.KindInferenceSecret:                    inferenceSecretHandler(),
		types.KindInferencePolicy:                    inferencePolicyHandler(),
		types.KindRetrievalModel:                     retrievalModelHandler(),
		types.KindInstaller:                          installerHandler(),
		types.KindKubeServer:                         kubeServerHandler(),
		types.KindKubernetesCluster:                  kubeClusterHandler(),
		types.KindLinuxDesktop:                       linuxDesktopHandler(),
		types.KindLock:                               lockHandler(),
		types.KindNode:                               serverHandler(),
		types.KindOIDCConnector:                      oidcConnectorHandler(),
		types.KindProxy:                              proxyHandler(),
		types.KindRelayServer:                        relayServerHandler(),
		types.KindRole:                               roleHandler(),
		types.KindSAMLConnector:                      samlConnectorHandler(),
		types.KindSAMLIdPServiceProvider:             samlIdPServiceProviderHandler(),
		types.KindServerInfo:                         serverInfoHandler(),
		types.KindSessionRecordingConfig:             sessionRecordingConfigHandler(),
		types.KindSigstorePolicy:                     sigstorePolicyHandler(),
		types.KindSPIFFEFederation:                   spiffeFederationHandler(),
		types.KindStaticHostUser:                     staticHostUserHandler(),
		types.KindToken:                              tokenHandler(),
		types.KindUIConfig:                           uiConfigHandler(),
		types.KindUser:                               userHandler(),
		types.KindUserTask:                           userTasksHandler(),
		types.KindVnetConfig:                         vnetConfigHandler(),
		types.KindWindowsDesktop:                     windowsDesktopHandler(),
		types.KindWindowsDesktopService:              windowsDesktopServiceHandler(),
		types.KindWorkloadIdentity:                   workloadIdentityHandler(),
		types.KindWorkloadIdentityX509IssuerOverride: workloadIdentityX509IssuerOverrideHandler(),
		types.KindWorkloadIdentityX509Revocation:     workloadIdentityX509RevocationHandler(),
		types.KindAppAuthConfig:                      appAuthConfigHandler(),
		scopedaccess.KindScopedRole:                  scopedRoleHandler(),
		scopedaccess.KindScopedRoleAssignment:        scopedRoleAssignmentHandler(),
		types.KindWorkloadCluster:                    workloadClusterHandler(),
		scopedaccess.KindScopedToken:                 scopedTokenHandler(),
	}

	if subca.Enabled() {
		m[types.KindCertAuthorityOverride] = certAuthorityOverrideHandler()
	}

	return m
}

// Handler represents a resource supported by the tctl resource command.
// It contains all the information about the resources and the functions
// to create, update, get and delete it.
// Some resources might not implement all functions (e.g. some resources are
// read-only, they cannot be created).
type Handler struct {
	// getHandler powers "tctl get {kind}" and its variants "{kind}/{name}" and
	// "{kind}/{subkind}/{name}".
	// Make sure to inspect the services.Ref and implement as many modes as
	// appropriate (typically, List and Get).
	getHandler func(context.Context, *authclient.Client, services.Ref, GetOpts) (Collection, error)
	// createHandler powers "tctl create", "tctl create -f" and "tctl edit"
	// (fallback from a nil updateHandler).
	createHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	// updateHandler powers "tctl edit".
	updateHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	// deleteHandler powers "tctl rm".
	deleteHandler func(context.Context, *authclient.Client, services.Ref) error
	// singleton informs tctl whether the resource is a single instance, instead
	// of a collection of resources.
	// Examples of singleton resources include cluster_auth_preference and
	// session_recording_config.
	singleton bool
	// mfaRequired informs "tctl get" whether the resource is read sensitive
	// (exposes secrets or otherwise requires admin MFA protection).
	mfaRequired bool
	// description is a resource description used by "tctl list-kinds".
	description string
}

// GetOpts contains the possible options when getting a resource.
type GetOpts struct {
	// WithSecrets is true if the user set --with-secrets
	WithSecrets bool
}

// CreateOpts contains the possible options when creating/updating a resource.
type CreateOpts struct {
	// Force is true if the user set --Force
	Force bool
	// Confirm is true if the user set --Confirm
	Confirm bool
}

// Get queries the cluster to get the desired resource and returns a Collection.
// Getting with an empty ref.Name returns all resources of the specified ref.Kind.
func (r *Handler) Get(ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if r.getHandler == nil {
		return nil, trace.NotImplemented("resource does not support 'tctl get'")
	}
	return r.getHandler(ctx, clt, ref, opts)
}

// Create takes a raw resource manifest, decodes it, and creates the
// corresponding resource in Teleport.
func (r *Handler) Create(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if r.createHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl create'")
	}
	return r.createHandler(ctx, clt, raw, opts)
}

// Update takes a raw resource manifest, decodes it, and updates the
// corresponding resource in Teleport.
func (r *Handler) Update(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if r.updateHandler == nil {
		return trace.NotImplemented("resource does not have an update handler")
	}
	return r.updateHandler(ctx, clt, raw, opts)
}

// Delete takes a resource kind and name, and deletes the corresponding resource
// in Teleport.
func (r *Handler) Delete(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	if r.deleteHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl rm'")
	}
	if ref.Name == "" && !r.singleton {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}
	return r.deleteHandler(ctx, clt, ref)
}

// MFARequired indicates that this resource requires MFA to Get the resource.
func (r *Handler) MFARequired() bool {
	return r.mfaRequired
}

// SupportedCommands returns the list of supported tctl commands for this resource Handler.
func (r *Handler) SupportedCommands() []string {
	var verbs []string
	if r.getHandler != nil {
		verbs = append(verbs, "get")
	}
	if r.createHandler != nil {
		verbs = append(verbs, "create")
	}
	if r.deleteHandler != nil {
		verbs = append(verbs, "rm")
	}
	// No check on the update handler for the "update" command because it is not
	// doing anything useful today: https://github.com/gravitational/teleport/issues/61381

	return verbs
}

// Description returns the description of the Handler's resource.
// The description is intended for aim users to understand what this resource
// does and in which case they should interact with it.
func (r *Handler) Description() string {
	return r.description
}

// Singleton indicates if the handled resource is a singleton (only one can
// exist in the Teleport cluster).
func (r *Handler) Singleton() bool {
	return r.singleton
}

// upsertVerb generates the correct string form of a verb based on the action taken
func upsertVerb(exists bool, force bool) string {
	if !force && exists {
		return "updated"
	}
	return "created"
}
