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

package authn

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// Ceremony is the device authentication ceremony.
// It takes the client role of
// [devicepb.DeviceTrustServiceClient.AuthenticateDevice]
type Ceremony struct {
	GetDeviceCredential          func() (*devicepb.DeviceCredential, error)
	CollectDeviceData            func(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error)
	SignChallenge                func(chal []byte) (sig []byte, err error)
	SolveTPMAuthnDeviceChallenge func(challenge *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error)
	GetDeviceOSType              func() devicepb.OSType
}

// NewCeremony creates a new ceremony that delegates per-device behavior
// to lib/devicetrust/native.
// If you want to customize a [Ceremony], for example for testing purposes, you
// may create a configure an instance directly, without calling this method.
func NewCeremony() *Ceremony {
	return &Ceremony{
		GetDeviceCredential:          native.GetDeviceCredential,
		CollectDeviceData:            native.CollectDeviceData,
		SignChallenge:                native.SignChallenge,
		SolveTPMAuthnDeviceChallenge: native.SolveTPMAuthnDeviceChallenge,
		GetDeviceOSType:              native.GetDeviceOSType,
	}
}

// Run performs the client-side device authentication ceremony.
//
// Device authentication requires a previously registered and enrolled device
// (see the lib/devicetrust/enroll package).
//
// The outcome of the authentication ceremony is a pair of user certificates
// augmented with device extensions.
func (c *Ceremony) Run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, certs *devicepb.UserCertificates) (*devicepb.UserCertificates, error) {
	switch {
	case devicesClient == nil:
		return nil, trace.BadParameter("devicesClient required")
	case certs == nil:
		return nil, trace.BadParameter("certs required")
	}

	// Fetch device data early, this automatically excludes unsupported platforms
	// and unenrolled devices.
	cred, err := c.GetDeviceCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cd, err := c.CollectDeviceData(native.CollectedDataMaybeEscalate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stream, err := devicesClient.AuthenticateDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	defer stream.CloseSend()

	// 1. Init.
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				UserCertificates: &devicepb.UserCertificates{
					// Forward only the SSH certificate, the TLS identity is part of the
					// connection.
					SshAuthorizedKey: certs.SshAuthorizedKey,
				},
				CredentialId: cred.Id,
				DeviceData:   cd,
			},
		},
	}); err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	// Unimplemented errors are not expected to happen after this point.

	// 2. Challenge.
	switch c.GetDeviceOSType() {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = c.authenticateDeviceMacOS(stream, resp)
		// err handled below
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		err = c.authenticateDeviceTPM(stream, resp)
		// err handled below
	default:
		// This should be caught by the c.GetDeviceCredential() and
		// c.CollectDeviceData() calls above.
		return nil, devicetrust.ErrPlatformNotSupported
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. User certificates.
	newCerts := resp.GetUserCertificates()
	if newCerts == nil {
		return nil, trace.BadParameter("unexpected payload from server, expected UserCertificates: %T", resp.Payload)
	}
	return newCerts, nil
}

func (c *Ceremony) authenticateDeviceMacOS(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
) error {
	chalResp := resp.GetChallenge()
	if chalResp == nil {
		return trace.BadParameter("unexpected payload from server, expected AuthenticateDeviceChallenge: %T", resp.Payload)
	}
	sig, err := c.SignChallenge(chalResp.Challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature: sig,
			},
		},
	})
	return trace.Wrap(err)
}

func (c *Ceremony) authenticateDeviceTPM(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
) error {
	challenge := resp.GetTpmChallenge()
	if challenge == nil {
		return trace.BadParameter("unexpected payload from server, expected TPMAuthenticateDeviceChallenge: %T", resp.Payload)
	}
	challengeResponse, err := c.SolveTPMAuthnDeviceChallenge(challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: challengeResponse,
		},
	})
	return trace.Wrap(err)
}
