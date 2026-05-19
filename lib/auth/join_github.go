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
	"github.com/gravitational/teleport/lib/join/githubactions"
)

// GetGHAIDTokenValidator returns the validator implementation for GitHub
// OIDC tokens.
func (a *Server) GetGHAIDTokenValidator() githubactions.GithubIDTokenValidator {
	return a.ghaIDTokenValidator
}

// SetGHAIDTokenValidator sets the validator implementation for GitHub OIDC
// tokens, used in tests.
func (a *Server) SetGHAIDTokenValidator(validator githubactions.GithubIDTokenValidator) {
	// Note: Unfortunately tests now live in lib/join/ so exporting these
	// test-only functions in exporter_test.go isn't sufficient.
	a.ghaIDTokenValidator = validator
}

// GetGHAIDTokenJWKSValidator returns the validator implementation for GitHub
// OIDC tokens. This validator is for static JWKS cases where OIDC endpoints are
// not accessible to Teleport.
func (a *Server) GetGHAIDTokenJWKSValidator() githubactions.GithubIDTokenJWKSValidator {
	return a.ghaIDTokenJWKSValidator
}

// SetGHAIDTokenJWKSValidator returns the validator implementation for GitHub
// OIDC tokens. This validator is for static JWKS cases where OIDC endpoints are
// not accessible to Teleport. Used in tests.
func (a *Server) SetGHAIDTokenJWKSValidator(validator githubactions.GithubIDTokenJWKSValidator) {
	a.ghaIDTokenJWKSValidator = validator
}

func (a *Server) checkGitHubJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*githubactions.IDTokenClaims, error) {
	claims, err := githubactions.CheckGithubIDToken(ctx, &githubactions.CheckGithubIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Clock:          a.GetClock(),
		Validator:      a.ghaIDTokenValidator,
		JWKSValidator:  a.ghaIDTokenJWKSValidator,
	})

	return claims, trace.Wrap(err)
}
