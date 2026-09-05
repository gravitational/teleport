package devicetrustv1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dterrors "github.com/gravitational/teleport/e/lib/devicetrust/errors"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

var errDeniedByNonAutoToken = errors.New("user lacks permissions to spend non auto-enroll token")

// EnrollDeviceCeremonyParams configures [RunEnrollDeviceCeremony].
type EnrollDeviceCeremonyParams struct {
	Logger  *slog.Logger
	Storage *storage.S
	// AuditCallback is called exactly once with the ceremony outcome, either
	// after the first error or before the last Send of the stream.
	AuditCallback func(d *devicepb.Device, err error)
	// AllowedOSTypes gates which device OS types may enroll through the calling
	// RPC surface: desktop OS types belong to the private Device Trust service,
	// mobile ones to the public service. See RFD 32e.
	AllowedOSTypes []devicepb.OSType
	// User the enrollment token must be bound to.
	User string
}

// RunEnrollDeviceCeremony runs the enrollment ceremony on behalf of RPC
// surfaces outside the package, such as the public Device Trust service.
//
// Returns the enrolled device and an error. As long as any device information
// is acquired from the stream, a non-nil device is returned, even if the
// ceremony itself failed.
//
// tokenSpent reports whether the ceremony got as far as spending the enrollment
// token. A failure with tokenSpent set cannot be retried with the same token.
func RunEnrollDeviceCeremony(
	stream devicepb.DeviceTrustService_EnrollDeviceServer,
	params EnrollDeviceCeremonyParams,
) (dev *devicepb.Device, tokenSpent bool, err error) {
	c := &enrollCeremony{
		logger:         params.Logger,
		storage:        params.Storage,
		auditCallback:  params.AuditCallback,
		allowedOSTypes: params.AllowedOSTypes,
	}
	// The auto-enroll exemption belongs to the private service's authorization
	// path. Callers of this function authorize on their own terms before the
	// ceremony runs, so allowedByAutoEnroll has nothing to restrict here. See
	// [enrollCeremony.EnrollDevice].
	dev, err = c.EnrollDevice(stream, params.User, false /* allowedByAutoEnroll */)
	return dev, c.tokenSpent, trace.Wrap(err)
}

type enrollCeremony struct {
	logger           *slog.Logger
	storage          *storage.S
	auditCallback    func(d *devicepb.Device, err error)
	ekCertAllowedCAs []string
	// allowedOSTypes gates which device OS types may enroll through the calling
	// RPC surface: desktop OS types belong to the private Device Trust service,
	// mobile ones to the public service. See RFD 32e.
	allowedOSTypes []devicepb.OSType
	// tokenSpent is set once SpendDeviceEnrollToken succeeds, so that callers can
	// tell a failure that left the token intact from one that consumed it.
	tokenSpent bool
}

// EnrollDevice implements the device enrollment ceremony, as described by
// devicepb.DeviceTrustService.EnrollDevice.
//
// Returns the enrolled device and an error.
//
// As long as any device information is acquired from the stream, a non-nil
// device is returned, even if the ceremony itself failed.
//
// The ceremony auditCallback is guaranteed to be called exactly once, either
// after the first error or before the last Send of the stream.
// The outcome of the last Send is not considered for audit purposes.
//
// user is the caller the token is spent for. A token bound to a user can only
// be spent by that user. Admin-issued tokens carry no user and are spendable by
// whoever the caller authenticated.
//
// allowedByAutoEnroll records that the caller passed authorization only through
// the cluster's auto-enroll exemption, after failing the device.enroll rule
// check. Such a caller may enroll only with tokens minted by auto-enroll, so
// the exemption does not extend to admin-issued tokens. A caller that passed
// the rule check outright passes false, which restricts nothing.
func (c *enrollCeremony) EnrollDevice(
	stream devicepb.DeviceTrustService_EnrollDeviceServer,
	user string,
	allowedByAutoEnroll bool,
) (*devicepb.Device, error) {
	dev, err := c.enrollDevice(stream, user, allowedByAutoEnroll)
	c.auditCallback(dev, err)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Success (only send after audit).
	err = stream.Send(devicepb.EnrollDeviceResponse_builder{
		Success: devicepb.EnrollDeviceSuccess_builder{
			Device: dev,
		}.Build(),
	}.Build())
	return dev, trace.Wrap(err)
}

