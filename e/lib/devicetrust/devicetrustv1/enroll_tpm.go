package devicetrustv1

import (
	"crypto"
	"crypto/x509"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

// enrollDeviceTPM completes the enrollment procedure for a TPM.
//
// It works on a chain of trust from an attestation key to an endorsement key,
// where we then optionally ensure that this endorsement key's public part
// is contained within a certificate issued by a device manufacturer's
// certificate authority that the user trusts.
//
// Where this certificate check is omitted (e.g the user has not configured
// trusted device manufacturer CAs) many of the assurances provided by TPM2.0
// are not trust-worthy as we do not have proof that we are communicating with
// a legitimate TPM2.0 that has not been compromised and that follows the rules
// set out in the TPM2.0 specification. In this case, we rely on the user
// manually validating that they are using a legitimate TPM before enrolling.
func (c *enrollCeremony) enrollDeviceTPM(
	initReq *devicepb.EnrollDeviceInit,
	dev *devicepb.Device,
	stream devicepb.DeviceTrustService_EnrollDeviceServer,
) (*devicepb.DeviceCredential, error) {
	if !devicetrust.TPMEnrollmentActive {
		return nil, trace.BadParameter("tpm enrollment is currently disabled")
	}
	logger := c.logger.WithFields(log.Fields{
		"device_id":     dev.Id,
		"asset_tag":     dev.AssetTag,
		"credential_id": initReq.CredentialId,
	})
	// Validate provided request includes the correct fields.
	switch {
	case initReq.Tpm == nil:
		return nil, trace.BadParameter("tpm enrollment payload required")
	case initReq.Tpm.AttestationParameters == nil:
		return nil, trace.BadParameter("attestation parameters required")
	case initReq.Tpm.Ek == nil:
		return nil, trace.BadParameter("ek_pub or ek_cert required")
	}

	// First we extract the EK from the request
	validEK, err := parseAndValidateEK(
		initReq.Tpm,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.Debug("Validated EK submitted by client")

	// Next we initiate the two challenges:
	// - Credential Activation
	// - Platform Attestation
	attestationParameters := dtoss.AttestationParametersFromProto(
		initReq.Tpm.AttestationParameters,
	)
	encryptedCredential, finishCredentialActivation, err := credentialActivationChallenge(
		validEK.publicKey,
		attestationParameters,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestNonce, finishPlatformAttestation, err := platformAttestationChallenge(
		attestationParameters.Public,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.Debug("Sending enrollment challenge")
	// Send the two challenges to the client and wait for a response
	// containing the Credential Activation solution and the Platform
	// Attestation
	if err := stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_TpmChallenge{
			TpmChallenge: &devicepb.TPMEnrollChallenge{
				EncryptedCredential: dtoss.EncryptedCredentialToProto(
					encryptedCredential,
				),
				AttestationNonce: attestNonce,
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.Debug("Received enrollment challenge response")

	// Validate the challenge response
	chalResp := resp.GetTpmChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("bad payload, expected TPMEnrollChallengeResponse")
	case chalResp.PlatformParameters == nil:
		return nil, trace.BadParameter("platform parameters required")
	case len(chalResp.Solution) == 0:
		return nil, trace.BadParameter("credential activation solution required")
	}
	// Use the values sent by the client in the challenge response to finish
	// the credential activation and platform attestation challenges
	if err := finishCredentialActivation(chalResp.Solution); err != nil {
		logger.WithError(err).Debug("TPM credential activation provided in enrollment challenge response failed verification")
		return nil, trace.BadParameter("credential activation verification failed")
	}
	if err := finishPlatformAttestation(
		dtoss.PlatformParametersFromProto(chalResp.PlatformParameters),
	); err != nil {
		logger.WithError(err).Debug("TPM platform attestation provided in enrollment challenge response failed verification")
		return nil, trace.BadParameter("platform attestation verification failed")
	}

	// Create credential storage type
	cred := &devicepb.DeviceCredential{
		Id:                    initReq.CredentialId,
		DeviceAttestationType: validEK.attestationType,
		TpmAkPublic:           attestationParameters.Public,
	}

	return cred, nil
}

type validatedEK struct {
	// publicKey is a special "any". It will usually hold a *rsa.PublicKey but
	// could hold other types of public keys.
	publicKey       crypto.PublicKey
	attestationType devicepb.DeviceAttestationType
}

// parseAndValidateEK extracts the EK public key from the enrollment request.
// This will either be a directly a public key, or a public key included
// within a certificate signed by a device manufacturer CA.
// It ensures the certificate is signed by a CA on the allow-list if the list
// is non-empty.
// TODO(strideynet): Support parsing EKCert and ensuring its signed by a device
// manufacturer CA: https://github.com/gravitational/teleport.e/issues/1393
func parseAndValidateEK(
	tpm *devicepb.TPMEnrollPayload,
) (
	*validatedEK,
	error,
) {
	switch v := tpm.Ek.(type) {
	case *devicepb.TPMEnrollPayload_EkKey:
		// In the case of the key, we can just use this as is.
		ekPub, err := x509.ParsePKIXPublicKey(v.EkKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &validatedEK{
			attestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			publicKey:       ekPub,
		}, nil
	default:
		return nil, trace.BadParameter("unknown EK type (%T)", v)
	}
}
