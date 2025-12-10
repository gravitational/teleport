/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/azurejoin"
	"github.com/gravitational/teleport/lib/join/legacyjoin"
)

// RegisterUsingAzureMethod registers the caller using the Azure join method
// and returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *proto.RegisterUsingAzureMethodRequest with a signed attested data document
// including the challenge as a nonce.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func (a *Server) RegisterUsingAzureMethod(
	ctx context.Context,
	challengeResponse client.RegisterAzureChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, nil, joinRequest)
		}
	}()

	if legacyjoin.Disabled() {
		return nil, trace.Wrap(legacyjoin.ErrDisabled)
	}

	challenge, err := azurejoin.GenerateAzureChallenge()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinRequest = req.RegisterUsingTokenRequest

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodAzure {
		return nil, trace.AccessDenied("this token does not support the Azure join method")
	}

	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.Wrap(err, "Azure join method only supports ProvisionTokenV2, got %T", provisionToken)
	}

	joinAttrs, err := azurejoin.CheckAzureRequest(ctx, azurejoin.CheckAzureRequestParams{
		AzureJoinConfig: a.GetAzureJoinConfig(),
		Token:           ptv2,
		Challenge:       challenge,
		AttestedData:    req.AttestedData,
		AccessToken:     req.AccessToken,
		Logger:          a.logger,
		Clock:           a.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "checking Azure challenge response")
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		params := makeBotCertsParams(req.RegisterUsingTokenRequest, nil /*rawClaims*/, &workloadidentityv1pb.JoinAttrs{
			Azure: joinAttrs,
		})
		certs, _, err := a.GenerateBotCertsForJoin(ctx, provisionToken, params)
		return certs, trace.Wrap(err)
	}
	params := makeHostCertsParams(req.RegisterUsingTokenRequest, nil /*rawClaims*/)
	certs, err = a.GenerateHostCertsForJoin(ctx, provisionToken, params)
	return certs, trace.Wrap(err)
}

// GetAzureJoinConfig gets configuration options for azure joining.
func (a *Server) GetAzureJoinConfig() *azurejoin.AzureJoinConfig {
	return a.azureJoinConfig
}

// SetAzureJoinConfig sets configuration options for azure joining.
func (a *Server) SetAzureJoinConfig(c *azurejoin.AzureJoinConfig) {
	a.azureJoinConfig = c
}
