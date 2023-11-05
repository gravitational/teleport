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
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// checkOktaUpdateAccess tests that, if the identity provided in authzCtx has
// the built-in Okta role, that the "origin: Okta" label is present on both the
// existing user resource and the new value it will take after any update
// completes. If either of these records is missing the label, this function
// returns an error.
func (s *Service) checkOktaUpdateAccess(ctx context.Context, authzCtx *authz.Context, new types.User, existingUsername string) error {
	if authz.HasBuiltinRole(*authzCtx, string(types.RoleOkta)) {
		// Check that the caller is not trying to erase the okta origin label
		if !hasOriginOkta(new) {
			return trace.BadParameter("Users updated by the Okta service must include an Okta origin label")
		}

		// Check that the resource-to-be-updated is managed by Okta
		if err := isOktaWriteableUserResource(ctx, s.cache, existingUsername); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// isOktaWriteableUserResource tests if the existing record for a user (if any)
// is writable for the Okta service. Writability is conferred by having the
// "Origin: Okta" label
func isOktaWriteableUserResource(ctx context.Context, cache Cache, username string) error {
	targetUser, err := cache.GetUser(ctx, username, false)
	switch {
	case trace.IsNotFound(err):
		// If no such user exists, then we will treat it as writable. Otherwise
		// we won't be able to create the user on an upsert.
		return nil

	case err != nil:
		return trace.Wrap(err)
	}

	if hasOriginOkta(targetUser) {
		return nil
	}

	return trace.AccessDenied("Okta service may only update okta users")
}

// hasOriginOkta tests that a resource has the "Source: Okta" label, which
// implies that it is under the control of the Okta service and may be
// manipulated by it.
func hasOriginOkta(r types.Resource) bool {
	if resourceOrigin, present := r.GetMetadata().Labels[types.OriginLabel]; present {
		return resourceOrigin == types.OriginOkta
	}
	return false
}