func (c *enrollCeremony) enrollDevice(
	stream devicepb.DeviceTrustService_EnrollDeviceServer,
	user string,
	allowedByAutoEnroll bool,
) (*devicepb.Device, error) {
	// 1. Init.
	req, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Do some basic checks, so we can fetch the device...
	initReq := req.GetInit()
	switch {
	case initReq == nil:
		return nil, trace.BadParameter("bad payload, expected EnrollDeviceInit")
	case initReq.GetToken() == "":
		return nil, trace.BadParameter("enrollment token required")
	case !initReq.HasDeviceData():
		return nil, trace.BadParameter("device data required")
	case initReq.GetDeviceData().GetOsType() == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return nil, trace.BadParameter("device OS type required")
	case initReq.GetDeviceData().GetSerialNumber() == "":
		return nil, trace.BadParameter("device serial number required")
	}

	if !slices.Contains(c.allowedOSTypes, initReq.GetDeviceData().GetOsType()) {
		return nil, trace.BadParameter("unsupported OS type: %v",
			dtoss.FriendlyOSType(initReq.GetDeviceData().GetOsType()))
	}

	// ...fetch the device...
	ctx := stream.Context()
	dev, err := findDeviceBySerial(ctx, c.storage, initReq.GetDeviceData().GetOsType(), initReq.GetDeviceData().GetSerialNumber())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// from here onwards always return the device, so we can record audit logs
	// against it.

	// ...then immediately spend the enrollment token.
	tokenData, err := c.storage.SpendDeviceEnrollToken(ctx, dev.GetId(), initReq.GetToken())
	if err != nil {
		// err swallowed/obscured on purpose.
		return dev, trace.Wrap(dterrors.ErrInvalidDeviceEnrollToken)
	}
	c.tokenSpent = true
	if allowedByAutoEnroll && !tokenData.CreatedByAutoEnroll {
		return dev, trace.Wrap(errDeniedByNonAutoToken)
	}
	// A token minted for a specific user, like the mobile enrollment from RFD
	// 32e, can only be spent by that user. This stops a different authenticated
	// caller from enrolling the device onto their own account with a token
	// intercepted from the unauthenticated mobile flow. Admin issued tokens carry
	// no user and stay spendable by whoever the ceremony authenticates.
	if tokenData.User != "" && tokenData.User != user {
		message := fmt.Sprintf("enrollment token user mismatch (want %s, got %s)", tokenData.User, user)
		return dev, auditStatusError{
			Err:         trace.Wrap(dterrors.ErrInvalidDeviceEnrollToken),
			UserMessage: message,
		}
	}

	// Perform remaining init validation.
	if initReq.GetCredentialId() == "" {
		return dev, trace.BadParameter("credential ID required")
	}

	// Run a few storage validations manually, so we catch errors and mismatches
	// before continuing the ceremony.
	if err := protectReadOnlyDeviceDataFields(initReq.GetDeviceData()); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := storage.ValidateCollectedData(initReq.GetDeviceData()); err != nil {
		return dev, trace.Wrap(err)
	}
	if err := storage.ValidateCollectedDataAgainstDevice(initReq.GetDeviceData(), dev); err != nil {
		return dev, trace.Wrap(err)
	}

	// Fan out according to the OS type.
	var cred *devicepb.DeviceCredential
	switch dev.GetOsType() {
	case devicepb.OSType_OS_TYPE_MACOS:
		cred, err = c.enrollDeviceMacOS(initReq, dev, stream)
	case devicepb.OSType_OS_TYPE_IOS, devicepb.OSType_OS_TYPE_IPADOS:
		// iOS and iPadOS keys live in Secure Enclave like macOS keys, and the
		// public service adapts their payloads onto the macOS fields, so the macOS
		// flow applies as is.
		cred, err = c.enrollDeviceMacOS(initReq, dev, stream)
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		cred, err = c.enrollDeviceTPM(initReq, dev, stream)
	default:
		return dev, trace.BadParameter("unsupported OS type: %v", dtoss.FriendlyOSType(dev.GetOsType()))
	}
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Update stored device.
	enrolled, err := c.storage.EnrollDevice(ctx, dev.GetId(), cred, initReq.GetDeviceData(), user)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	return enrolled, nil
}

func (c *enrollCeremony) enrollDeviceMacOS(
	initReq *devicepb.EnrollDeviceInit,
	dev *devicepb.Device,
	stream devicepb.DeviceTrustService_EnrollDeviceServer) (*devicepb.DeviceCredential, error) {
	// Verify macOS data.
	if !initReq.HasMacos() {
		return nil, trace.BadParameter("macOS enrollment payload required")
	}
	cred := devicepb.DeviceCredential_builder{
		Id:           initReq.GetCredentialId(),
		PublicKeyDer: initReq.GetMacos().GetPublicKeyDer(),
	}.Build()
	pubKey, err := storage.ValidateDeviceCredential(cred, dev.GetOsType())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Enclave keys are always ECDSA, P-256.
	// Let's make sure the key is ECDSA and its length is sufficient, but leaving
	// some margin for the future.
	ctx := stream.Context()
	switch ecKey, ok := pubKey.(*ecdsa.PublicKey); {
	case !ok:
		return nil, trace.BadParameter("unexpected public key type: %T", pubKey)
	case ecKey.Curve == elliptic.P224():
		return nil, trace.BadParameter("public key length too small, expected P-256")
	case ecKey.Curve != elliptic.P256():
		c.logger.WarnContext(ctx,
			"Unexpected macOS public key curve found, is the device genuine?",
			"device_id", dev.GetId(),
			"asset_tag", dev.GetAssetTag(),
			"credential_id", cred.GetId(),
			"curve", ecKey.Curve,
		)
		// TODO(codingllama): Forbid unexpected macOS key curve?
	}

	// 2. Challenge.
	chal, err := challenge.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(devicepb.EnrollDeviceResponse_builder{
		MacosChallenge: devicepb.MacOSEnrollChallenge_builder{
			Challenge: chal,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	chalResp := resp.GetMacosChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("bad payload, expected MacOSEnrollChallengeResponse")
	case len(chalResp.GetSignature()) == 0:
		return nil, trace.BadParameter("signature required")
	}
	if err := challenge.Verify(chal, chalResp.GetSignature(), pubKey); err != nil {
		c.logger.DebugContext(ctx,
			"EnrollDevice: signature verification failed",
			"error", err,
		)
		return nil, trace.BadParameter("signature verification failed")
	}

	return cred, nil
}
