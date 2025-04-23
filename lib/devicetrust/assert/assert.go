// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package assert

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	authntypes "github.com/gravitational/teleport/lib/devicetrust/authn/types"
)

// AssertDeviceClientStream is the client-side device assertion stream.
type AssertDeviceClientStream interface {
	Send(*devicepb.AssertDeviceRequest) error
	Recv() (*devicepb.AssertDeviceResponse, error)
}

// Ceremony implements the client-side assertion ceremony.
//
// See [devicepb.AssertDeviceRequest] for details.
type Ceremony struct {
	// newAuthnCeremony defaults to authn.NewCeremony.
	newAuthnCeremony func() *authn.Ceremony
}

// CeremonyOpt is a creation option for [Ceremony].
type CeremonyOpt func(*Ceremony)

// WithNewAuthnCeremonyFunc overrides the default authn.Ceremony constructor,
// allowing callers to change the underlying assert implementation.
//
// Useful for testing. Avoid for production code.
func WithNewAuthnCeremonyFunc(f func() *authn.Ceremony) CeremonyOpt {
	return func(c *Ceremony) {
		c.newAuthnCeremony = f
	}
}

// NewCeremony creates a new [Ceremony], binding all native device trust
// methods.
func NewCeremony(opts ...CeremonyOpt) (*Ceremony, error) {
	c := &Ceremony{
		newAuthnCeremony: authn.NewCeremony,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Run runs the client-side device assertion ceremony.
// Requests and responses are consumed from the stream until the device is
// asserted or authentication fails.
func (c *Ceremony) Run(ctx context.Context, stream AssertDeviceClientStream) error {
	newAuthn := c.newAuthnCeremony
	if newAuthn == nil {
		newAuthn = authn.NewCeremony
	}

	devices := &devicesClientAdapter{
		stream: stream,
	}

	// Implement Assertion in terms of Authentication, so we borrow both the
	// Secure Enclave and TPM branches from it.
	// TODO(codingllama): Refactor so we don't need so many adapters?
	_, err := newAuthn().Run(ctx, &authntypes.CeremonyRunParams{
		DevicesClient: devices,
		Certs:         &devicepb.UserCertificates{}, // required but not used.
	})
	return trace.Wrap(err)
}

type devicesClientAdapter struct {
	devicepb.DeviceTrustServiceClient

	stream AssertDeviceClientStream
}

func (d *devicesClientAdapter) AuthenticateDevice(ctx context.Context, opts ...grpc.CallOption) (devicepb.DeviceTrustService_AuthenticateDeviceClient, error) {
	return &authnStreamAdapter{
		ctx:    ctx,
		stream: d.stream,
	}, nil
}

// authnStreamAdapter adapts an [AssertDeviceClientStream] to a
// [devicepb.DeviceTrustService_AuthenticateDeviceClient] stream. This allows
// the assertion ceremony to borrow the [authn.Ceremony] logic for itself.
type authnStreamAdapter struct {
	devicepb.DeviceTrustService_AuthenticateDeviceClient

	ctx    context.Context
	stream AssertDeviceClientStream
}

func (s *authnStreamAdapter) CloseSend() error {
	return nil
}

func (s *authnStreamAdapter) Context() context.Context {
	return s.ctx
}

func (s *authnStreamAdapter) Recv() (*devicepb.AuthenticateDeviceResponse, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp == nil || resp.Payload == nil {
		return nil, trace.BadParameter("assertion response payload required")
	}

	switch resp.Payload.(type) {
	case *devicepb.AssertDeviceResponse_Challenge:
		return &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_Challenge{
				Challenge: resp.GetChallenge(),
			},
		}, nil
	case *devicepb.AssertDeviceResponse_TpmChallenge:
		return &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_TpmChallenge{
				TpmChallenge: resp.GetTpmChallenge(),
			},
		}, nil
	case *devicepb.AssertDeviceResponse_DeviceAsserted:
		// Pass an empty UserCertificates to signify success.
		return &devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
				UserCertificates: &devicepb.UserCertificates{},
			},
		}, nil
	default:
		return nil, trace.BadParameter("unexpected assertion response payload: %T", resp.Payload)
	}
}

func (s *authnStreamAdapter) Send(authnReq *devicepb.AuthenticateDeviceRequest) error {
	if authnReq == nil || authnReq.Payload == nil {
		return trace.BadParameter("authenticate request payload required")
	}

	req := &devicepb.AssertDeviceRequest{}
	switch authnReq.Payload.(type) {
	case *devicepb.AuthenticateDeviceRequest_Init:
		init := authnReq.GetInit()
		req.Payload = &devicepb.AssertDeviceRequest_Init{
			Init: &devicepb.AssertDeviceInit{
				CredentialId: init.GetCredentialId(),
				DeviceData:   init.GetDeviceData(),
			},
		}
	case *devicepb.AuthenticateDeviceRequest_ChallengeResponse:
		req.Payload = &devicepb.AssertDeviceRequest_ChallengeResponse{
			ChallengeResponse: authnReq.GetChallengeResponse(),
		}
	case *devicepb.AuthenticateDeviceRequest_TpmChallengeResponse:
		req.Payload = &devicepb.AssertDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: authnReq.GetTpmChallengeResponse(),
		}
	default:
		return trace.BadParameter("unexpected authenticate request payload: %T", authnReq.Payload)
	}

	return trace.Wrap(s.stream.Send(req))
}
