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
	"github.com/gravitational/teleport/lib/services"
)

// Handlers returns a map of Handler per kind.
// This map will be filled as we convert existing resources
// to the Handler format.
func Handlers() map[string]Handler {
	// When adding resources, please keep the map alphabetically ordered.
	return map[string]Handler{
		types.KindApp:                                appHandler(),
		types.KindAppServer:                          appServerHandler(),
		types.KindAuthServer:                         authHandler(),
		types.KindAutoUpdateBotInstanceReport:        autoUpdateBotInstanceReportHandler(),
		types.KindBot:                                botHandler(),
		types.KindBotInstance:                        botInstanceHandler(),
		types.KindDatabase:                           databaseHandler(),
		types.KindLock:                               lockHandler(),
		types.KindNode:                               serverHandler(),
		types.KindProxy:                              proxyHandler(),
		types.KindRole:                               roleHandler(),
		types.KindSigstorePolicy:                     sigstorePolicyHandler(),
		types.KindSPIFFEFederation:                   spiffeFederationHandler(),
		types.KindUser:                               userHandler(),
		types.KindWorkloadIdentity:                   workloadIdentityHandler(),
		types.KindWorkloadIdentityX509IssuerOverride: workloadIdentityX509IssuerOverrideHandler(),
		types.KindWorkloadIdentityX509Revocation:     workloadIdentityX509RevocationHandler(),
	}
}

// Handler represents a resource supported by the tctl resource command.
// It contains all the information about the resources and the functions
// to create, update, get and delete it.
// Some resources might not implement all functions (e.g. some resources are
// read-only, they cannot be created).
type Handler struct {
	getHandler    func(context.Context, *authclient.Client, services.Ref, GetOpts) (Collection, error)
	createHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	updateHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	deleteHandler func(context.Context, *authclient.Client, services.Ref) error
	singleton     bool
	mfaRequired   bool
	description   string
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
		return trace.NotImplemented("resource does not support 'tctl delete'")
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
	if r.updateHandler != nil {
		verbs = append(verbs, "update")
	}

	return verbs
}

// Description returns the description of the Handler's resource.
// The description is intended for aim users to understand what this resource
// does and in which case they should interact with it.
func (r *Handler) Description() string {
	return r.description
}

// upsertVerb generates the correct string form of a verb based on the action taken
func upsertVerb(exists bool, force bool) string {
	if !force && exists {
		return "updated"
	}
	return "created"
}
