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

package okta

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	oktaplugin "github.com/gravitational/teleport/lib/okta/plugin"
	"github.com/gravitational/teleport/lib/services"
)

// Okta-origin resources have some special access rules that are implemented in
// this file:
//
// 1. Only the Teleport Okta service may create an Okta-origin resource.
// 2. Only the Teleport Okta service may modify an Okta-origin resource.
// 3. Anyone with User RW can delete an Okta-origin resource (otherwise there is
//    no recourse for a Teleport admin to clean up obsolete resources if an Okta
//    integration is deleted.
//
// The implementation of these rules is spread through the functions below

// CheckOrigin checks that the supplied resource has an appropriate origin label
// set. In this case "appropriate" means having the Okta origin set if and only
// if the supplied auth context has the built-in Okta role. An auth context
// without the Okta role may supply any origin value *other than* okta
// (including nil).
// Returns an error if the user origin value is "inappropriate".
func CheckOrigin(authzCtx *authz.Context, res types.ResourceWithLabels) error {
	isOktaService := authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta))
	hasOktaOrigin := res.Origin() == types.OriginOkta

	switch {
	case isOktaService && !hasOktaOrigin:
		return trace.BadParameter(`Okta service must supply "okta" origin`)

	case !isOktaService && hasOktaOrigin:
		return trace.BadParameter(`Must be Okta service to set "okta" origin`)

	default:
		return nil
	}
}

// CheckAccess gates access to update operations on resource records based
// on the origin label on the supplied resource.
//
// A nil `existingResource` is interpreted as there being no matching existing
// resource in the cluster; if there is no user then there is no resource to
// overwrite, so access is granted
func CheckAccess(authzCtx *authz.Context, existingResource types.ResourceWithLabels, verb string) error {
	// We base or decision to allow write access to a resource on the Origin
	// label. If there is no existing user, then there can be no label to block
	// access, so anyone can do anything.
	if existingResource == nil {
		return nil
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta)) {
		// The only thing a non-okta service caller is allowed to do to an
		// Okta-origin user is delete it
		if (existingResource.Origin() == types.OriginOkta) && (verb != types.VerbDelete) {
			return trace.BadParameter("Okta origin may not be changed")
		}
		return nil
	}

	// An okta-service caller only has rights over the resource if that resource
	// has an "Origin: Okta" label
	if existingResource.Origin() == types.OriginOkta {
		return nil
	}

	// If we get to here, we have exhausted all possible ways that the caller
	// may be allowed to modify a resource, so they get AccessDenied by
	// default.
	return trace.AccessDenied("Okta service may only %s Okta resources", verb)
}

// BidirectionalSyncEnabled checks if the bidirectional sync is enabled on the Okta plugin.  If the
// Okta plugin does not exist the result is false.
func BidirectionalSyncEnabled(ctx context.Context, plugins services.Plugins) (bool, error) {
	plugin, err := oktaplugin.Get(ctx, plugins, false /* withSecrets */)
	if trace.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, trace.Wrap(err, "getting Okta plugin")
	}
	return plugin.Spec.GetOkta().GetSyncSettings().GetEnableBidirectionalSync(), nil
}

// AccessPoint provides services required by [CheckResourcesRequestable].
type AccessPoint struct {
	Plugins              services.Plugins
	UnifiedResourceCache *services.UnifiedResourceCache
}

var (
	// OktaResourceNotRequestableError means resources cannot be requested because Okta
	// bidirectional sync is disabled.
	OktaResourceNotRequestableError = trace.BadParameter("Okta resources not requestable due to disabled bidirectional sync")
)

// CheckResourcesRequestable checks if the provided resources are requestable from the Okta integration point of view.
//
// Resources are considered not requestable and AccessDenied error is returned when:
//
//   - bidirectional sync is disabled in the Okta plugin
//   - any of the resources is an Okta-originated app_server
//
// If any resource is not requestable, [OktaResourceNotRequestableError] will be returned. Any
// other error can be returned.
func CheckResourcesRequestable(ctx context.Context, ids []types.ResourceID, ap AccessPoint) error {
	bidirectionalSyncEnabled, err := BidirectionalSyncEnabled(ctx, ap.Plugins)
	if err != nil {
		return trace.Wrap(err, "getting bidirectional sync")
	}
	if bidirectionalSyncEnabled {
		return nil
	}

	var denied []types.ResourceID
	for app, err := range ap.UnifiedResourceCache.AppServers(ctx, services.UnifiedResourcesIterateParams{}) {
		if err != nil {
			return trace.Wrap(err, "iterating cached AppServers")
		}
		for _, id := range ids {
			switch id.Kind {
			case types.KindApp, types.KindAppServer:
				// ok
			default:
				continue
			}
			if app.GetName() == id.Name && app.Origin() == types.OriginOkta {
				denied = append(denied, id)
			}
		}
	}

	if len(denied) == 0 {
		return nil
	}

	resourcesStr, err := types.ResourceIDsToString(denied)
	if err != nil {
		return trace.Wrap(err, "converting resource IDs to string")
	}

	return trace.Wrap(OktaResourceNotRequestableError, "requested Okta resources: %s", resourcesStr)
}
