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

package usersv1

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// Okta-origin users have some special access rules that are implemented in this
// file:
//
// 1. Only the Teleport Okta service may create an Okta-origin user.
// 2. Only the Teleport Okta service may modify an Okta-origin user.
// 3. Anyone with User RW can delete an Okta-origin user (otherwise there is no
//    recourse for a Teleport admin to clean up obsolete users if an Okta
//    integration is deleted.
//
// The implementation of these rules is spread through the functions below

// checkOktaOrigin checks that the supplied user has an appropriate origin label
// set. In this case "appropriate" means having the Okta origin set if and only
// if the supplied auth context has the build-in Okta role. An auth context
// without the Okta role may supply any origin value *other than* okta
// (including nil).
// Returns an error if the user origin value is "inappropriate".
func CheckOktaOrigin(authzCtx *authz.Context, user types.User) error {
	isOktaService := authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta))
	hasOktaOrigin := user.Origin() == types.OriginOkta

	switch {
	case isOktaService && !hasOktaOrigin:
		return trace.BadParameter(`Okta service must supply "okta" origin`)

	case !isOktaService && hasOktaOrigin:
		return trace.BadParameter(`Must be Okta service to set "okta" origin`)

	default:
		return nil
	}
}

// CheckOktaAccess gates access to update operations on user records based
// on the origin label on the supplied user record.
//
// A nil `existingUser` is interpreted as there being no matching existing
// user in the cluster; if there is no user then there is no user to
// overwrite, so access is granted
func CheckOktaAccess(authzCtx *authz.Context, existingUser types.User, verb string) error {
	// We base or decision to allow write access to a resource on the Origin
	// label. If there is no existing user, then there can be no label to block
	// access, so anyone can do anything.
	if existingUser == nil {
		return nil
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta)) {
		// The only thing a non-okta service caller is allowed to do to an
		// Okta-origin user is delete it
		if (existingUser.Origin() == types.OriginOkta) && (verb != types.VerbDelete) {
			return trace.BadParameter("Okta origin may not be changed")
		}
		return nil
	}

	// An okta-service caller only has rights over the user if they have an
	// "Origin: Okta" label
	if existingUser.Origin() == types.OriginOkta {
		return nil
	}

	// If we get to here, we have exhausted all possible ways that the caller
	// may be allowed to modify a user, so they get AccessDenied by default.
	return trace.AccessDenied("Okta service may only %s Okta users", verb)
}
