/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/legacyjoin"
	"github.com/gravitational/teleport/lib/join/tpmjoin"
	"github.com/gravitational/teleport/lib/tpm"
)

func (a *Server) RegisterUsingTPMMethod(
	ctx context.Context,
	initReq *proto.RegisterUsingTPMMethodInitialRequest,
	solveChallenge client.RegisterTPMChallengeResponseFunc,
) (_ *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinFailureMetadata any
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, joinFailureMetadata, initReq.JoinRequest,
			)
		}
	}()

	if legacyjoin.Disabled() {
		return nil, trace.Wrap(legacyjoin.ErrDisabled)
	}

	// First, check the specified token exists, and is a TPM-type join token.
	if err := initReq.JoinRequest.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, initReq.JoinRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
	}
	if ptv2.Spec.JoinMethod != types.JoinMethodTPM {
		return nil, trace.BadParameter("specified join token is not for `tpm` method")
	}

	solve := func(ec *attest.EncryptedCredential) ([]byte, error) {
		solution, err := solveChallenge(tpm.EncryptedCredentialToProto(ec))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return solution.Solution, nil
	}

	validatedEK, err := tpmjoin.CheckTPMRequest(ctx, tpmjoin.CheckTPMRequestParams{
		Token:        ptv2,
		TPMValidator: a.GetTPMValidator(),
		EKCert:       initReq.GetEkCert(),
		EKKey:        initReq.GetEkKey(),
		AttestParams: tpm.AttestationParametersFromProto(initReq.AttestationParams),
		Solve:        solve,
	})
	if validatedEK != nil {
		joinFailureMetadata = validatedEK
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if initReq.JoinRequest.Role == types.RoleBot {
		params := makeBotCertsParams(initReq.JoinRequest, validatedEK, &workloadidentityv1pb.JoinAttrs{
			Tpm: validatedEK.JoinAttrs(),
		})
		certs, _, err := a.GenerateBotCertsForJoin(ctx, ptv2, params)
		return certs, trace.Wrap(err, "generating certs for bot")
	}
	params := makeHostCertsParams(initReq.JoinRequest, validatedEK)
	certs, err := a.GenerateHostCertsForJoin(ctx, ptv2, params)
	return certs, trace.Wrap(err, "generating certs for host")
}

// GetTPMValidator returns the server's TPM validator.
func (a *Server) GetTPMValidator() tpmjoin.TPMValidator {
	return a.tpmValidator
}

// SetTPMValidator sets the server's TPM validator.
func (a *Server) SetTPMValidator(v tpmjoin.TPMValidator) {
	a.tpmValidator = v
}
