package internal

import (
	"context"
	"crypto"
	_ "crypto/sha256" // imported for crypto.SHA256
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/challenge"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

const (
	deviceAuthnFailedMessage     = "device authentication failed"
	invalidInitMessage           = "invalid initial payload"
	invalidDeviceWebTokenMessage = "invalid device web token"
)

var errInvalidDeviceWebToken = &trace.AccessDeniedError{Message: invalidDeviceWebTokenMessage}

// UsersService represents the [local.IdentityService] methods used by
// [AuthnCeremony].
type UsersService interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// DeviceAuthnAuditData holds additional audit data used by
// AuthnCeremony.AuditCallback.
type DeviceAuthnAuditData struct {
	HasDeviceWebToken   bool
	WebAuthenticationID string
}

// AuthenticateDeviceStream abstracts
// devicepb.DeviceTrustService_AuthenticateDeviceServer.
type AuthenticateDeviceStream interface {
	Send(*devicepb.AuthenticateDeviceResponse) error
	Recv() (*devicepb.AuthenticateDeviceRequest, error)
}

// AuthnCeremony is the device authentication ceremony.
type AuthnCeremony struct {
	Logger  *slog.Logger
	Storage *storage.S
	// SkipOwnerBackfill skips backfilling device owners if set to true.
	SkipOwnerBackfill bool
	CachedUsers       UsersService
	// AugmentCertsFunc calls its namesake auth.Server function.
	// May be nil for ceremonies that don't issue new certificates (like device
	// assertion ceremonies)
	AugmentCertsFunc func(ctx context.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error)
	AuditCallback    func(dev *devicepb.Device, auditData *DeviceAuthnAuditData, err error)
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
func (c *AuthnCeremony) AuthenticateDevice(
	ctx context.Context,
	stream AuthenticateDeviceStream,
	user string,
) (*devicepb.Device, error) {
	var auditData DeviceAuthnAuditData
	dev, successResp, err := c.authenticateDevice(ctx, stream, user, &auditData)
	c.AuditCallback(dev, &auditData, err)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Attempt to "backfill" owner and trusted device IDs.
	if !c.SkipOwnerBackfill {
		owner := dev.Owner
		backfill := false
		if owner == "" {
			owner = user
			backfill = true
		} else {
			u, err := c.CachedUsers.GetUser(ctx, owner, false /* withSecrets */)
			backfill = err == nil && !slices.Contains(u.GetTrustedDeviceIDs(), dev.Id)
		}
		if backfill {
			c.Logger.DebugContext(ctx,
				"Backfilling device owner",
				"device_id", dev.Id,
				"asset_tag", dev.AssetTag,
				"owner", owner,
			)
			if _, err := c.Storage.AssignDeviceOwner(ctx, dev.Id, owner); err != nil {
				c.Logger.WarnContext(ctx,
					"Failed to backfill device owner or user trusted device IDs",
					"error", err,
					"device_id", dev.Id,
					"asset_tag", dev.AssetTag,
					"owner", owner,
				)
			}
		}
	}

	// Success (only send after audit).
	return dev, trace.Wrap(stream.Send(successResp))
}

func (c *AuthnCeremony) authenticateDevice(
	ctx context.Context,
	stream AuthenticateDeviceStream,
	user string,
	auditData *DeviceAuthnAuditData,
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
	case initReq.DeviceData == nil:
		err = trace.BadParameter("device data required")
	case initReq.DeviceData.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		err = trace.BadParameter("device OS type required")
	case initReq.DeviceData.SerialNumber == "":
		err = trace.BadParameter("device serial number required")
	}
	if err != nil {
		return nil, nil, AuditStatusError{
			Err:         err,
			UserMessage: invalidInitMessage,
		}
	}

	// ...fetch the device...
	dev, err := FindDeviceBySerial(ctx, c.Storage, initReq.DeviceData.OsType, initReq.DeviceData.SerialNumber)
	if err != nil {
		return nil, nil, AuditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: "device not found",
		}
	}

	// ...and always return it, so callers can write audit logs against it.
	successResp, err := c.authenticate(ctx, stream, initReq, dev, user)
	return dev, successResp, trace.Wrap(err)
}

