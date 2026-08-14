package devicetrustv1

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"log/slog"
	"math/big"
	"strings"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
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
	ctx := stream.Context()
	logger := c.logger.With(
		"device_id", dev.GetId(),
		"asset_tag", dev.GetAssetTag(),
		"credential_id", initReq.GetCredentialId(),
	)
	// Validate provided request includes the correct fields.
	switch {
	case !initReq.HasTpm():
		return nil, trace.BadParameter("tpm enrollment payload required")
	case !initReq.GetTpm().HasAttestationParameters():
		return nil, trace.BadParameter("attestation parameters required")
	case !initReq.GetTpm().HasEk():
		return nil, trace.BadParameter("ek_pub or ek_cert required")
	}

	validEK, err := parseAndValidateEK(
		logger,
		initReq.GetTpm(),
		c.ekCertAllowedCAs,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Validated EK submitted by client")

	// Next we initiate the two challenges:
	// - Credential Activation
	// - Platform Attestation
	attestationParameters := dtoss.AttestationParametersFromProto(
		initReq.GetTpm().GetAttestationParameters(),
	)
	encryptedCredential, finishCredentialActivation, err := credentialActivationChallenge(
		validEK.publicKey,
		attestationParameters,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestNonce, finishPlatformAttestation, err := platformAttestationChallenge(
		dev.GetOsType(),
		attestationParameters.Public,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Sending enrollment challenge")
	// Send the two challenges to the client and wait for a response
	// containing the Credential Activation solution and the Platform
	// Attestation
	if err := stream.Send(devicepb.EnrollDeviceResponse_builder{
		TpmChallenge: devicepb.TPMEnrollChallenge_builder{
			EncryptedCredential: dtoss.EncryptedCredentialToProto(
				encryptedCredential,
			),
			AttestationNonce: attestNonce,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.DebugContext(ctx, "Received enrollment challenge response")

	// Validate the challenge response
	chalResp := resp.GetTpmChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("bad payload, expected TPMEnrollChallengeResponse")
	case len(chalResp.GetSolution()) == 0:
		return nil, trace.BadParameter("credential activation solution required")
	}
	// Use the values sent by the client in the challenge response to finish
	// the credential activation and platform attestation challenges
	if err := finishCredentialActivation(chalResp.GetSolution()); err != nil {
		logger.DebugContext(ctx,
			"TPM credential activation failed verification",
			"error", err,
		)
		return nil, trace.BadParameter("credential activation verification failed")
	}
	platformAttestation, err := finishPlatformAttestation(
		dtoss.PlatformParametersFromProto(chalResp.GetPlatformParameters()),
	)
	if err != nil {
		logger.DebugContext(ctx,
			"TPM platform attestation failed verification",
			"error", err,
		)
		return nil, trace.BadParameter("platform attestation verification failed")
	}
	// Persist platform attestation record in collected data.
	initReq.GetDeviceData().SetTpmPlatformAttestation(platformAttestation)

	// Create credential storage type
	cred := devicepb.DeviceCredential_builder{
		Id:                    initReq.GetCredentialId(),
		DeviceAttestationType: validEK.attestationType,
		TpmEkcertSerial:       validEK.tpmSerial,
		TpmAkPublic:           attestationParameters.Public,
	}.Build()

	return cred, nil
}

type validatedEK struct {
	// publicKey is a special "any". It will usually hold a *rsa.PublicKey but
	// could hold other types of public keys.
	publicKey       crypto.PublicKey
	attestationType devicepb.DeviceAttestationType
	tpmSerial       string
}

var sanExtensionOID = []int{2, 5, 29, 17}

// parseAndValidateEK extracts the EK public key from the enrollment request.
// This will either be a directly a public key, or a public key included
// within a certificate signed by a device manufacturer CA.
// It ensures the certificate is signed by a CA on the allow-list if the list
// is non-empty.
func parseAndValidateEK(
	logger *slog.Logger,
	tpm *devicepb.TPMEnrollPayload,
	allowedCAs []string,
) (
	*validatedEK,
	error,
) {
	switch v := tpm.WhichEk(); v {
	case devicepb.TPMEnrollPayload_EkKey_case:
		if len(allowedCAs) > 0 {
			return nil, trace.BadParameter("tpm device did not submit an ek_cert and ekcert_allowed_cas is configured")
		}

		// In the case of the key, we can just use this as is.
		ekPub, err := x509.ParsePKIXPublicKey(tpm.GetEkKey())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &validatedEK{
			attestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			publicKey:       ekPub,
		}, nil
	case devicepb.TPMEnrollPayload_EkCert_case:
		// In the case of a certificate, we need to decode the cert and then
		// extract the public key, optionally, we also need to verify the
		// certificates CA.
		ekCert, err := attest.ParseEKCertificate(tpm.GetEkCert())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tpmSerial := serialString(ekCert.SerialNumber)

		// Second, we want to check if the certificate is signed by an
		// allow-listed CA. If there's no configured allow-listed CAs, we can
		// skip this check.
		if len(allowedCAs) == 0 {
			return &validatedEK{
				publicKey:       ekCert.PublicKey,
				attestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT,
				tpmSerial:       tpmSerial,
			}, nil
		}

		allowedPool, err := x509PEMsToCertPool(allowedCAs)
		if err != nil {
			// Hypothetically, this case is never triggered. We validate these
			// entries in CheckAndSetDefaults.
			return nil, trace.Wrap(err, "invalid device trust EKCertAllowedCAs entry")
		}

		// EKCerts often include some additional data bundled within the SAN
		// extension. This ext is also sometimes marked critical. This causes
		// the Verify() to reject the cert because not all data within a
		// critical extension has been handled. We mark this as OK here by
		// stripping the SAN Extension OID out of UnhandledCriticalExtensions.
		var exts []asn1.ObjectIdentifier
		for _, ext := range ekCert.UnhandledCriticalExtensions {
			if ext.Equal(sanExtensionOID) {
				logger.DebugContext(context.Background(),
					"Ignoring unhandled critical extension in EKCert",
					"oid", ext,
				)
				continue
			}
			exts = append(exts, ext)
		}
		ekCert.UnhandledCriticalExtensions = exts

		_, err = ekCert.Verify(x509.VerifyOptions{
			Roots: allowedPool,
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
			publicKey:       ekCert.PublicKey,
			attestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED,
			tpmSerial:       tpmSerial,
		}, nil
	default:
		return nil, trace.BadParameter("unknown EK type (%v)", v)
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

func x509PEMsToCertPool(certPEMs []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, cert := range certPEMs {
		if !pool.AppendCertsFromPEM([]byte(cert)) {
			return nil, trace.BadParameter("failed to parse certificate PEM")
		}
	}
	return pool, nil
}
