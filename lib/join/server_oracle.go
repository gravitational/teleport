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
	"encoding/base64"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinutils"
	"github.com/gravitational/teleport/lib/join/oraclejoin"
	"github.com/gravitational/teleport/lib/join/provision"
)

// handleOracleJoin handles join attempts for the Oracle join method.
//
// The Oracle join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server OracleInit
// client<-server OracleChallenge
// client->server OracleChallengeSolution
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the OracleInit message, handle the challenge-response, and send the
// final result if everything checks out.
func (s *Server) handleOracleJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken provision.Token,
) (messages.Response, error) {
	// Receive the OracleInit message from the client.
	oracleInit, err := messages.RecvRequest[*messages.OracleInit](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving OracleInit message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &oracleInit.ClientParams)

	// Generate and send the challenge.
	challenge, err := joinutils.GenerateChallenge(base64.RawStdEncoding, 32)
	if err != nil {
		return nil, trace.Wrap(err, "generating challenge")
	}
	if err := stream.Send(&messages.OracleChallenge{
		Challenge: challenge,
	}); err != nil {
		return nil, trace.Wrap(err, "sending challenge")
	}

	// Receive the solution from the client.
	solution, err := messages.RecvRequest[*messages.OracleChallengeSolution](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving challenge solution")
	}

	claims, err := oraclejoin.CheckChallengeSolution(stream.Context(), &oraclejoin.CheckChallengeSolutionParams{
		Challenge:      challenge,
		Solution:       solution,
		ProvisionToken: provisionToken,
		HTTPClient:     s.cfg.OracleHTTPClient,
		RootCACache:    s.oracleRootCACache,
	})
	// CheckOracleRequest may return claims even when returning an error, which
	// may aid in debugging.
	stream.Diagnostic().Set(func(info *diagnostic.Info) {
		info.RawJoinAttrs = claims
	})
	if err != nil {
		return nil, trace.Wrap(err, "verifying challenge response")
	}

	// Make and return the final result message.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&oracleInit.ClientParams,
		provisionToken,
		claims,
		&workloadidentityv1pb.JoinAttrs{
			Oracle: claims.JoinAttrs(),
		},
	)
	return result, trace.Wrap(err)
}
