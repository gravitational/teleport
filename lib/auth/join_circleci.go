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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/circleci"
)

// GetCircleCITokenValidate returns the currently configured CircleCI OIDC token
// validator.
func (a *Server) GetCircleCITokenValidator() circleci.Validator {
	return a.circleCITokenValidate
}

// SetCircleCITokenValidate sets the CircleCI OIDC token validator
// implementation. Used in tests.
func (a *Server) SetCircleCITokenValidator(validator circleci.Validator) {
	a.circleCITokenValidate = validator
}

func (a *Server) checkCircleCIJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*circleci.IDTokenClaims, error) {
	claims, err := circleci.CheckIDToken(ctx, &circleci.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Validator:      a.circleCITokenValidate,
	})

	// Note: try to return claims even if there was an error, they may provide
	// useful auditing context.
	return claims, trace.Wrap(err)
}
