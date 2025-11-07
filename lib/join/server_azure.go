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
	"github.com/gravitational/teleport/lib/join/azurejoin"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
)

// handleAzureJoin handles join attempts for the Azure join method.
//
// The Azure join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server AzureInit
// client<-server AzureChallenge
// client->server AzureChallengeSolution
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the AzureInit message, handle the challenge-response, and send the
// final result if everything checks out.
func (s *Server) handleAzureJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	token provision.Token,
) (messages.Response, error) {
	// Receive the AzureInit message from the client.
	azureInit, err := messages.RecvRequest[*messages.AzureInit](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving AzureInit message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &azureInit.ClientParams)

	// Generate and send the challenge.
	challenge, err := azurejoin.GenerateAzureChallenge()
	if err != nil {
		return nil, trace.Wrap(err, "generating challenge")
	}
	if err := stream.Send(&messages.AzureChallenge{
		Challenge: challenge,
	}); err != nil {
		return nil, trace.Wrap(err, "sending AzureChallenge")
	}

	// Receive the solution from the client.
	solution, err := messages.RecvRequest[*messages.AzureChallengeSolution](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving AzureChallengeSolution")
	}

	ptv2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("Azure join method only supports ProvisionTokenV2, got %T", token)
	}

	// Verify the client's idenitty and make sure it matches an allow rule in the provision token.
	claims, err := azurejoin.CheckAzureRequest(stream.Context(), azurejoin.CheckAzureRequestParams{
		AzureJoinConfig: s.cfg.AuthService.GetAzureJoinConfig(),
		Token:           ptv2,
		Challenge:       challenge,
		AttestedData:    solution.AttestedData,
		AccessToken:     solution.AccessToken,
		Logger:          log,
		Clock:           s.cfg.AuthService.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "checking Azure challenge solution")
	}

	// Make and return the final result message.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&azureInit.ClientParams,
		token,
		nil, // rawClaims
		&workloadidentityv1pb.JoinAttrs{
			Azure: claims,
		},
	)
	return result, trace.Wrap(err)
}
