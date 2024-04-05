package auth

import (
	"context"
	"crypto"
	"crypto/subtle"
	"crypto/x509"
	"math/big"
	"strings"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tpmjoin"
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

	// Now, we can perform the initial validation of the token. We check the
	// EKCert/EKKey against the tokens configured rules.
	ek, err := validateEK(initReq, ptv2.Spec.TPM.EKCertAllowedCAs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Compare to rules!!

	// Knowing that the presented EK matches the rules, we can now perform the
	// credential activation. We generate a challenge, submit this to the
	// client, and then validate their solution.
	challenge, checkSolution, err := credentialActivationChallenge(
		ek.publicKey,
		tpmjoin.AttestationParametersFromProto(initReq.AttestationParams),
	)
	if err != nil {
		return nil, trace.Wrap(err)

	}
	challengeResp, err := solveChallenge(tpmjoin.EncryptedCredentialToProto(challenge))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkSolution(challengeResp.Solution); err != nil {
		return nil, trace.Wrap(err, "solution did not match")
	}

	return nil, trace.NotImplemented("RegisterUsingTPMMethod is not implemented")
}

func credentialActivationChallenge(
	ek crypto.PublicKey,
	attestationParameters attest.AttestationParameters,
) (
	*attest.EncryptedCredential,
	func(clientSolution []byte) error,
	error,
) {
	activationParameters := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		AK:         attestationParameters,
		EK:         ek,
	}
	// The generate method completes initial validation that provides the
	// following assurances:
	// - The attestation key is of a secure length
	// - The attestation key is marked as created within a TPM
	// - The attestation key is marked as restricted (e.g cannot be used to
	//   sign or decrypt external data)
	// When the returned challenge is solved by the TPM using ActivateCredential
	// the following additional assurance is given:
	// - The attestation key resides in the same TPM as the endorsement key
	solution, encryptedCredential, err := activationParameters.Generate()
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating credential activation challenge")
	}
	return encryptedCredential, func(clientSolution []byte) error {
		if subtle.ConstantTimeCompare(clientSolution, solution) == 0 {
			return trace.BadParameter("invalid credential activation solution")
		}
		return nil
	}, nil
}

type validatedEK struct {
	publicKey crypto.PublicKey
	serial    string
}

func validateEK(
	initReq *proto.RegisterUsingTPMMethodInitialRequest,
	allowedCAs [][]byte,
) (*validatedEK, error) {
	switch v := initReq.Ek.(type) {
	case *proto.RegisterUsingTPMMethodInitialRequest_EkCert:
		// Validate that the EKCert is signed by one of the CAs
		ekCert, err := attest.ParseEKCertificate(v.EkCert)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(allowedCAs) == 0 {
			return &validatedEK{
				publicKey: ekCert.PublicKey,
				serial:    "SERIAL TODO",
			}, nil
		}

		// Collect CAs into a pool to use for validation
		caPool := x509.NewCertPool()
		for _, caPEM := range allowedCAs {
			if !caPool.AppendCertsFromPEM(caPEM) {
				return nil, trace.BadParameter("invalid CA PEM")
			}
		}

		// Validate EKCert against CA pool
		_, err = ekCert.Verify(x509.VerifyOptions{
			Roots: caPool,
			KeyUsages: []x509.ExtKeyUsage{
				// Go's x509 Verification doesn't support the EK certificate
				// ExtKeyUsage (http://oid-info.com/get/2.23.133.8.1), so we
				// allow any.
				x509.ExtKeyUsageAny,
			},
		})
		if err != nil {
			return nil, trace.BadParameter("presented EKCert failed verification: %v", err)
		}

		return &validatedEK{
			publicKey: ekCert.PublicKey,
			serial:    serialString(ekCert.SerialNumber),
		}, nil
	case *proto.RegisterUsingTPMMethodInitialRequest_EkKey:
		if len(allowedCAs) > 0 {
			// If a CA allow-list is configured, then we need to reject joins
			// where there is no EKCert.
			return nil, trace.AccessDenied("tpm device did not submit an ek_cert but ekcert_allowed_cas is configured")
		}
		pubKey, err := x509.ParsePKIXPublicKey(v.EkKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &validatedEK{
			publicKey: pubKey,
		}, nil
	default:
		return nil, trace.BadParameter("invalid EK type %T", v)
	}
}

// serialString converts a serial number into a readable colon-delimited hex
// string thats user-readable e.g ab:ab:ab:ff:ff:ff
func serialString(serial *big.Int) string {
	hex := serial.Text(16)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}

	out := strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}
