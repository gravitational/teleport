package devicetrustv1

import (
	"context"
	"crypto"
	_ "crypto/sha256" // imported for crypto.SHA256
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	dterrors "github.com/gravitational/teleport/e/lib/devicetrust/errors"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

const (
	deviceAuthnFailedMessage     = "device authentication failed"
	invalidInitMessage           = "invalid initial payload"
	invalidDeviceWebTokenMessage = "invalid device web token"
)

// deviceAuthnAuditData holds additional audit data used by
// AuthnCeremony.AuditCallback.
type deviceAuthnAuditData struct {
	HasDeviceWebToken   bool
	WebAuthenticationID string
}

// authenticateDeviceStream abstracts
// devicepb.DeviceTrustService_AuthenticateDeviceServer.
type authenticateDeviceStream interface {
	Send(*devicepb.AuthenticateDeviceResponse) error
	Recv() (*devicepb.AuthenticateDeviceRequest, error)
}

// authnCeremony is the device authentication ceremony.
type authnCeremony struct {
	logger  *slog.Logger
	storage *storage.S
	// skipOwnerBackfill skips backfilling device owners if set to true.
	skipOwnerBackfill bool
	// cachedUsers is only required for owner backfill.
	cachedUsers UsersService
	// augmentCertsFunc calls its namesake auth.Server function.
	// May be nil for ceremonies that don't issue new certificates (like device
	// assertion ceremonies)
	augmentCertsFunc func(ctx context.Context, opts *auth.AugmentUserCertificateOpts) (*clientproto.Certs, error)
	auditCallback    func(dev *devicepb.Device, auditData *deviceAuthnAuditData, err error)
}

// AuthenticateDevice implements the trusted device authentication ceremony, as
// described by devicepb.DeviceTrustService.AuthenticateDevice.
//
// As long as any device information is acquired from the stream, a non-nil
// device is returned, even if the ceremony itself failed.
//
// The ceremony auditCallback is guaranteed to be called exactly once, either
// after the first error or before the last Send of the stream.
// The outcome of the last Send is not considered for audit purposes.
func (c *authnCeremony) AuthenticateDevice(
	ctx context.Context,
	stream authenticateDeviceStream,
	user string,
) (*devicepb.Device, error) {
	var auditData deviceAuthnAuditData
	dev, successResp, err := c.authenticateDevice(ctx, stream, user, &auditData)
	c.auditCallback(dev, &auditData, err)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	c.backfillDeviceOwner(ctx, dev, user)

	// Success (only send after audit).
	return dev, trace.Wrap(stream.Send(successResp))
}

func (c *authnCeremony) authenticateDevice(
	ctx context.Context,
	stream authenticateDeviceStream,
	user string,
	auditData *deviceAuthnAuditData,
) (*devicepb.Device, *devicepb.AuthenticateDeviceResponse, error) {
	// 1. Init.
	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Do some preliminary checks so we can fetch the device...
	initReq := resp.GetInit()
	auditData.HasDeviceWebToken = initReq.GetDeviceWebToken() != nil
	auditData.WebAuthenticationID = initReq.GetDeviceWebToken().GetId()
	switch {
	case initReq == nil:
		err = trace.BadParameter("bad payload, expected AuthenticateDeviceInit")
	case !initReq.HasDeviceData():
		err = trace.BadParameter("device data required")
	case initReq.GetDeviceData().GetOsType() == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		err = trace.BadParameter("device OS type required")
	case initReq.GetDeviceData().GetSerialNumber() == "":
		err = trace.BadParameter("device serial number required")
	}
	if err != nil {
		return nil, nil, auditStatusError{
			Err:         err,
			UserMessage: invalidInitMessage,
		}
	}

	// ...fetch the device...
	dev, err := findDeviceBySerial(ctx, c.storage, initReq.GetDeviceData().GetOsType(), initReq.GetDeviceData().GetSerialNumber())
	if err != nil {
		return nil, nil, auditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: "device not found",
		}
	}

	// ...and always return it, so callers can write audit logs against it.
	successResp, err := c.authenticate(ctx, stream, initReq, dev, user)
	return dev, successResp, trace.Wrap(err)
}

