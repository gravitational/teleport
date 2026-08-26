/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/join/bitbucket"
)

// GetBitbucketIDTokenValidator returns the currently configured token validator
// for Bitbucket.
func (a *Server) GetBitbucketIDTokenValidator() bitbucket.Validator {
	return a.bitbucketIDTokenValidator
}

// SetBitbucketIDTokenValidator sets the current Bitbucket token validator
// implementation. Used in tests.
func (a *Server) SetBitbucketIDTokenValidator(validator bitbucket.Validator) {
	a.bitbucketIDTokenValidator = validator
}

func (a *Server) checkBitbucketJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*bitbucket.IDTokenClaims, error) {
	claims, err := bitbucket.CheckIDToken(ctx, &bitbucket.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Clock:          a.GetClock(),
		Validator:      a.bitbucketIDTokenValidator,
	})

	// Note: We try to return claims regardless of whether or not an error is
	// returned for downstream logging.
	return claims, trace.Wrap(err)
}
