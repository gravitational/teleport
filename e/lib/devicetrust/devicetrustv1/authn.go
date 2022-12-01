package devicetrustv1

import (
	"crypto"
	_ "crypto/sha256" // imported for crypto.SHA256
	"crypto/x509"
	"errors"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/challenge"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

type authnCeremony struct {
	logger           *log.Entry
	storage          *storage.S
	augmentCertsFunc AugmentContextCertsFunc
}

// AuthenticateDevice implements the trusted device authentication ceremony, as
// described by devicepb.DeviceTrustService.AuthenticateDevice.
//
// Returns the authenticated device and an error.
//
// As long as any device information is acquired from the stream, a non-nil
// device is returned, even if the ceremony itself failed. This allows callers
// to write audit information about the device.
func (c *authnCeremony) AuthenticateDevice(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) (*devicepb.Device, error) {
	// 1. Init.
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Do some preliminary checks so we can fetch the device...
	initReq := resp.GetInit()
	switch {
	case initReq == nil:
		return nil, trace.BadParameter("bad payload, expected AuthenticateDeviceInit")
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

	// ...and always return it, so callers can write audit logs against it.
	err = c.authenticate(stream, initReq, dev)
	return dev, trace.Wrap(err)
}

func (c *authnCeremony) authenticate(stream devicepb.DeviceTrustService_AuthenticateDeviceServer, initReq *devicepb.AuthenticateDeviceInit, dev *devicepb.Device) error {
	// Perform the remaining init validation.
	// Note that we let auth validate the user certificates.
	// Additionally, we don't require UserCertificates.SshAuthorizedKey to be
	// present.
	if err := storage.ValidateCollectedData(initReq.DeviceData); err != nil {
		return trace.Wrap(err)
	}
	switch {
	case initReq.CredentialId == "":
		return trace.BadParameter("credential ID required")
	case dev.EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		return trace.BadParameter("device not enrolled")
	// Sanity check, this shouldn't happen for an enrolled device.
	case dev.Credential == nil:
		c.logger.
			WithFields(log.Fields{
				"DeviceID": dev.Id,
				"AssetTag": dev.AssetTag,
			}).
			Error("Internal: Enrolled device has nil credential")
		return trace.Wrap(errors.New("device has no registered credential"))
	case dev.Credential.Id != initReq.CredentialId:
		return trace.BadParameter("unknown device credential")
	}

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
		c.logger.WithError(err).Debug("AuthenticateDevice: signature verification failed")
		return trace.BadParameter("signature verification failed")
	}

	// Augment certificates.
	ctx := stream.Context()
	newCerts, err := c.augmentCertsFunc(ctx, initReq.UserCertificates)
	if err != nil {
		return trace.Wrap(err)
	}
	// Record collected data.
	if err := c.storage.RecordDeviceAuthnData(ctx, dev.Id, initReq.DeviceData); err != nil {
		return trace.Wrap(err)
	}

	// 4. User certificates.
	err = stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
			UserCertificates: newCerts,
		},
	})
	return trace.Wrap(err)
}