func (c *AuthnCeremony) authenticate(
	ctx context.Context,
	stream AuthenticateDeviceStream,
	initReq *devicepb.AuthenticateDeviceInit,
	dev *devicepb.Device,
	user string,
) (*devicepb.AuthenticateDeviceResponse, error) {
	// Perform the remaining init validation.
	// Note that we let auth validate the user certificates.
	// Additionally, we don't require UserCertificates.SshAuthorizedKey to be
	// present.
	if err := ProtectReadOnlyDeviceDataFields(initReq.DeviceData); err != nil {
		return nil, AuditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: invalidInitMessage,
		}
	}
	if err := storage.ValidateCollectedData(initReq.DeviceData); err != nil {
		return nil, AuditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: "invalid device collected data",
		}
	}
	switch {
	case initReq.CredentialId == "":
		return nil, AuditStatusError{
			Err:         trace.BadParameter("credential ID required"),
			UserMessage: invalidInitMessage,
		}
	case dev.EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		const deviceNotEnrolled = "device not enrolled"
		return nil, AuditStatusError{
			Err:         trace.BadParameter(deviceNotEnrolled),
			UserMessage: deviceNotEnrolled,
		}
	// Sanity check, this shouldn't happen for an enrolled device.
	case dev.Credential == nil:
		c.Logger.ErrorContext(ctx,
			"Internal: Enrolled device has nil credential",
			"device_id", dev.Id,
			"asset_tag", dev.AssetTag,
		)
		return nil, trace.Wrap(errors.New("device has no registered credential"))
	case dev.Credential.Id != initReq.CredentialId:
		const unknownCredential = "unknown device credential"
		return nil, AuditStatusError{
			Err:         trace.BadParameter(unknownCredential),
			UserMessage: unknownCredential,
		}
	}

	// Always ignore user-supplied X.509 certs, we get those either from the
	// user's TLS cert (regular authn) or from the WebSession (web authn).
	if initReq.UserCertificates != nil {
		initReq.UserCertificates.X509Der = nil
	}

	// Device Web Authentication related logic.
	confirmToken, err := c.processDeviceWebToken(ctx, initReq.DeviceWebToken, dev, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Hand off to the platform dependent implementations
	var platformAttestation *devicepb.TPMPlatformAttestation
	switch dev.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = c.authenticateDeviceMacOS(ctx, dev, stream)
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		platformAttestation, err = c.authenticateDeviceTPM(ctx, dev, stream)
		// Persist platform attestation record in collected data.
		initReq.DeviceData.TpmPlatformAttestation = platformAttestation
	default:
		c.deleteConfirmToken(ctx, confirmToken)
		return nil, trace.BadParameter("unsupported OS type: %v", dtoss.FriendlyOSType(dev.OsType))
	}
	if err != nil {
		c.deleteConfirmToken(ctx, confirmToken)
		return nil, trace.Wrap(err)
	}

	// Augment end-user certificates?
	var resp *devicepb.AuthenticateDeviceResponse
	if confirmToken == nil {
		var err error
		resp, err = c.augmentEndUserCerts(ctx, initReq, dev)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		resp = &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_ConfirmationToken{
				ConfirmationToken: confirmToken,
			},
		}
	}

	// Record collected data.
	if err := c.Storage.RecordDeviceAuthnData(ctx, dev.Id, initReq.DeviceData); err != nil {
		return nil, trace.Wrap(err)
	}

	// 4. User certificates.
	return resp, nil
}

// processDeviceWebToken validates the webToken, spends it and returns the
// associated web session data.
//
// Returns `nil, nil` if there is no webToken.
func (c *AuthnCeremony) processDeviceWebToken(
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
	case webToken.Id == "":
		return nil, AuditStatusError{
			Err:         trace.BadParameter("device web token ID required"),
			UserMessage: invalidInitMessage,
		}
	case webToken.Token == "":
		return nil, AuditStatusError{
			Err:         trace.BadParameter("device web token plaintext token required"),
			UserMessage: invalidInitMessage,
		}
	}

	// Spend the token immediately, regardless of outcome.
	storedToken, confirmToken, err := c.Storage.SpendDeviceWebToken(ctx, webToken, dev.Id)
	if err != nil {
		c.Logger.DebugContext(ctx,
			"AuthenticateDevice: device web authentication attempt failed",
			"error", err,
		)
		// err swallowed on purpose.
		return nil, AuditStatusError{
			Err:         trace.Wrap(errInvalidDeviceWebToken),
			UserMessage: invalidDeviceWebTokenMessage,
		}
	}

	if err := c.validateDeviceWebToken(ctx, storedToken, dev, user); err != nil {
		c.deleteConfirmToken(ctx, confirmToken)
		return nil, trace.Wrap(err)
	}

	return confirmToken, nil
}

func (c *AuthnCeremony) validateDeviceWebToken(
	ctx context.Context,
	storedToken *devicepb.DeviceWebToken,
	dev *devicepb.Device,
	user string,
) error {
	switch {
	// User must match token.
	case storedToken.User != user:
		return AuditStatusError{
			// Use a nicer message than errInvalidDeviceWebToken here, this can happen
			// in certain legitimate situations (like Connect using the wrong user).
			Err:         trace.AccessDenied("the user being confirmed does not match the logged in user"),
			UserMessage: "device web token user mismatch",
		}
	// User must match device owner.
	case dev.Owner != user:
		return AuditStatusError{
			Err:         trace.Wrap(errInvalidDeviceWebToken),
			UserMessage: "device web authentication owner mismatch",
		}
	}

	// Verify expected device IDs.
	deviceFound := false
	for _, deviceID := range storedToken.ExpectedDeviceIds {
		if deviceID == dev.Id {
			deviceFound = true
			break
		}
	}
	if !deviceFound {
		return AuditStatusError{
			Err:         trace.Wrap(errInvalidDeviceWebToken),
			UserMessage: "device web authentication expected device mismatch",
		}
	}

	// Verify user IP.
	sourceIP, err := getSourceIPFromContext(ctx)
	if err != nil {
		c.Logger.DebugContext(ctx,
			"AuthenticateDevice: failed to get source IP from context",
			"error", err,
		)
		return trace.Wrap(errInvalidDeviceWebToken)
	}
	if sourceIP != storedToken.BrowserIp {
		return AuditStatusError{
			Err:         trace.Wrap(errInvalidDeviceWebToken),
			UserMessage: "device web authentication IP mismatch",
		}
	}

	return nil
}

