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
	"crypto"
	"errors"
	"io"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	dtauthntypes "github.com/gravitational/teleport/lib/devicetrust/authn/types"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
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
func (c *Ceremony) Run(
	ctx context.Context,
	params *dtauthntypes.CeremonyRunParams,
) (*devicepb.UserCertificates, error) {
	switch {
	case params.DevicesClient == nil:
		return nil, trace.BadParameter("DevicesClient required")
	case params.Certs == nil:
		return nil, trace.BadParameter("Certs required")
	}
	// nil SSHSigner is okay.

	resp, err := c.run(ctx, params.DevicesClient, &devicepb.AuthenticateDeviceInit{
		UserCertificates: &devicepb.UserCertificates{
			// Forward only the SSH certificate, the TLS identity is part of the
			// connection.
			SshAuthorizedKey: params.Certs.SshAuthorizedKey,
		},
	}, params.SSHSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newCerts := resp.GetUserCertificates()
	if newCerts == nil {
		return nil, trace.BadParameter("unexpected payload from server, expected UserCertificates: %T", resp.Payload)
	}

	return newCerts, nil
}

// RunWeb performs on-behalf-of device authentication. It exchanges a webToken
// issued for the Web UI for a device authentication attempt.
//
// On success a [devicepb.DeviceConfirmationToken] is issued. To complete
// authentication the browser that originated the attempt must forward the token
// to the /webapi/device/webconfirm endpoint.
func (c *Ceremony) RunWeb(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	webToken *devicepb.DeviceWebToken,
) (*devicepb.DeviceConfirmationToken, error) {
	switch {
	case devicesClient == nil:
		return nil, trace.BadParameter("devicesClient required")
	case webToken == nil:
		return nil, trace.BadParameter("webToken required")
	}

	// It's not necessary to sign with the SSH key for Device Trust Web, the SSH
	// cert is implicitly trusted when it's taken directly from the web session.
	resp, err := c.run(ctx, devicesClient, &devicepb.AuthenticateDeviceInit{
		DeviceWebToken: webToken,
	}, nil /*sshSigner*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	confirmToken := resp.GetConfirmationToken()
	if confirmToken == nil {
		return nil, trace.BadParameter("unexpected payload from server, expected ConfirmationToken: %T", resp.Payload)
	}

	return confirmToken, trace.Wrap(err)
}

func (c *Ceremony) run(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	init *devicepb.AuthenticateDeviceInit,
	sshSigner crypto.Signer,
) (*devicepb.AuthenticateDeviceResponse, error) {
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
	init.CredentialId = cred.Id
	init.DeviceData = cd
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: init,
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		// [io.EOF] indicates that the server has closed the stream.
		// The client should handle the underlying error on the subsequent Recv call.
		// All other errors are client-side errors and should be returned.
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
		err = c.authenticateDeviceMacOS(stream, resp, sshSigner)
		// err handled below
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		err = c.authenticateDeviceTPM(stream, resp, sshSigner)
		// err handled below
	default:
		// This should be caught by the c.GetDeviceCredential() and
		// c.CollectDeviceData() calls above.
		return nil, devicetrust.ErrPlatformNotSupported
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Success (either UserCertificates or DeviceConfirmationToken).
	resp, err = stream.Recv()
	return resp, trace.Wrap(err)
}

func (c *Ceremony) authenticateDeviceMacOS(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
	sshSigner crypto.Signer,
) error {
	chalResp := resp.GetChallenge()
	if chalResp == nil {
		return trace.BadParameter("unexpected payload from server, expected AuthenticateDeviceChallenge: %T", resp.Payload)
	}
	sig, err := c.SignChallenge(chalResp.Challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	var sshSig []byte
	if sshSigner != nil {
		sshSig, err = challenge.Sign(chalResp.Challenge, sshSigner)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature:    sig,
				SshSignature: sshSig,
			},
		},
	})
	if err != nil && !errors.Is(err, io.EOF) {
		// [io.EOF] indicates that the server has closed the stream.
		// The client should handle the underlying error on the subsequent Recv call.
		// All other errors are client-side errors and should be returned.
		return trace.Wrap(err)
	}
	return nil
}

func (c *Ceremony) authenticateDeviceTPM(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
	sshSigner crypto.Signer,
) error {
	tpmChallenge := resp.GetTpmChallenge()
	if tpmChallenge == nil {
		return trace.BadParameter("unexpected payload from server, expected TPMAuthenticateDeviceChallenge: %T", resp.Payload)
	}
	challengeResponse, err := c.SolveTPMAuthnDeviceChallenge(tpmChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	if sshSigner != nil {
		challengeResponse.SshSignature, err = challenge.Sign(tpmChallenge.AttestationNonce, sshSigner)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: challengeResponse,
		},
	})
	if err != nil && !errors.Is(err, io.EOF) {
		// [io.EOF] indicates that the server has closed the stream.
		// The client should handle the underlying error on the subsequent Recv call.
		// All other errors are client-side errors and should be returned.
		return trace.Wrap(err)
	}
	return nil
}
