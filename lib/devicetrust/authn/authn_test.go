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

package authn_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestCeremony_Run(t *testing.T) {
	t.Parallel()

	env := testenv.MustNew(
		testenv.WithAutoCreateDevice(true),
	)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a fake device and enroll it, so the fake server has the necessary
	// data to verify challenge signatures.
	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	linuxDev1 := testenv.NewFakeLinuxDevice()
	windowsDev1 := testenv.NewFakeWindowsDevice()

	// Enroll all fake devices.
	for _, dev := range []testenv.FakeDevice{
		macOSDev1,
		linuxDev1,
		windowsDev1,
	} {
		_, err := enrollDevice(ctx, devices, dev)
		require.NoError(t, err, "EnrollDevice failed")
	}

	tests := []struct {
		name  string
		dev   testenv.FakeDevice
		certs *devicepb.UserCertificates
	}{
		{
			name: "macOS ok",
			dev:  macOSDev1,
			certs: &devicepb.UserCertificates{
				// SshAuthorizedKey is not parsed by the fake server.
				SshAuthorizedKey: []byte("<a proper SSH certificate goes here>"),
			},
		},
		{
			name: "linux ok",
			dev:  linuxDev1,
			certs: &devicepb.UserCertificates{
				// SshAuthorizedKey is not parsed by the fake server.
				SshAuthorizedKey: []byte("<a proper SSH certificate goes here>"),
			},
		},
		{
			name: "windows ok",
			dev:  windowsDev1,
			certs: &devicepb.UserCertificates{
				// SshAuthorizedKey is not parsed by the fake server.
				SshAuthorizedKey: []byte("<a proper SSH certificate goes here>"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err = newAuthnCeremony(test.dev).Run(ctx, devices, test.certs)

			// A nil error is good enough for this test.
			assert.NoError(t, err, "RunCeremony failed")
		})
	}
}

func TestCeremony_RunWeb(t *testing.T) {
	t.Parallel()

	env := testenv.MustNew(
		testenv.WithAutoCreateDevice(true),
	)
	t.Cleanup(func() { env.Close() })

	devicesClient := env.DevicesClient
	fakeService := env.Service
	ctx := context.Background()

	macOSFakeDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	// Enroll the fake device before authentication.
	macOSDev1, err := enrollDevice(ctx, devicesClient, macOSFakeDev1)
	require.NoError(t, err, "EnrollDevice failed")

	runError := func(t *testing.T, wantErr string, dev testenv.FakeDevice, webToken *devicepb.DeviceWebToken) {
		t.Helper()

		_, err := newAuthnCeremony(dev).RunWeb(ctx, devicesClient, webToken)
		assert.ErrorContains(t, err, wantErr, "RunWeb expected to fail")
	}

	// Verify that fake validations are working.
	t.Run("sanity checks", func(t *testing.T) {
		t.Parallel()

		devID := macOSDev1.Id
		dev := macOSFakeDev1

		webToken1, err := fakeService.CreateDeviceWebTokenForTesting(testenv.CreateDeviceWebTokenParams{
			ExpectedDeviceID: devID,
			WebSessionID:     "my-web-session-1",
		})
		require.NoError(t, err, "CreateDeviceWebTokenForTesting failed")

		invalidDeviceToken, err := fakeService.CreateDeviceWebTokenForTesting(testenv.CreateDeviceWebTokenParams{
			ExpectedDeviceID: "I'm a llama not a device ID",
			WebSessionID:     "my-web-session-2",
		})
		require.NoError(t, err, "CreateDeviceWebTokenForTesting failed")

		const wantErr = "invalid device web token"

		// Unknown token fails.
		runError(t, wantErr, dev, &devicepb.DeviceWebToken{
			Id:    "I'm a llama not a token",
			Token: webToken1.Token,
		})

		// Incorrect token fails (spends webToken1 regardless).
		runError(t, wantErr, dev, &devicepb.DeviceWebToken{
			Id:    webToken1.Id,
			Token: webToken1.Token + "BAD",
		})

		// Spent token fails (spent above).
		runError(t, wantErr, dev, webToken1)

		// Mismatched device fails.
		runError(t, wantErr, dev, invalidDeviceToken)
	})

	t.Run("ok", func(t *testing.T) {
		devID := macOSDev1.Id
		dev := macOSFakeDev1

		// Create a fake DeviceWebToken. This and a previous enrollment is all the
		// fake service requires.
		webToken, err := fakeService.CreateDeviceWebTokenForTesting(testenv.CreateDeviceWebTokenParams{
			ExpectedDeviceID: devID,
			WebSessionID:     "my-web-session-ok",
		})
		require.NoError(t, err, "CreateDeviceWebTokenForTesting failed")

		confirmToken, err := newAuthnCeremony(dev).RunWeb(ctx, devicesClient, webToken)
		require.NoError(t, err, "RunWeb failed")
		assert.NoError(t, fakeService.VerifyConfirmationToken(confirmToken))
	})
}

func newAuthnCeremony(dev testenv.FakeDevice) *authn.Ceremony {
	return &authn.Ceremony{
		GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
			return dev.GetDeviceCredential(), nil
		},
		CollectDeviceData:            dev.CollectDeviceData,
		SignChallenge:                dev.SignChallenge,
		SolveTPMAuthnDeviceChallenge: dev.SolveTPMAuthnDeviceChallenge,
		GetDeviceOSType:              dev.GetDeviceOSType,
	}
}

func enrollDevice(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev testenv.FakeDevice) (*devicepb.Device, error) {
	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.CloseSend()

	// 1. Init.
	enrollDeviceInit, err := dev.EnrollDeviceInit()
	if err != nil {
		return nil, fmt.Errorf("enroll device init: %w", err)
	}
	enrollDeviceInit.Token = testenv.FakeEnrollmentToken
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: enrollDeviceInit,
		},
	}); err != nil {
		return nil, err
	}

	// 2. Challenge.
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("challenge Recv: %w", err)
	}
	switch osType := dev.GetDeviceOSType(); osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		sig, err := dev.SignChallenge(resp.GetMacosChallenge().Challenge)
		if err != nil {
			return nil, err
		}
		if err := stream.Send(&devicepb.EnrollDeviceRequest{
			Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
				MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
					Signature: sig,
				},
			},
		}); err != nil {
			return nil, err
		}
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		solution, err := dev.SolveTPMEnrollChallenge(resp.GetTpmChallenge(), false /* debug */)
		if err != nil {
			return nil, err
		}
		if err := stream.Send(&devicepb.EnrollDeviceRequest{
			Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
				TpmChallengeResponse: solution,
			},
		}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized device os type %q", osType)
	}

	// 3. Success.
	resp, err = stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("challenge response Recv: %w", err)
	}
	success := resp.GetSuccess()
	if success == nil {
		return nil, fmt.Errorf("success response is nil, got %T instead", resp.Payload)
	}
	return success.Device, nil
}
