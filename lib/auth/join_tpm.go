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
) (certs *proto.Certs, err error) {
	var validated *tpmjoin.ValidatedTPM
	defer func() {
		if err != nil {
			log.WithError(err).Error("An attempted join using the TPM method failed")
		}
	}()

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

	if err := checkTPMAllowRules(validatedEK, ptv2.Spec.TPM.Allow); err != nil {
		return nil, trace.Wrap(err)
	}

	if initReq.JoinRequest.Role == types.RoleBot {
		certs, err = a.generateCertsBot(
			ctx, ptv2, initReq.JoinRequest, validatedEK,
		)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(
		ctx, ptv2, initReq.JoinRequest, validatedEK,
	)
	return certs, trace.Wrap(err)
}

func checkTPMAllowRules(tpm *tpmjoin.ValidatedTPM, rules []*types.ProvisionTokenSpecV2TPM_Rule) error {
	for _, rule := range rules {

	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
