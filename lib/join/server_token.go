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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// handleTokenJoin handles join attempts for the token join method.
func (s *Server) handleTokenJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken types.ProvisionToken,
) (messages.Response, error) {
	// Receive the TokenInit message from the client.
	tokenInit, err := messages.RecvRequest[*messages.TokenInit](stream)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &tokenInit.ClientParams)

	// There are no additional checks for the token join method, just make the
	// result message and return it.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&tokenInit.ClientParams,
		nil, /*rawClaims*/
		provisionToken,
	)
	return result, trace.Wrap(err)
}
