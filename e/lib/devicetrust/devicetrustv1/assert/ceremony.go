package assert

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1/internal"
)

// AssertDeviceServerStream represents a server-side device assertion stream.
type AssertDeviceServerStream interface {
	Send(*devicepb.AssertDeviceResponse) error
	Recv() (*devicepb.AssertDeviceRequest, error)
}

// Ceremony is the device assertion ceremony.
//
// Device assertion is a light form of device authentication where the user
// isn't considered and no side-effects (like certificate issuance) happen.
//
// Assertion is meant to be embedded in RPCs or streams external to the
// DeviceTrustService itself.
type Ceremony struct {
	logger *slog.Logger
	impl   *internal.AuthnCeremony
}

// NewCeremony creates a new [Ceremony].
//
// This method is only viable inside the devicetrustv1 package. The recommended
// way to create a [Ceremony] is via a function pointer bound to
// devicetrustv1.Service.CreateAssertCeremony
func NewCeremony(params *internal.AssertParams) (*Ceremony, error) {
	switch {
	case params == nil:
		return nil, trace.BadParameter("params required")
	case params.Storage == nil:
		return nil, trace.BadParameter("params.Storage required")
	}

	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Ceremony{
		logger: logger,
		impl: &internal.AuthnCeremony{
			Logger:            logger,
			Storage:           params.Storage,
			SkipOwnerBackfill: true, // Don't backfill, caller may not be the owner.
			AugmentCertsFunc:  nil,  // Don't issue certificates.
			AuditCallback: func(*devicepb.Device, *internal.DeviceAuthnAuditData, error) {
				// Audit is responsibility of the caller.
			},
		},
	}, nil
}

// AssertDevice runs the device assertion ceremonies.
//
// Requests and responses are consumed from the stream until the device is
// asserted or authentication fails.
//
// As long as any device information is acquired from the stream, a non-nil
// device is returned, even if the ceremony itself failed.
func (c *Ceremony) AssertDevice(ctx context.Context, stream AssertDeviceServerStream) (*devicepb.Device, error) {
	if c.impl == nil {
		return nil, trace.BadParameter("assert.Ceremony instance not properly initialized")
	}

	dev, err := c.impl.AuthenticateDevice(ctx, &streamAdapter{
		logger: c.logger,
		ctx:    ctx,
		stream: stream,
	}, "" /* user */)
	return dev, trace.Wrap(err)
}

type streamAdapter struct {
	logger *slog.Logger
	ctx    context.Context // Typically part of the stream.
	stream AssertDeviceServerStream
}

func (s *streamAdapter) Recv() (*devicepb.AuthenticateDeviceRequest, error) {
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

func (s *streamAdapter) Send(authnResp *devicepb.AuthenticateDeviceResponse) error {
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