func (c *AuthnCeremony) deleteConfirmToken(ctx context.Context, confirmToken *devicepb.DeviceConfirmationToken) {
	if confirmToken.GetId() == "" {
		return
	}

	ctx = context.WithoutCancel(ctx) // Delete always happens
	if err := c.Storage.DeleteDeviceWebAuthenticationAttempt(ctx, confirmToken.Id); err != nil {
		c.Logger.DebugContext(ctx,
			"Failed to delete device authentication attempt on error",
			"error", err,
		)
	}
}

func (c *AuthnCeremony) augmentEndUserCerts(
	ctx context.Context,
	initReq *devicepb.AuthenticateDeviceInit,
	dev *devicepb.Device,
) (*devicepb.AuthenticateDeviceResponse, error) {
	// This is allowed for assertion ceremonies.
	// Return an empty UserCertificates struct.
	if c.AugmentCertsFunc == nil {
		return &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
				UserCertificates: &devicepb.UserCertificates{},
			},
		}, nil
	}

	exts := &auth.DeviceExtensions{
		DeviceID:     dev.Id,
		AssetTag:     dev.AssetTag,
		CredentialID: dev.Credential.Id,
	}

	newCerts, err := c.AugmentCertsFunc(ctx, &auth.AugmentUserCertificateOpts{
		SSHAuthorizedKey: initReq.UserCertificates.GetSshAuthorizedKey(),
		DeviceExtensions: exts,
	})
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

	return &devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
			UserCertificates: &devicepb.UserCertificates{
				X509Der:          x509DER,
				SshAuthorizedKey: newCerts.SSH,
			},
		},
	}, nil
}

func (c *AuthnCeremony) authenticateDeviceMacOS(
	ctx context.Context,
	dev *devicepb.Device,
	stream AuthenticateDeviceStream,
) error {
	pubKey, err := x509.ParsePKIXPublicKey(dev.Credential.PublicKeyDer)
	if err != nil {
		return trace.Wrap(err)
	}

	// 2. Challenge.
	chal, err := challenge.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_Challenge{
			Challenge: &devicepb.AuthenticateDeviceChallenge{
				Challenge: chal,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	// 3. Challenge response.
	resp, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	chalResp := resp.GetChallengeResponse()
	switch {
	case chalResp == nil:
		return trace.BadParameter("bad payload, expected AuthenticateDeviceChallengeResponse")
	case len(chalResp.Signature) == 0:
		return trace.BadParameter("signature required")
	}
	if err := challenge.Verify(chal, chalResp.Signature, pubKey, crypto.SHA256); err != nil {
		c.Logger.DebugContext(ctx,
			"AuthenticateDevice: signature verification failed",
			"error", err,
		)
		return AuditStatusError{
			Err:         trace.BadParameter("signature verification failed"),
			UserMessage: deviceAuthnFailedMessage,
		}
	}

	return nil
}

// authenticateDeviceTPM issues a platform attestation challenge based on the
// known AK of the device (from enrollment). The device completes the platform
// attestation and returns this to the server, where we can then validate that
// the platform attestation includes the nonce the server provided and that
// the quotes within the attestation are signed by the known AK.
func (c *AuthnCeremony) authenticateDeviceTPM(
	ctx context.Context,
	dev *devicepb.Device,
	stream AuthenticateDeviceStream,
) (*devicepb.TPMPlatformAttestation, error) {
	// 2. Issue challenge
	nonce, finishPlatformAttestation, err := PlatformAttestationChallenge(
		dev.OsType,
		dev.Credential.TpmAkPublic,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.Logger.DebugContext(ctx, "AuthenticateDevice : Sending TPM authentication challenge")
	if err := stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_TpmChallenge{
			TpmChallenge: &devicepb.TPMAuthenticateDeviceChallenge{
				AttestationNonce: nonce,
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	c.Logger.DebugContext(ctx, "AuthenticateDevice: Received TPM authentication challenge response")
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chalResp := resp.GetTpmChallengeResponse()
	if chalResp == nil {
		return nil, trace.BadParameter("bad payload, expected TPMAuthenticateDeviceChallengeResponse")
	}
	platformAttestation, err := finishPlatformAttestation(
		dtoss.PlatformParametersFromProto(chalResp.PlatformParameters),
	)
	if err != nil {
		c.Logger.DebugContext(ctx,
			"TPM platform attestation failed verification",
			"error", err,
		)
		return nil, AuditStatusError{
			Err:         trace.BadParameter("platform attestation verification failed"),
			UserMessage: GetUserMessage(err), // Use the message from finishPlatformAttestation.
		}
	}

	return platformAttestation, nil
}
