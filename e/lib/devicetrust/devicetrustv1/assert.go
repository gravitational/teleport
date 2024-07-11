package devicetrustv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
)

// assertCeremony implements [assertserver.Ceremony].
type assertCeremony struct {
	logger *slog.Logger
	impl   *authnCeremony
}

func (c *assertCeremony) AssertDevice(
	ctx context.Context,
	stream assertserver.AssertDeviceServerStream,
) (*devicepb.Device, error) {
	if c.impl == nil {
		return nil, trace.BadParameter("assert.Ceremony instance not properly initialized")
	}

	dev, err := c.impl.AuthenticateDevice(ctx, &assertStreamAdapter{
		logger: c.logger,
		ctx:    ctx,
		stream: stream,
	}, "" /* user */)
	return dev, trace.Wrap(err)
}

// assertStreamAdapter adapts an [assertserver.AssertDeviceServerStream] to an
// [internal.AuthenticateDeviceStream].
type assertStreamAdapter struct {
	logger *slog.Logger
	ctx    context.Context // Typically part of the stream.

	stream assertserver.AssertDeviceServerStream
}

func (s *assertStreamAdapter) Recv() (*devicepb.AuthenticateDeviceRequest, error) {
	req, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert AssertDeviceRequest to AuthenticateDeviceRequest.
	if req == nil || req.Payload == nil {
		return nil, trace.BadParameter("assert request payload required")
	}
	authnReq := &devicepb.AuthenticateDeviceRequest{}
	switch req.Payload.(type) {
	case *devicepb.AssertDeviceRequest_Init:
		init := req.GetInit()
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				CredentialId: init.GetCredentialId(),
				DeviceData:   init.GetDeviceData(),
			},
		}
	case *devicepb.AssertDeviceRequest_ChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: req.GetChallengeResponse(),
		}
	case *devicepb.AssertDeviceRequest_TpmChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: req.GetTpmChallengeResponse(),
		}
	default:
		return nil, trace.BadParameter("unexpected assert request payload: %T", req.Payload)
	}

	return authnReq, nil
}

func (s *assertStreamAdapter) Send(authnResp *devicepb.AuthenticateDeviceResponse) error {
	if authnResp == nil || authnResp.Payload == nil {
		return trace.BadParameter("authenticate response payload required")
	}

	// Convert AuthenticateDeviceResponse to AssertDeviceResponse.
	resp := &devicepb.AssertDeviceResponse{}
	switch authnResp.Payload.(type) {
	case *devicepb.AuthenticateDeviceResponse_Challenge:
		resp.Payload = &devicepb.AssertDeviceResponse_Challenge{
			Challenge: authnResp.GetChallenge(),
		}
	case *devicepb.AuthenticateDeviceResponse_TpmChallenge:
		resp.Payload = &devicepb.AssertDeviceResponse_TpmChallenge{
			TpmChallenge: authnResp.GetTpmChallenge(),
		}
	case *devicepb.AuthenticateDeviceResponse_UserCertificates:
		// An empty UserCertificates signifies success for assertion.
		certs := authnResp.GetUserCertificates()
		if len(certs.GetX509Der()) > 0 || len(certs.GetSshAuthorizedKey()) > 0 {
			// If this happens it means the internal.AuthnCeremony is issuing
			// certificates. We don't want that.
			s.logger.WarnContext(s.ctx,
				"AssertCeremony received non-empty UserCertificates",
				"has_x509", len(certs.GetX509Der()) > 0,
				"has_ssh_authorized_key", len(certs.SshAuthorizedKey) > 0,
			)
		}
		resp.Payload = &devicepb.AssertDeviceResponse_DeviceAsserted{
			DeviceAsserted: &devicepb.DeviceAsserted{},
		}
	case *devicepb.AuthenticateDeviceResponse_ConfirmationToken:
		// This shouldn't be possible, there's no way to pass a DeviceWebToken via
		// assertion requests.
		return trace.BadParameter("unallowed ConfirmationToken payload received from authentication ceremony")
	default:
		return trace.BadParameter("unexpected authentication response payload: %T", authnResp.Payload)
	}

	return trace.Wrap(s.stream.Send(resp))
}
