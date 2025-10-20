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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/iamjoin"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// handleIAMJoin handles join attempts for the IAM join method.
//
// The IAM join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server IAMInit
// client<-server IAMChallenge
// client->server IAMChallengeSolution
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the IAMInit message, handle the challenge-response, and send the
// final result if everything checks out.
func (s *Server) handleIAMJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken types.ProvisionToken,
) (messages.Response, error) {
	// Receive the IAMInit message from the client.
	iamInit, err := messages.RecvRequest[*messages.IAMInit](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving IAMInit message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &iamInit.ClientParams)

	// Generate and send the challenge.
	challenge, err := iamjoin.GenerateIAMChallenge()
	if err != nil {
		return nil, trace.Wrap(err, "generating challenge")
	}
	if err := stream.Send(&messages.IAMChallenge{
		Challenge: challenge,
	}); err != nil {
		return nil, trace.Wrap(err, "sending challenge")
	}

	// Receive the solution from the client.
	solution, err := messages.RecvRequest[*messages.IAMChallengeSolution](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving challenge solution")
	}

	// Verify the sts:GetCallerIdentity request, send it to AWS, and make sure
	// the verified identity matches allow rules in the provision token.
	verifiedIdentity, err := iamjoin.CheckIAMRequest(stream.Context(), &iamjoin.CheckIAMRequestParams{
		Challenge:          challenge,
		ProvisionToken:     provisionToken,
		STSIdentityRequest: solution.STSIdentityRequest,
		HTTPClient:         s.cfg.AuthService.GetHTTPClientForAWSSTS(),
		FIPS:               s.cfg.FIPS,
	})
	// An identity will be returned even on error if the sts:GetCallerIdentity
	// request was completed but no allow rules were matched, include it in the
	// diagnostic for debugging.
	stream.Diagnostic().Set(func(info *diagnostic.Info) {
		info.RawJoinAttrs = verifiedIdentity
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
		&iamInit.ClientParams,
		provisionToken,
		verifiedIdentity,
		&workloadidentityv1pb.JoinAttrs{
			Iam: verifiedIdentity.JoinAttrs(),
		},
	)
	return result, trace.Wrap(err)
}
