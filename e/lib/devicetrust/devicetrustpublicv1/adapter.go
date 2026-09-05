package devicetrustpublicv1

import (
	"github.com/gravitational/trace"

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// enrollDeviceStreamAdapter presents the public EnrollDevice stream as the
// private one, so the enrollment ceremony from devicetrustv1 runs unchanged.
// iOS payloads ride in the macOS fields. Their contents are identical, both
// platforms enroll a Secure Enclave key. See RFD 32e.
type enrollDeviceStreamAdapter struct {
	devicetrustpublicv1pb.DeviceTrustService_EnrollDeviceServer

	// init is the already-read init message, replayed on the first Recv. The
	// handler consumes it from the stream to resolve the token user before the
	// ceremony starts.
	//
	// Recv is not safe for concurrent use, like the stream it wraps, so init
	// needs no lock.
	init *devicetrustpublicv1pb.EnrollDeviceInit
}

func (a *enrollDeviceStreamAdapter) Recv() (*devicepb.EnrollDeviceRequest, error) {
	if init := a.init; init != nil {
		a.init = nil
		return devicepb.EnrollDeviceRequest_builder{
			Init: devicepb.EnrollDeviceInit_builder{
				Token:        init.GetToken(),
				CredentialId: init.GetCredentialId(),
				DeviceData:   init.GetDeviceData(),
				Macos: devicepb.MacOSEnrollPayload_builder{
					PublicKeyDer: init.GetIos().GetPublicKeyDer(),
				}.Build(),
			}.Build(),
		}.Build(), nil
	}

	req, err := a.DeviceTrustService_EnrollDeviceServer.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp := req.GetIosChallengeResponse(); resp != nil {
		return devicepb.EnrollDeviceRequest_builder{
			MacosChallengeResponse: devicepb.MacOSEnrollChallengeResponse_builder{
				Signature: resp.GetSignature(),
			}.Build(),
		}.Build(), nil
	}
	return nil, trace.BadParameter("bad payload, expected IOSEnrollChallengeResponse")
}

func (a *enrollDeviceStreamAdapter) Send(resp *devicepb.EnrollDeviceResponse) error {
	switch {
	case resp.GetMacosChallenge() != nil:
		return trace.Wrap(a.DeviceTrustService_EnrollDeviceServer.Send(
			devicetrustpublicv1pb.EnrollDeviceResponse_builder{
				IosChallenge: devicetrustpublicv1pb.IOSEnrollChallenge_builder{
					Challenge: resp.GetMacosChallenge().GetChallenge(),
				}.Build(),
			}.Build(),
		))
	case resp.GetSuccess() != nil:
		return trace.Wrap(a.DeviceTrustService_EnrollDeviceServer.Send(
			devicetrustpublicv1pb.EnrollDeviceResponse_builder{
				Success: devicetrustpublicv1pb.EnrollDeviceSuccess_builder{
					Device: resp.GetSuccess().GetDevice(),
				}.Build(),
			}.Build(),
		))
	default:
		return trace.Errorf("enrollment ceremony produced a %q response the public stream cannot carry, this is a bug",
			resp.WhichPayload())
	}
}
