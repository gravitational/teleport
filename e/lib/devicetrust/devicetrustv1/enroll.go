package devicetrustv1

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	_ "crypto/sha256" // imported for crypto.SHA256
	"log/slog"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/challenge"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

type enrollCeremony struct {
	logger           *slog.Logger
	storage          *storage.S
	auditCallback    func(d *devicepb.Device, err error)
	ekCertAllowedCAs []string
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
func (c *enrollCeremony) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer, user string) (*devicepb.Device, error) {
	dev, err := c.enrollDevice(stream, user)
	c.auditCallback(dev, err)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Success (only send after audit).
	err = stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_Success{
			Success: &devicepb.EnrollDeviceSuccess{
				Device: dev,
			},
		},
	})
	return dev, trace.Wrap(err)
}

func (c *enrollCeremony) enrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer, user string) (*devicepb.Device, error) {
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
	case initReq.Token == "":
		return nil, trace.BadParameter("enrollment token required")
	case initReq.DeviceData == nil:
		return nil, trace.BadParameter("device data required")
	case initReq.DeviceData.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return nil, trace.BadParameter("device OS type required")
	case initReq.DeviceData.SerialNumber == "":
		return nil, trace.BadParameter("device serial number required")
	}

	// ...fetch the device...
	ctx := stream.Context()
	dev, err := findDeviceBySerial(ctx, c.storage, initReq.DeviceData.OsType, initReq.DeviceData.SerialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// from here onwards always return the device, so we can record audit logs
	// against it.

	// ...then immediately spend the enrollment token.
	if err := c.storage.SpendDeviceEnrollToken(ctx, dev.Id, initReq.Token); err != nil {
		// err swallowed/obscured on purpose.
		return dev, trace.AccessDenied("invalid device enrollment token")
	}

	// Perform remaining init validation.
	if initReq.CredentialId == "" {
		return dev, trace.BadParameter("credential ID required")
	}

	// Run a few storage validations manually, so we catch errors and mismatches
	// before continuing the ceremony.
	if err := protectReadOnlyDeviceDataFields(initReq.DeviceData); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := storage.ValidateCollectedData(initReq.DeviceData); err != nil {
		return dev, trace.Wrap(err)
	}
	if err := storage.ValidateCollectedDataAgainstDevice(initReq.DeviceData, dev); err != nil {
		return dev, trace.Wrap(err)
	}

	// Fan out according to the OS type.
	var cred *devicepb.DeviceCredential
	switch dev.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		cred, err = c.enrollDeviceMacOS(initReq, dev, stream)
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		cred, err = c.enrollDeviceTPM(initReq, dev, stream)
	default:
		return dev, trace.BadParameter("unsupported OS type: %v", dtoss.FriendlyOSType(dev.OsType))
	}
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Update stored device.
	enrolled, err := c.storage.EnrollDevice(ctx, dev.Id, cred, initReq.DeviceData, user)
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
	if initReq.Macos == nil {
		return nil, trace.BadParameter("macOS enrollment payload required")
	}
	cred := &devicepb.DeviceCredential{
		Id:           initReq.CredentialId,
		PublicKeyDer: initReq.Macos.GetPublicKeyDer(),
	}
	pubKey, err := storage.ValidateDeviceCredential(cred, dev.OsType)
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
			"device_id", dev.Id,
			"asset_tag", dev.AssetTag,
			"credential_id", cred.Id,
			"curve", ecKey.Curve,
		)
		// TODO(codingllama): Forbid unexpected macOS key curve?
	}

	// 2. Challenge.
	chal, err := challenge.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_MacosChallenge{
			MacosChallenge: &devicepb.MacOSEnrollChallenge{
				Challenge: chal,
			},
		},
	}); err != nil {
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
	case len(chalResp.Signature) == 0:
		return nil, trace.BadParameter("signature required")
	}
	if err := challenge.Verify(chal, chalResp.Signature, pubKey, crypto.SHA256); err != nil {
		c.logger.DebugContext(ctx,
			"EnrollDevice: signature verification failed",
			"error", err,
		)
		return nil, trace.BadParameter("signature verification failed")
	}

	return cred, nil
}
