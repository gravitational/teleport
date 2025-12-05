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

package mcp

import (
	"context"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

type rewriteAuthDetails struct {
	rewriteAuthHeader bool
	hasIDTokenTrait   bool
	hasJWTTrait       bool
}

// rewriteTraitsTest are fake traits used for testing if {{internal.jwt}} and
// {{internal.id_token}} are defined in app rewrite. The number of elements in
// the slice can be used to quickly tell which traits have been applied.
var rewriteTraitsTest = wrappers.Traits{
	constants.TraitJWT:     {"j", "w", "t"},
	constants.TraitIDToken: {"i", "d"},
}

func newRewriteAuthDetails(rewrite *types.Rewrite) rewriteAuthDetails {
	if rewrite == nil {
		return rewriteAuthDetails{}
	}

	var r rewriteAuthDetails
	for _, header := range rewrite.Headers {
		if strings.EqualFold(header.Name, "Authorization") {
			r.rewriteAuthHeader = true
		}

		interpolated, _ := services.ApplyValueTraits(header.Value, rewriteTraitsTest)
		switch len(interpolated) {
		case 3:
			r.hasJWTTrait = true
		case 2:
			r.hasIDTokenTrait = true
		}
	}
	return r
}

func generateIDToken(ctx context.Context, identity *tlsca.Identity, app types.Application, auth AuthClient) (string, error) {
	roles, traits := appcommon.RolesAndTraitsForAppToken(identity, app)

	// Use types.OIDCIdPCA to generate the token.
	idToken, err := auth.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username:      identity.Username,
		Roles:         roles,
		Traits:        traits,
		URI:           app.GetURI(),
		Expires:       identity.Expires,
		AuthorityType: types.OIDCIdPCA,
	})
	return idToken, trace.Wrap(err)
}
