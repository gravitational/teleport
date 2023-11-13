// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usersv1

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// checkOktaOrigin checks that the supplied user has an appropriate origin label
// set. In this case "appropriate" means having the Okta origin set if and only
// if the supplied auth context has the build-in Okta role. Context without the
// Okta role may supply any origin value *other than* okta (including nil).
// Returns an error if the user origin value is "inappropriate".
func checkOktaOrigin(authzCtx *authz.Context, user types.User, verb string) error {
	isOktaService := authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta))
	hasOktaOrigin := user.Origin() == types.OriginOkta

	switch {
	case isOktaService && !hasOktaOrigin:
		return trace.BadParameter(`Okta service must supply "okta" origin`)

	case (verb == types.VerbCreate) && !isOktaService && hasOktaOrigin:
		return trace.BadParameter("Must be Okta service to set Okta origin")

	case !isOktaService && hasOktaOrigin:
		return nil

	default:
		return nil
	}
}

// checkOktaAccess gates access to update operations on user records based
// on the origin labels of the existing and new user records.
//
//   - a nil `existingUser` is interpreted as there being no matching existing
//     user in the cluster; if there is no user then there is no user to
//     overwrite, so access is grated
//   - when `authzCtx` represents a non-okta caller, then the only disallowed
//     operation is removing an "Origin: okta" label
//   - when `authzCtx` represents the okta service, then access is granted if and
//     only if the existing user has an "Origin: okta" label
func checkOktaAccess(authzCtx *authz.Context, newUser, existingUser types.User, verb string) error {
	// We base or decision to allow write access to a resource on the Origin
	// label. If there is no existing user, then there can be no label to block
	// access, so anyone can do anything.
	if existingUser == nil {
		return nil
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta)) {
		// The only thing a non-okta service caller is prevented from doing is
		// changing a user record's "Origin: Okta" label - everything else is
		// fair game.
		if existingUser.Origin() == types.OriginOkta {
			if newUser.Origin() != types.OriginOkta {
				return trace.BadParameter("Okta origin may not be changed")
			}
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
