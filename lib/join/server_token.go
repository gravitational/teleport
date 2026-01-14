// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package join

import (
	"context"
	"crypto/subtle"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

// handleTokenJoin handles join attempts for the token join method.
func (s *Server) handleTokenJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	token provision.Token,
) (messages.Response, error) {
	// Receive the TokenInit message from the client.
	tokenInit, err := messages.RecvRequest[*messages.TokenInit](stream)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &tokenInit.ClientParams)

	// verify the secret provided in TokenInit for token's that have a secret
	if tokenSecret, tokenHasSecret := token.GetSecret(); tokenHasSecret {
		if subtle.ConstantTimeCompare([]byte(tokenSecret), []byte(tokenInit.Secret)) != 1 {
			return nil, trace.BadParameter("invalid token secret")
		}
	}

	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&tokenInit.ClientParams,
		token,
		nil, /*rawClaims*/
		nil, /*attrs*/
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Scoped tokens have usage limits, so once we've verified that host certs could
	// be generated we need to attempt to consume the token. Any error should be
	// considered a join failure.
	if scopedToken, ok := joining.GetScopedToken(token); ok {
		publicKey := tokenInit.ClientParams.HostParams.PublicKeys.PublicTLSKey
		hostID := "unknown"
		hostResult, ok := result.(*messages.HostResult)
		if ok {
			hostID = hostResult.HostID
		}

		if _, err := s.cfg.ScopedTokenService.UseScopedToken(stream.Context(), scopedToken, publicKey); err != nil {
			s.emitScopedTokenEvent(stream.Context(), events.ScopedTokenFailEvent, scopedToken, hostID)
			return nil, trace.Wrap(err)
		}
		s.emitScopedTokenEvent(stream.Context(), events.ScopedTokenUseEvent, scopedToken, hostID)
	}

	return result, nil
}

// emitScopedTokenEvent emits the "use" or "fail" events depending on whether or not a scoped token successfully
// provisioned a resource.
func (s *Server) emitScopedTokenEvent(ctx context.Context, kind string, token *joiningv1.ScopedToken, hostID string) {
	var event apievents.AuditEvent
	switch kind {
	case events.ScopedTokenUseEvent:
		event = &apievents.ScopedTokenUse{
			Metadata: apievents.Metadata{
				Type: events.ScopedTokenUseEvent,
				Code: events.ScopedTokenUseCode,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name:    token.GetMetadata().GetName(),
				Expires: token.GetMetadata().GetExpires().AsTime(),
			},
			Roles:         token.GetSpec().GetRoles(),
			JoinMethod:    token.GetSpec().GetJoinMethod(),
			Scope:         token.GetScope(),
			AssignedScope: token.GetSpec().GetAssignedScope(),
			HostId:        hostID,
		}
	case events.ScopedTokenFailEvent:
		event = &apievents.ScopedTokenFail{
			Metadata: apievents.Metadata{
				Type: events.ScopedTokenFailEvent,
				Code: events.ScopedTokenFailCode,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				Name:    token.GetMetadata().GetName(),
				Expires: token.GetMetadata().GetExpires().AsTime(),
			},
			Roles:         token.GetSpec().GetRoles(),
			JoinMethod:    token.GetSpec().GetJoinMethod(),
			Scope:         token.GetScope(),
			AssignedScope: token.GetSpec().GetAssignedScope(),
			HostId:        hostID,
		}
	default:
		return
	}

	if err := s.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		s.cfg.Logger.WarnContext(ctx, "failed to emit scoped token event", "error", err, "type", kind)
	}
}