func (c *authnCeremony) authenticate(
	ctx context.Context,
	stream authenticateDeviceStream,
	initReq *devicepb.AuthenticateDeviceInit,
	dev *devicepb.Device,
	user string,
) (*devicepb.AuthenticateDeviceResponse, error) {
	// Perform the remaining init validation.
	// Note that we let auth validate the user certificates.
	// Additionally, we don't require UserCertificates.SshAuthorizedKey to be
	// present.
	if err := protectReadOnlyDeviceDataFields(initReq.GetDeviceData()); err != nil {
		return nil, auditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: invalidInitMessage,
		}
	}
	if err := storage.ValidateCollectedData(initReq.GetDeviceData()); err != nil {
		return nil, auditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: "invalid device collected data",
		}
	}
	switch {
	case initReq.GetCredentialId() == "":
		return nil, auditStatusError{
			Err:         trace.BadParameter("credential ID required"),
			UserMessage: invalidInitMessage,
		}
	case dev.GetEnrollStatus() != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		const deviceNotEnrolled = "device not enrolled"
		return nil, auditStatusError{
			Err:         trace.BadParameter("%s", deviceNotEnrolled),
			UserMessage: deviceNotEnrolled,
		}
	// Sanity check, this shouldn't happen for an enrolled device.
	case !dev.HasCredential():
		c.logger.ErrorContext(ctx,
			"Internal: Enrolled device has nil credential",
			"device_id", dev.GetId(),
			"asset_tag", dev.GetAssetTag(),
		)
		return nil, trace.Wrap(errors.New("device has no registered credential"))
	case dev.GetCredential().GetId() != initReq.GetCredentialId():
		const unknownCredential = "unknown device credential"
		return nil, auditStatusError{
			Err:         trace.BadParameter("%s", unknownCredential),
			UserMessage: unknownCredential,
		}
	}

	// Always ignore user-supplied X.509 certs, we get those either from the
	// user's TLS cert (regular authn) or from the WebSession (web authn).
	if initReq.HasUserCertificates() {
		initReq.GetUserCertificates().SetX509Der(nil)
	}

	// Device Web Authentication related logic.
	confirmToken, err := c.processDeviceWebToken(ctx, initReq.GetDeviceWebToken(), dev, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Hand off to the platform dependent implementations
	sshChallenge, err := c.authenticateDevicePlatform(ctx, dev, stream, initReq)
	if err != nil {
		c.deleteConfirmToken(ctx, confirmToken)
		return nil, trace.Wrap(err)
	}

	sshAuthorizedKey := initReq.GetUserCertificates().GetSshAuthorizedKey()
	sshKeySatisfiedChallenge, err := sshChallenge.verify(sshAuthorizedKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Augment end-user certificates?
	var resp *devicepb.AuthenticateDeviceResponse
	if confirmToken == nil {
		var err error
		resp, err = c.augmentEndUserCerts(ctx, &auth.AugmentUserCertificateOpts{
			DeviceExtensions: &auth.DeviceExtensions{
				DeviceID:     dev.GetId(),
				AssetTag:     dev.GetAssetTag(),
				CredentialID: dev.GetCredential().GetId(),
			},
			SSHAuthorizedKey:         sshAuthorizedKey,
			SSHKeySatisfiedChallenge: sshKeySatisfiedChallenge,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		resp = devicepb.AuthenticateDeviceResponse_builder{
			ConfirmationToken: proto.ValueOrDefault(confirmToken),
		}.Build()
	}

	// Record collected data.
	if err := c.storage.RecordDeviceAuthnData(ctx, dev.GetId(), initReq.GetDeviceData()); err != nil {
		return nil, trace.Wrap(err)
	}

	// 4. User certificates.
	return resp, nil
}

// processDeviceWebToken validates the webToken, spends it and returns the
// associated web session data.
//
// Returns `nil, nil` if there is no webToken.
func (c *authnCeremony) processDeviceWebToken(
	ctx context.Context,
	webToken *devicepb.DeviceWebToken,
	dev *devicepb.Device,
	user string,
) (*devicepb.DeviceConfirmationToken, error) {
	if webToken == nil {
		return nil, nil
	}

	// Validate Web Token.
	switch {
	case webToken.GetId() == "":
		return nil, auditStatusError{
			Err:         trace.BadParameter("device web token ID required"),
			UserMessage: invalidInitMessage,
		}
	case webToken.GetToken() == "":
		return nil, auditStatusError{
			Err:         trace.BadParameter("device web token plaintext token required"),
			UserMessage: invalidInitMessage,
		}
	}

	// Spend the token immediately, regardless of outcome.
	storedToken, confirmToken, err := c.storage.SpendDeviceWebToken(ctx, webToken, dev.GetId())
	if err != nil {
		c.logger.DebugContext(ctx,
			"AuthenticateDevice: device web authentication attempt failed",
			"error", err,
		)
		// err swallowed on purpose.
		return nil, auditStatusError{
			Err:         trace.Wrap(dterrors.ErrInvalidDeviceWebToken),
			UserMessage: invalidDeviceWebTokenMessage,
		}
	}

	if err := c.validateDeviceWebToken(ctx, storedToken, dev, user); err != nil {
		c.deleteConfirmToken(ctx, confirmToken)
		return nil, trace.Wrap(err)
	}

	return confirmToken, nil
}

func (c *authnCeremony) validateDeviceWebToken(
	ctx context.Context,
	storedToken *devicepb.DeviceWebToken,
	dev *devicepb.Device,
	user string,
) error {
	switch {
	// User must match token.
	case storedToken.GetUser() != user:
		return auditStatusError{
			// Use a nicer message than [dterrors.ErrInvalidDeviceWebToken] here, this
			// can happen in certain legitimate situations (like Connect using the
			// wrong user).
			Err:         trace.AccessDenied("the user being confirmed does not match the logged in user"),
			UserMessage: "device web token user mismatch",
		}
	// User must match device owner.
	case dev.GetOwner() != user:
		return auditStatusError{
			Err:         trace.Wrap(dterrors.ErrInvalidDeviceWebToken),
			UserMessage: "device web authentication owner mismatch",
		}
	}

	// Verify expected device IDs.
	deviceFound := slices.Contains(storedToken.GetExpectedDeviceIds(), dev.GetId())
	if !deviceFound {
		return auditStatusError{
			Err:         trace.Wrap(dterrors.ErrInvalidDeviceWebToken),
			UserMessage: "device web authentication expected device mismatch",
		}
	}

	// Verify user IP.
	sourceIP, err := getSourceIPFromContext(ctx)
	if err != nil {
		c.logger.DebugContext(ctx,
			"AuthenticateDevice: failed to get source IP from context",
			"error", err,
		)
		return trace.Wrap(dterrors.ErrInvalidDeviceWebToken)
	}
	if sourceIP != storedToken.GetBrowserIp() {
		c.logger.DebugContext(ctx,
			"Device web authentication IP mismatch",
			"source_ip", sourceIP,
			"token_ip", storedToken.GetBrowserIp(),
		)

		message := fmt.Sprintf("device web authentication IP mismatch (want %s, got %s)", storedToken.GetBrowserIp(), sourceIP)
		return auditStatusError{
			Err:         trace.Wrap(dterrors.ErrInvalidDeviceWebToken),
			UserMessage: message,
		}
	}

	return nil
}

func (c *authnCeremony) deleteConfirmToken(ctx context.Context, confirmToken *devicepb.DeviceConfirmationToken) {
	if confirmToken.GetId() == "" {
		return
	}

	ctx = context.WithoutCancel(ctx) // Delete always happens
	if err := c.storage.DeleteDeviceWebAuthenticationAttempt(ctx, confirmToken.GetId()); err != nil {
		c.logger.DebugContext(ctx,
			"Failed to delete device authentication attempt on error",
			"error", err,
		)
	}
}

func (c *authnCeremony) augmentEndUserCerts(
	ctx context.Context,
	opts *auth.AugmentUserCertificateOpts,
) (*devicepb.AuthenticateDeviceResponse, error) {
	// This is allowed for assertion ceremonies.
	// Return an empty UserCertificates struct.
	if c.augmentCertsFunc == nil {
		return devicepb.AuthenticateDeviceResponse_builder{
			UserCertificates: &devicepb.UserCertificates{},
		}.Build(), nil
	}

	newCerts, err := c.augmentCertsFunc(ctx, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Decode TLS PEM to DER.
	// The SSH certificate is already in the authorized_key format, despite what
	// other comments might say.
	block, _ := pem.Decode(newCerts.TLS)
	if block == nil {
		return nil, trace.BadParameter("failed to decode X.509 PEM from Teleport CA")
	}
	x509DER := block.Bytes

	return devicepb.AuthenticateDeviceResponse_builder{
		UserCertificates: devicepb.UserCertificates_builder{
			X509Der:          x509DER,
			SshAuthorizedKey: newCerts.SSH,
		}.Build(),
	}.Build(), nil
}

func (c *authnCeremony) authenticateDevicePlatform(
	ctx context.Context,
	dev *devicepb.Device,
	stream authenticateDeviceStream,
	initReq *devicepb.AuthenticateDeviceInit,
) (*sshChallenge, error) {
	switch dev.GetOsType() {
	case devicepb.OSType_OS_TYPE_MACOS:
		return c.authenticateDeviceMacOS(ctx, dev, stream)
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		return c.authenticateDeviceTPM(ctx, dev, stream, initReq)
	default:
		return nil, trace.BadParameter("unsupported OS type: %v", dtoss.FriendlyOSType(dev.GetOsType()))
	}
}

func (c *authnCeremony) authenticateDeviceMacOS(
	ctx context.Context,
	dev *devicepb.Device,
	stream authenticateDeviceStream,
) (*sshChallenge, error) {
	pubKey, err := x509.ParsePKIXPublicKey(dev.GetCredential().GetPublicKeyDer())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 2. Challenge.
	chal, err := challenge.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(devicepb.AuthenticateDeviceResponse_builder{
		Challenge: devicepb.AuthenticateDeviceChallenge_builder{
			Challenge: chal,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chalResp := resp.GetChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("bad payload, expected AuthenticateDeviceChallengeResponse")
	case len(chalResp.GetSignature()) == 0:
		return nil, trace.BadParameter("signature required")
	}
	if err := challenge.Verify(chal, chalResp.GetSignature(), pubKey); err != nil {
		c.logger.DebugContext(ctx,
			"AuthenticateDevice: signature verification failed",
			"error", err,
		)
		return nil, auditStatusError{
			Err:         trace.BadParameter("signature verification failed"),
			UserMessage: deviceAuthnFailedMessage,
		}
	}

	return &sshChallenge{
		challenge: chal,
		signature: chalResp.GetSshSignature(),
	}, nil
}

// authenticateDeviceTPM issues a platform attestation challenge based on the
// known AK of the device (from enrollment). The device completes the platform
// attestation and returns this to the server, where we can then validate that
// the platform attestation includes the nonce the server provided and that
// the quotes within the attestation are signed by the known AK.
func (c *authnCeremony) authenticateDeviceTPM(
	ctx context.Context,
	dev *devicepb.Device,
	stream authenticateDeviceStream,
	initReq *devicepb.AuthenticateDeviceInit,
) (*sshChallenge, error) {
	// 2. Issue challenge
	nonce, finishPlatformAttestation, err := platformAttestationChallenge(
		dev.GetOsType(),
		dev.GetCredential().GetTpmAkPublic(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.logger.DebugContext(ctx, "AuthenticateDevice : Sending TPM authentication challenge")
	if err := stream.Send(devicepb.AuthenticateDeviceResponse_builder{
		TpmChallenge: devicepb.TPMAuthenticateDeviceChallenge_builder{
			AttestationNonce: nonce,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	c.logger.DebugContext(ctx, "AuthenticateDevice: Received TPM authentication challenge response")
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chalResp := resp.GetTpmChallengeResponse()
	if chalResp == nil {
		return nil, trace.BadParameter("bad payload, expected TPMAuthenticateDeviceChallengeResponse")
	}
	platformAttestation, err := finishPlatformAttestation(
		dtoss.PlatformParametersFromProto(chalResp.GetPlatformParameters()),
	)
	if err != nil {
		c.logger.DebugContext(ctx,
			"TPM platform attestation failed verification",
			"error", err,
		)
		return nil, auditStatusError{
			Err:         trace.BadParameter("platform attestation verification failed"),
			UserMessage: UserMessage(err), // Use the message from finishPlatformAttestation.
		}
	}

	// Persist platform attestation record in collected data.
	initReq.GetDeviceData().SetTpmPlatformAttestation(platformAttestation)

	return &sshChallenge{
		challenge: nonce,
		signature: chalResp.GetSshSignature(),
	}, nil
}

// sshChallenge holds a completed SSH challenge and response that can be
// verified to have been signed by the subject key of an SSH certificate.
type sshChallenge struct {
	challenge []byte
	signature []byte
}

// verify returns true iff [c.signature] is a valid signature over [c.challenge]
// by the subject key of [sshAuthorizedKey]. [sshAuthorizedKey] is expected to
// be an SSH certificate in authorized keys format.
//
// verify returns false with no error if [c.signature] is empty. This if for
// backward compatibility with older clients that don't send an SSH signature
// over the challenge. In this case we must augment the SSH cert iff the SSH
// subject key matches the TLS subject key exactly.
func (c *sshChallenge) verify(sshAuthorizedKey []byte) (bool, error) {
	switch {
	case len(c.signature) == 0:
		return false, nil
	case len(c.challenge) == 0:
		return false, trace.BadParameter("challenge required")
	case len(sshAuthorizedKey) == 0:
		return false, trace.BadParameter("sshAuthorizedKey required")
	}

	sshCert, err := sshutils.ParseCertificate(sshAuthorizedKey)
	if err != nil {
		return false, trace.Wrap(err, "parsing SSH certificate")
	}
	var pubKey crypto.PublicKey
	if cryptoKey, ok := sshCert.Key.(ssh.CryptoPublicKey); ok {
		pubKey = cryptoKey.CryptoPublicKey()
	} else {
		return false, trace.BadParameter("unsupported SSH public key type %T", sshCert.Key)
	}
	if err := challenge.Verify(c.challenge, c.signature, pubKey); err != nil {
		return false, auditStatusError{
			Err:         trace.BadParameter("SSH key verification failed: %v", err),
			UserMessage: "SSH key verification failed",
		}
	}
	return true, nil
}

func (c *authnCeremony) backfillDeviceOwner(ctx context.Context, dev *devicepb.Device, user string) {
	if c.skipOwnerBackfill {
		return
	}

	owner := dev.GetOwner()
	backfill := false

	// Is the owner empty? Backfill.
	if owner == "" {
		owner = user
		backfill = true
	}

	// Is the device written to the owner's devices?
	// Local users only, SSO users are ephemeral.
	if !backfill {
		u, err := c.cachedUsers.GetUser(ctx, owner, false /* withSecrets */)
		backfill = err == nil && u.GetUserType() == types.UserTypeLocal && !slices.Contains(u.GetTrustedDeviceIDs(), dev.GetId())
	}

	// Is the user->device index up-to-date?
	if !backfill {
		deviceIDs, err := c.storage.GetUserTrustedDeviceIDs(ctx, owner)
		backfill = err != nil || !slices.Contains(deviceIDs, dev.GetId())
	}

	if !backfill {
		return
	}

	c.logger.DebugContext(ctx,
		"Backfilling device owner",
		"device_id", dev.GetId(),
		"asset_tag", dev.GetAssetTag(),
		"owner", owner,
	)
	if _, err := c.storage.AssignDeviceOwner(ctx, dev.GetId(), owner); err != nil {
		c.logger.WarnContext(ctx,
			"Failed to backfill device owner or user trusted device IDs",
			"error", err,
			"device_id", dev.GetId(),
			"asset_tag", dev.GetAssetTag(),
			"owner", owner,
		)
	}
}
