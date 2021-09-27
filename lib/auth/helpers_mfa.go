/*
Copyright 2017-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TestDevice is a test MFA device.
type TestDevice struct {
	MFA        *types.MFADevice
	TOTPSecret string
	Key        *mocku2f.Key

	clock  clockwork.Clock
	origin string
}

// TestDeviceOpt is a creation option for TestDevice.
type TestDeviceOpt func(d *TestDevice)

func WithTestDeviceClock(clock clockwork.Clock) TestDeviceOpt {
	return func(d *TestDevice) {
		d.clock = clock
	}
}

func WithTestDeviceOrigin(origin string) TestDeviceOpt {
	return func(d *TestDevice) {
		d.origin = origin
	}
}

func NewTestDeviceFromChallenge(c *proto.MFARegisterChallenge, opts ...TestDeviceOpt) (*TestDevice, *proto.MFARegisterResponse, error) {
	dev := &TestDevice{}
	for _, opt := range opts {
		opt(dev)
	}

	registerRes, err := dev.solveRegister(c)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return dev, registerRes, nil
}

// RegisterTestDevice creates and registers a TestDevice.
// TOTP devices require a clock option.
func RegisterTestDevice(
	ctx context.Context, clt authClient, devName string, devType proto.DeviceType, authenticator *TestDevice, opts ...TestDeviceOpt) (*TestDevice, error) {
	dev := &TestDevice{} // Remaining parameters set during registration
	for _, opt := range opts {
		opt(dev)
	}
	if devType == proto.DeviceType_DEVICE_TYPE_TOTP && dev.clock == nil {
		return nil, trace.BadParameter("TOTP devices require the WithTestDeviceClock option")
	}
	return dev, dev.registerStream(ctx, clt, devName, devType, authenticator)
}

func (d *TestDevice) Origin() string {
	if d.origin == "" {
		return "https://localhost"
	}
	return d.origin
}

type authClient interface {
	AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error)
}

func (d *TestDevice) registerStream(
	ctx context.Context, clt authClient, devName string, devType proto.DeviceType, authenticator *TestDevice) error {
	stream, err := clt.AddMFADevice(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Inform device name and type.
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_Init{
			Init: &proto.AddMFADeviceRequestInit{
				DeviceName: devName,
				DeviceType: devType,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	// Solve authn challenge.
	resp, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	authResp, err := authenticator.SolveAuthn(resp.GetExistingMFAChallenge())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_ExistingMFAResponse{
			ExistingMFAResponse: authResp,
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	// Solve register challenge.
	resp, err = stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	registerResp, err := d.solveRegister(resp.GetNewMFARegisterChallenge())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{
			NewMFARegisterResponse: registerResp,
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	// Receive Ack.
	resp, err = stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	if resp.GetAck() == nil {
		return trace.BadParameter("expected ack, got %T", resp.Response)
	}
	d.MFA = resp.GetAck().GetDevice()
	return nil
}

func (d *TestDevice) SolveAuthn(c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	switch {
	case c.TOTP == nil && len(c.U2F) == 0 && c.WebauthnChallenge == nil:
		return &proto.MFAAuthenticateResponse{}, nil // no challenge
	case d.Key != nil:
		return d.solveAuthnKey(c)
	case d.TOTPSecret != "":
		return d.solveAuthnTOTP(c)
	default:
		return nil, trace.BadParameter("TestDevice has neither TOTPSecret or Key")
	}
}

func (d *TestDevice) solveAuthnKey(c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	switch {
	case c.WebauthnChallenge != nil:
		resp, err := d.Key.SignAssertion(d.Origin(), wanlib.CredentialAssertionFromProto(c.WebauthnChallenge))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
			},
		}, nil
	case len(c.U2F) > 0:
		// TODO(codingllama): Find correct challenge according to Key Handle.
		resp, err := d.Key.SignResponse(&u2f.AuthenticateChallenge{
			Version:   c.U2F[0].Version,
			Challenge: c.U2F[0].Challenge,
			KeyHandle: c.U2F[0].KeyHandle,
			AppID:     c.U2F[0].AppID,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_U2F{
				U2F: &proto.U2FResponse{
					KeyHandle:  resp.KeyHandle,
					ClientData: resp.ClientData,
					Signature:  resp.SignatureData,
				},
			},
		}, nil
	}
	return nil, trace.BadParameter("key-based challenge not present")
}

func (d *TestDevice) solveAuthnTOTP(c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if c.TOTP == nil {
		return nil, trace.BadParameter("TOTP challenge not present")
	}

	if d.clock == nil {
		return nil, trace.BadParameter("clock not set")
	}
	if c, ok := d.clock.(clockwork.FakeClock); ok {
		c.Advance(30 * time.Second)
	}
	code, err := totp.GenerateCode(d.TOTPSecret, d.clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: code,
			},
		},
	}, nil
}

func (d *TestDevice) solveRegister(c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	switch {
	case c.GetWebauthn() != nil:
		return d.solveRegisterWebauthn(c)
	case c.GetU2F() != nil:
		return d.solveRegisterU2F(c)
	case c.GetTOTP() != nil:
		return d.solveRegisterTOTP(c)
	default:
		return nil, trace.BadParameter("unexpected challenge type: %T", c.Request)
	}

}

func (d *TestDevice) solveRegisterWebauthn(c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	var err error
	d.Key, err = mocku2f.Create()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	d.Key.PreferRPID = true

	resp, err := d.Key.SignCredentialCreation(d.Origin(), wanlib.CredentialCreationFromProto(c.GetWebauthn()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wanlib.CredentialCreationResponseToProto(resp),
		},
	}, nil
}

func (d *TestDevice) solveRegisterU2F(c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	var err error
	d.Key, err = mocku2f.Create()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := d.Key.RegisterResponse(&u2f.RegisterChallenge{
		Version:   c.GetU2F().Version,
		Challenge: c.GetU2F().Challenge,
		AppID:     c.GetU2F().AppID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_U2F{
			U2F: &proto.U2FRegisterResponse{
				RegistrationData: resp.RegistrationData,
				ClientData:       resp.ClientData,
			},
		},
	}, nil
}

func (d *TestDevice) solveRegisterTOTP(c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	if d.clock == nil {
		return nil, trace.BadParameter("clock not set")
	}
	if c, ok := d.clock.(clockwork.FakeClock); ok {
		c.Advance(30 * time.Second)
	}

	if c.GetTOTP().Algorithm != otp.AlgorithmSHA1.String() {
		return nil, trace.BadParameter("unexpected TOTP challenge algorithm: %s", c.GetTOTP().Algorithm)
	}

	d.TOTPSecret = c.GetTOTP().Secret
	code, err := totp.GenerateCodeCustom(d.TOTPSecret, d.clock.Now(), totp.ValidateOpts{
		Period:    uint(c.GetTOTP().PeriodSeconds),
		Digits:    otp.Digits(c.GetTOTP().Digits),
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{
				Code: code,
			},
		},
	}, nil
}
