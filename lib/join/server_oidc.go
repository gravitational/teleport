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

package join

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
)

// oidcTokenValidator is a function type that validates an OIDC token and checks that
// it matches an allow rule configured in the provision token.
type oidcTokenValidator func(
	ctx context.Context,
	provisionToken provision.Token,
	idToken []byte,
) (rawClaims any, joinAttrs *workloadidentityv1.JoinAttrs, err error)

// handleOIDCJoin handles join attempts for all generic OIDC join methods.
func (s *Server) handleOIDCJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken provision.Token,
	validator oidcTokenValidator,
) (messages.Response, error) {
	// Receive the OIDCInit message from the client.
	oidcInit, err := messages.RecvRequest[*messages.OIDCInit](stream)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &oidcInit.ClientParams)

	rawClaims, joinAttrs, err := validator(stream.Context(), provisionToken, oidcInit.IDToken)
	stream.Diagnostic().Set(func(info *diagnostic.Info) {
		info.RawJoinAttrs = rawClaims
	})
	if err != nil {
		return nil, trace.Wrap(err, "verifying OIDC token")
	}

	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&oidcInit.ClientParams,
		provisionToken,
		rawClaims,
		joinAttrs,
	)
	return result, trace.Wrap(err)
}
