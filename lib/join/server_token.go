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
	"crypto/subtle"

	"github.com/gravitational/trace"

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
		if _, err := s.cfg.ScopedTokenService.UseScopedToken(stream.Context(), scopedToken, publicKey); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return result, nil
}
