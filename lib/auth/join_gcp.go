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
	"github.com/gravitational/teleport/lib/join/gcp"
)

// GetGCPIDTokenValidator returns the server's configured GCP ID token
// validator.
func (a *Server) GetGCPIDTokenValidator() gcp.Validator {
	return a.gcpIDTokenValidator
}

// SetGCPIDTokenValidator sets a new GCP ID token validator, used in tests.
func (a *Server) SetGCPIDTokenValidator(validator gcp.Validator) {
	a.gcpIDTokenValidator = validator
}

func (a *Server) checkGCPJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*gcp.IDTokenClaims, error) {
	claims, err := gcp.CheckIDToken(ctx, &gcp.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Validator:      a.gcpIDTokenValidator,
	})

	// Where possible, try to return any extracted claims along with the error
	// to improve audit logs for failed join attempts.
	return claims, trace.Wrap(err)
}
