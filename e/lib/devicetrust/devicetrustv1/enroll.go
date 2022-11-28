package devicetrustv1

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	_ "crypto/sha256" // imported for crypto.SHA256

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/challenge"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

type enrollCeremony struct {
	logger  *log.Entry
	storage *storage.S
}

// EnrollDevice implements the device enrollment ceremony, as described by
// devicepb.DeviceTrustService.EnrollDevice.
//
// Returns the enrolled device and an error.
//
// As long as any device information is acquired from the stream, a non-nil
// device is returned, even if the ceremony itself failed. This allows callers
// to write audit information about the device.
func (c *enrollCeremony) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer) (*devicepb.Device, error) {
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
	dev, err := c.findDeviceBySerial(ctx, initReq.DeviceData.OsType, initReq.DeviceData.SerialNumber)
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
		return nil, trace.BadParameter("credential ID required")
	}
	// Run a few storage validations manually, so we catch errors and mismatches
	// before continuing the ceremony.
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
	default:
		return dev, trace.BadParameter("unsupported OS type: %v", dtoss.FriendlyOSType(dev.OsType))
	}
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Update stored device.
	enrolled, err := c.storage.EnrollDevice(ctx, dev.Id, cred, initReq.DeviceData)
	if err != nil {
		return dev, trace.Wrap(err)
	}

	// Success.
	err = stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_Success{
			Success: &devicepb.EnrollDeviceSuccess{
				Device: enrolled,
			},
		},
	})
	return enrolled, trace.Wrap(err)
}

func (c *enrollCeremony) findDeviceBySerial(ctx context.Context, osType devicepb.OSType, serialNumber string) (*devicepb.Device, error) {
	devs, err := c.storage.GetDevicesByAssetTag(ctx, serialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, dev := range devs {
		if dev.OsType == osType {
			return dev, nil
		}
	}
	return nil, trace.NotFound("device %v/%v not registered", serialNumber, dtoss.FriendlyOSType(osType))
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
	pubKey, err := storage.ValidateDeviceCredential(cred)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Enclave keys are always ECDSA, P-256.
	// Let's make sure the key is ECDSA and its length is sufficient, but leaving
	// some margin for the future.
	switch ecKey, ok := pubKey.(*ecdsa.PublicKey); {
	case !ok:
		return nil, trace.BadParameter("unexpected public key type: %T", pubKey)
	case ecKey.Curve == elliptic.P224():
		return nil, trace.BadParameter("public key length too small, expected P-256")
	case ecKey.Curve != elliptic.P256():
		c.logger.
			WithFields(log.Fields{
				"DeviceID":     dev.Id,
				"AssetTag":     dev.AssetTag,
				"CredentialID": cred.Id,
				"Curve":        ecKey.Curve,
			}).
			Warn("Unexpected macOS public key curve found, is the device genuine?")
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
		c.logger.WithError(err).Debug("EnrollDevice: signature verification failed")
		return nil, trace.BadParameter("signature verification failed")
	}

	return cred, nil
}
