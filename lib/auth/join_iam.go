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
	"github.com/gravitational/teleport/lib/join/iamjoin"
	"github.com/gravitational/teleport/lib/utils"
)

// SetHTTPClientForAWSSTS sets an HTTP client that will be used for sending
// client-signed sts:GetCallerIdentity requests to AWS, for tests.
func (a *Server) SetHTTPClientForAWSSTS(clt utils.HTTPDoClient) {
	a.httpClientForAWSSTS = clt
}

// GetHTTPClientForAWSSTS returns an HTTP client that should be used for sending
// client-signed sts:GetCallerIdentity requests to AWS.
func (a *Server) GetHTTPClientForAWSSTS() utils.HTTPDoClient {
	return a.httpClientForAWSSTS
}

// RegisterUsingIAMMethod registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *types.RegisterUsingTokenRequest with a signed sts:GetCallerIdentity request
// including the challenge as a signed header.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func (a *Server) RegisterUsingIAMMethod(
	ctx context.Context,
	challengeResponse client.RegisterIAMChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	var joinFailureMetadata any
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, joinFailureMetadata, joinRequest,
			)
		}
	}()

	challenge, err := iamjoin.GenerateIAMChallenge()
	if err != nil {
		return nil, trace.Wrap(err, "generating IAM challenge")
	}

	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err, "getting challenge response")
	}
	joinRequest = req.RegisterUsingTokenRequest

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating request parameters")
	}

	// perform common token checks
	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err, "completing common token checks")
	}

	// check that the GetCallerIdentity request is valid and matches the token
	verifiedIdentity, err := iamjoin.CheckIAMRequest(ctx, &iamjoin.CheckIAMRequestParams{
		Challenge:          challenge,
		ProvisionToken:     provisionToken,
		STSIdentityRequest: req.StsIdentityRequest,
		HTTPClient:         a.GetHTTPClientForAWSSTS(),
		FIPS:               a.fips,
	})
	if verifiedIdentity != nil {
		joinFailureMetadata = verifiedIdentity
	}
	if err != nil {
		return nil, trace.Wrap(err, "checking iam request")
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		params := makeBotCertsParams(req.RegisterUsingTokenRequest, verifiedIdentity, &workloadidentityv1pb.JoinAttrs{
			Iam: verifiedIdentity.JoinAttrs(),
		})
		certs, _, err := a.GenerateBotCertsForJoin(ctx, provisionToken, params)
		return certs, trace.Wrap(err, "generating bot certs")
	}
	params := makeHostCertsParams(req.RegisterUsingTokenRequest, verifiedIdentity)
	certs, err = a.GenerateHostCertsForJoin(ctx, provisionToken, params)
	return certs, trace.Wrap(err, "generating certs")
}
