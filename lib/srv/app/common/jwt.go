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

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/tlsca"
)

// AppTokenGenerator defines an interface for generating JWT token for an
// application (by auth).
type AppTokenGenerator interface {
	GenerateAppToken(context.Context, types.GenerateAppTokenRequest) (string, error)
}

// GenerateJWTAndTraits is helper that generates a JWT for an application and
// populates the user traits with the result JWT for templating.
func GenerateJWTAndTraits(
	ctx context.Context,
	identity *tlsca.Identity,
	app types.Application,
	generator AppTokenGenerator,
) (string, wrappers.Traits, error) {
	rewrite := app.GetRewrite()
	traits := identity.Traits
	roles := identity.Groups
	if rewrite != nil {
		switch rewrite.JWTClaims {
		case types.JWTClaimsRewriteNone:
			traits = nil
			roles = nil
		case types.JWTClaimsRewriteRoles:
			traits = nil
		case types.JWTClaimsRewriteTraits:
			roles = nil
		case "", types.JWTClaimsRewriteRolesAndTraits:
		}
	}

	// Request a JWT token that will be attached to all requests.
	jwt, err := generator.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username: identity.Username,
		Roles:    roles,
		Traits:   traits,
		URI:      app.GetURI(),
		Expires:  identity.Expires,
	})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	if traits == nil {
		traits = make(wrappers.Traits)
	}
	traits[constants.TraitJWT] = []string{jwt}
	return jwt, traits, trace.Wrap(err)
}
