/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package join

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/join/genericoidc"
	"github.com/gravitational/teleport/lib/join/provision"
)

type GenericOIDCTokenValidator interface {
	ValidateToken(
		ctx context.Context,
		provisionToken provision.Token,
		idToken []byte,
	) (*genericoidc.IDTokenClaims, error)
}

// validateGenericOIDCToken performs OIDC token verification for generic OIDC JWTs,
// suitable for use in `handleOIDCJoin`
func (s *Server) validateGenericOIDCToken(
	ctx context.Context,
	provisionToken provision.Token,
	idToken []byte,
) (any, *workloadidentityv1.JoinAttrs, error) {
	// Validator errors may contain sensitive info. Eventually we want to plumb
	// the raw error into the audit log, but for now we'll log it and mask the
	// error from the client.
	verifiedIdentity, err := s.cfg.AuthService.GetGenericOIDCIDTokenValidator().ValidateToken(ctx, provisionToken, idToken)
	if err != nil {
		s.cfg.Logger.WarnContext(ctx, "denying generic_oidc join attempt", "error", err)
		return nil, nil, trace.AccessDenied("unable to join via generic_oidc")
	}

	// Implementation note: there's no explicit attribute validation step for
	// this join method in the join server; it's internal to ValidateToken().

	attrs, err := verifiedIdentity.JoinAttrs()
	if err != nil {
		return nil, nil, trace.Wrap(err, "converting join attrs")
	}

	return verifiedIdentity, workloadidentityv1.JoinAttrs_builder{
		GenericOidc: attrs,
	}.Build(), nil
}
