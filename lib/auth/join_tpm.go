package auth

import (
	"context"
	"log/slog"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/tpmjoin"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

func (a *Server) RegisterUsingTPMMethod(
	ctx context.Context,
	initReq *proto.RegisterUsingTPMMethodInitialRequest,
	solveChallenge client.RegisterTPMChallengeResponseFunc,
) (*proto.Certs, error) {
	// First, check the specified token exists, and is a TPM-type join token.
	if err := initReq.JoinRequest.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	pt, err := a.checkTokenJoinRequestCommon(ctx, initReq.JoinRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ptv2 := pt.(*types.ProvisionTokenV2)
	if ptv2.Spec.JoinMethod != types.JoinMethodTPM {
		return nil, trace.AccessDenied("specified join token is not for `tpm` method")
	}

	validatedEK, err := tpmjoin.Validate(ctx, slog.Default(), tpmjoin.ValidateParams{
		EKCert:       initReq.GetEkCert(),
		EKKey:        initReq.GetEkKey(),
		AttestParams: tpmjoin.AttestationParametersFromProto(initReq.AttestationParams),
		AllowedCAs:   ptv2.Spec.TPM.EKCertAllowedCAs,
		Solve: func(ec *attest.EncryptedCredential) ([]byte, error) {
			solution, err := solveChallenge(dtoss.EncryptedCredentialToProto(ec))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return solution.Solution, nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Compare to rules!!

	return nil, trace.NotImplemented("RegisterUsingTPMMethod is not implemented")
}
