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
	"github.com/gravitational/teleport/lib/join/gitlab"
)

// GetGitlabIDTokenValidator returns the currently configured gitlab OIDC token
// validator.
func (a *Server) GetGitlabIDTokenValidator() gitlab.Validator {
	return a.gitlabIDTokenValidator
}

// SetGitlabIDTokenValidator sets the validator implementation used to verify
// GitLab OIDC tokens. Used in tests to provide mock implementations.
func (a *Server) SetGitlabIDTokenValidator(validator gitlab.Validator) {
	a.gitlabIDTokenValidator = validator
}

func (a *Server) checkGitLabJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*gitlab.IDTokenClaims, error) {
	claims, err := gitlab.CheckIDToken(ctx, &gitlab.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Validator:      a.gitlabIDTokenValidator,
	})

	// Where possible, we try to return any extracted claims along with the
	// error to provide better audit logs of failed join attempts.
	return claims, trace.Wrap(err)
}
