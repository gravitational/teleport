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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/boundkeypair"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// handleBoundKeypairJoin takes over the join process after the ClientInit
// message has been received for the bound keypair join method.
func (s *Server) handleBoundKeypairJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken types.ProvisionToken,
) (*messages.BotResult, error) {
	ctx := stream.Context()
	diag := stream.Diagnostic()
	// Only bot joining is supported at the moment - unique ID verification is
	// required and this is currently only implemented for bots.
	if clientInit.SystemRole != types.RoleBot.String() {
		return nil, trace.BadParameter("bound keypair joining is only supported for bots")
	}
	if err := stream.Send(&messages.ServerInit{
		JoinMethod: string(types.JoinMethodBoundKeypair),
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	boundKeypairInit, err := messages.RecvRequest[*messages.BoundKeypairInit](stream)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	issueChallenge := func(challenge *messages.BoundKeypairChallenge) (*messages.BoundKeypairChallengeSolution, error) {
		if err := stream.Send(challenge); err != nil {
			return nil, trace.Wrap(err)
		}
		solution, err := messages.RecvRequest[*messages.BoundKeypairChallengeSolution](stream)
		return solution, trace.Wrap(err)
	}
	issueRotationRequest := func(rotationReq *messages.BoundKeypairRotationRequest) (*messages.BoundKeypairRotationResponse, error) {
		if err := stream.Send(rotationReq); err != nil {
			return nil, trace.Wrap(err)
		}
		rotationResp, err := messages.RecvRequest[*messages.BoundKeypairRotationResponse](stream)
		return rotationResp, trace.Wrap(err)
	}
	generateBotCerts := func(ctx context.Context, previousBotInstanceID string, claims any) (*messages.Certificates, string, error) {
		botCertsParams, err := makeBotCertsParams(diag, authCtx, boundKeypairInit.ClientParams.BotParams, claims)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		botCertsParams.PreviousBotInstanceID = previousBotInstanceID
		protoCerts, botInstanceID, err := s.cfg.AuthService.GenerateBotCertsForJoin(ctx, provisionToken, botCertsParams)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		botCerts, err := convertCerts(protoCerts)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return botCerts, botInstanceID, nil
	}
	return boundkeypair.HandleBoundKeypairJoin(ctx, &boundkeypair.JoinParams{
		AuthService:          s.cfg.AuthService,
		AuthCtx:              authCtx,
		Diag:                 diag,
		ProvisionToken:       provisionToken,
		ClientInit:           clientInit,
		BoundKeypairInit:     boundKeypairInit,
		IssueChallenge:       issueChallenge,
		IssueRotationRequest: issueRotationRequest,
		GenerateBotCerts:     generateBotCerts,
		Clock:                s.clock,
		Logger:               log,
	})
}

// AdaptRegisterUsingBoundKeypairMethod handles requests from the legacy join
// gRPC service and adapts the request types to the protocol-agnostic types
// defined in [messages] before calling [boundkeypair.HandleBoundKeypairJoin]
// which contains the actual logic for bound keypair joining.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func AdaptRegisterUsingBoundKeypairMethod(
	ctx context.Context,
	a AuthService,
	createBoundKeypairValidator boundkeypair.CreateBoundKeypairValidator,
	req *proto.RegisterUsingBoundKeypairInitialRequest,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) (_ *client.BoundKeypairRegistrationResponse, err error) {
	diag := diagnostic.New()
	diag.Set(func(i *diagnostic.Info) {
		i.RemoteAddr = req.JoinRequest.RemoteAddr
		i.Role = req.JoinRequest.Role.String()
		i.RequestedJoinMethod = string(types.JoinMethodBoundKeypair)
		i.BotInstanceID = req.JoinRequest.BotInstanceID
		i.BotGeneration = uint64(req.JoinRequest.BotGeneration)
	})
	defer func() {
		if err != nil {
			diag.Set(func(i *diagnostic.Info) { i.Error = err })
			handleJoinFailure(ctx, a, diag)
		}
	}()

	// Construct an [authz.Context] to pass to HandleBoundKeypairJoin.
	authCtx := &authz.Context{
		// These are verified at the gRPC layer by the legacy join service.
		BotInstanceID: req.JoinRequest.BotInstanceID,
		BotGeneration: uint64(req.JoinRequest.BotGeneration),
	}

	// Only bot joining is supported at the moment - unique ID verification is
	// required and this is currently only implemented for bots.
	if req.JoinRequest.Role != types.RoleBot {
		return nil, trace.BadParameter("bound keypair joining is only supported for bots")
	}

	provisionToken, err := a.ValidateToken(ctx, req.JoinRequest.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set any diagnostic info we can get from the token.
	diag.Set(func(i *diagnostic.Info) {
		i.SafeTokenName = provisionToken.GetSafeName()
		i.TokenJoinMethod = string(provisionToken.GetJoinMethod())
		i.TokenExpires = provisionToken.Expiry()
		i.BotName = provisionToken.GetBotName()
	})
	if provisionToken.GetJoinMethod() != types.JoinMethodBoundKeypair {
		return nil, trace.BadParameter("specified join token is not for `%s` method", types.JoinMethodBoundKeypair)
	}

	// Assert that the provision token allows the requested system role.
	if err := ProvisionTokenAllowsRole(provisionToken, req.JoinRequest.Role); err != nil {
		return nil, trace.Wrap(err)
	}

	clientInit := clientInitFromRegisterUsingTokenRequest(req.JoinRequest, string(types.JoinMethodBoundKeypair))
	clientParams, err := clientParamsFromRegisterUsingTokenRequest(req.JoinRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	boundKeypairInit := &messages.BoundKeypairInit{
		ClientParams:      *clientParams,
		InitialJoinSecret: req.InitialJoinSecret,
		PreviousJoinState: req.PreviousJoinState,
	}

	generateBotCerts := func(ctx context.Context, previousBotInstanceID string, claims any) (*messages.Certificates, string, error) {
		botCertsParams, err := makeBotCertsParams(diag, authCtx, clientParams.BotParams, claims)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		botCertsParams.PreviousBotInstanceID = previousBotInstanceID
		protoCerts, botInstanceID, err := a.GenerateBotCertsForJoin(ctx, provisionToken, botCertsParams)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		botCerts, err := convertCerts(protoCerts)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return botCerts, botInstanceID, nil
	}

	botResult, err := boundkeypair.HandleBoundKeypairJoin(ctx, &boundkeypair.JoinParams{
		AuthService:                 a,
		AuthCtx:                     authCtx,
		Diag:                        diag,
		ProvisionToken:              provisionToken,
		ClientInit:                  clientInit,
		BoundKeypairInit:            boundKeypairInit,
		IssueChallenge:              adaptBoundKeypairChallenge(challengeResponse),
		IssueRotationRequest:        adaptBoundKeypairRotationRequest(challengeResponse),
		CreateBoundKeypairValidator: createBoundKeypairValidator,
		GenerateBotCerts:            generateBotCerts,
		Clock:                       a.GetClock(),
		Logger:                      log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := protoCertsFromCertificates(botResult.Certificates)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &client.BoundKeypairRegistrationResponse{
		Certs:          certs,
		BoundPublicKey: string(botResult.BoundKeypairResult.PublicKey),
		JoinState:      botResult.BoundKeypairResult.JoinState,
	}, nil
}

func adaptBoundKeypairChallenge(challengeResponseFunc client.RegisterUsingBoundKeypairChallengeResponseFunc) boundkeypair.ChallengeResponseFunc {
	return func(challengeMsg *messages.BoundKeypairChallenge) (*messages.BoundKeypairChallengeSolution, error) {
		challenge := &proto.RegisterUsingBoundKeypairMethodResponse{
			Response: &proto.RegisterUsingBoundKeypairMethodResponse_Challenge{
				Challenge: &proto.RegisterUsingBoundKeypairChallenge{
					PublicKey: string(challengeMsg.PublicKey),
					Challenge: challengeMsg.Challenge,
				},
			},
		}
		resp, err := challengeResponseFunc(challenge)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challengeResp := resp.GetChallengeResponse()
		if challengeResp == nil {
			return nil, trace.BadParameter("client did not send challenge response, got %T", resp.Payload)
		}
		return &messages.BoundKeypairChallengeSolution{
			Solution: challengeResp.Solution,
		}, nil
	}
}

func adaptBoundKeypairRotationRequest(challengeResponseFunc client.RegisterUsingBoundKeypairChallengeResponseFunc) boundkeypair.RotationFunc {
	return func(rotationMsg *messages.BoundKeypairRotationRequest) (*messages.BoundKeypairRotationResponse, error) {
		suite, err := types.SignatureAlgorithmSuiteFromString(rotationMsg.SignatureAlgorithmSuite)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challenge := &proto.RegisterUsingBoundKeypairMethodResponse{
			Response: &proto.RegisterUsingBoundKeypairMethodResponse_Rotation{
				Rotation: &proto.RegisterUsingBoundKeypairRotationRequest{
					SignatureAlgorithmSuite: suite,
				},
			},
		}
		resp, err := challengeResponseFunc(challenge)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rotationResp := resp.GetRotationResponse()
		if rotationResp == nil {
			return nil, trace.BadParameter("client did not send rotation response, got %T", resp.Payload)
		}
		return &messages.BoundKeypairRotationResponse{
			PublicKey: []byte(rotationResp.PublicKey),
		}, nil
	}
}
