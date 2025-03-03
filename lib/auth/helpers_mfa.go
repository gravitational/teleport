/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// TestDevice is a test MFA device.
type TestDevice struct {
	MFA        *types.MFADevice
	TOTPSecret string
	Key        *mocku2f.Key

	clock        clockwork.Clock
	origin       string
	passwordless bool
}

// TestDeviceOpt is a creation option for TestDevice.
type TestDeviceOpt func(d *TestDevice)

func WithTestDeviceClock(clock clockwork.Clock) TestDeviceOpt {
	return func(d *TestDevice) {
		d.clock = clock
	}
}

func WithPasswordless() TestDeviceOpt {
	return func(d *TestDevice) {
		d.passwordless = true
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
	ctx context.Context, clt authClientI, devName string, devType proto.DeviceType, authenticator *TestDevice, opts ...TestDeviceOpt,
) (*TestDevice, error) {
	dev := &TestDevice{} // Remaining parameters set during registration
	for _, opt := range opts {
		opt(dev)
	}
	if devType == proto.DeviceType_DEVICE_TYPE_TOTP && dev.clock == nil {
		return nil, trace.BadParameter("TOTP devices require the WithTestDeviceClock option")
	}
	return dev, dev.registerDevice(ctx, clt, devName, devType, authenticator)
}

func (d *TestDevice) Origin() string {
	if d.origin == "" {
		return "https://localhost"
	}
	return d.origin
}

type authClientI interface {
	CreateAuthenticateChallenge(context.Context, *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error)
	CreateRegisterChallenge(context.Context, *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error)
	AddMFADeviceSync(context.Context, *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error)
}

func (d *TestDevice) registerDevice(ctx context.Context, authClient authClientI, devName string, devType proto.DeviceType, authenticator *TestDevice) error {
	mfaCeremony := &mfa.Ceremony{
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				return authenticator.SolveAuthn(chal)
			})
		},
		CreateAuthenticateChallenge: authClient.CreateAuthenticateChallenge,
	}

	authnSolved, err := mfaCeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Acquire and solve registration challenge.
	usage := proto.DeviceUsage_DEVICE_USAGE_MFA
	if d.passwordless {
		usage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	}
	registerChal, err := authClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: authnSolved,
		DeviceType:          devType,
		DeviceUsage:         usage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	registerSolved, err := d.solveRegister(registerChal)
	if err != nil {
		return trace.Wrap(err)
	}

	// Register.
	addResp, err := authClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		NewDeviceName:  devName,
		NewMFAResponse: registerSolved,
		DeviceUsage:    usage,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	d.MFA = addResp.Device
	return nil
}

func (d *TestDevice) SolveAuthn(c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	switch {
	case c.TOTP == nil && c.WebauthnChallenge == nil:
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
	if c.WebauthnChallenge == nil {
		return nil, trace.BadParameter("key-based challenge not present")
	}
	resp, err := d.Key.SignAssertion(d.Origin(), wantypes.CredentialAssertionFromProto(c.WebauthnChallenge))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
		},
	}, nil
}

func (d *TestDevice) solveAuthnTOTP(c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if c.TOTP == nil {
		return nil, trace.BadParameter("TOTP challenge not present")
	}

	if d.clock == nil {
		return nil, trace.BadParameter("clock not set")
	}
	clocki.Advance(d.clock, 30*time.Second)

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

	if d.passwordless {
		d.Key.SetPasswordless()
	}

	resp, err := d.Key.SignCredentialCreation(d.Origin(), wantypes.CredentialCreationFromProto(c.GetWebauthn()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(resp),
		},
	}, nil
}

func (d *TestDevice) solveRegisterTOTP(c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	if d.clock == nil {
		return nil, trace.BadParameter("clock not set")
	}
	clocki.Advance(d.clock, 30*time.Second)

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
				ID:   c.GetTOTP().ID,
			},
		},
	}, nil
}
