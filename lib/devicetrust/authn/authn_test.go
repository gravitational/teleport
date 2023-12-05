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

func TestRunCeremony(t *testing.T) {
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
			ceremony := authn.Ceremony{
				GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
					return test.dev.GetDeviceCredential(), nil
				},
				CollectDeviceData:            test.dev.CollectDeviceData,
				SignChallenge:                test.dev.SignChallenge,
				SolveTPMAuthnDeviceChallenge: test.dev.SolveTPMAuthnDeviceChallenge,
				GetDeviceOSType:              test.dev.GetDeviceOSType,
			}

			// We need to enroll the device before we can test device auth
			require.NoError(
				t,
				enrollDevice(ctx, devices, test.dev),
				"enrollDevice failed",
			)

			_, err := ceremony.Run(ctx, devices, test.certs)
			// A nil error is good enough for this test.
			assert.NoError(t, err, "RunCeremony failed")
		})
	}
}

func enrollDevice(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev testenv.FakeDevice) error {
	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()

	// 1. Init.
	enrollDeviceInit, err := dev.EnrollDeviceInit()
	if err != nil {
		return fmt.Errorf("enroll device init: %w", err)
	}
	enrollDeviceInit.Token = testenv.FakeEnrollmentToken
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: enrollDeviceInit,
		},
	}); err != nil {
		return err
	}

	// 2. Challenge.
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("challenge Recv: %w", err)
	}
	switch osType := dev.GetDeviceOSType(); osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		sig, err := dev.SignChallenge(resp.GetMacosChallenge().Challenge)
		if err != nil {
			return err
		}
		if err := stream.Send(&devicepb.EnrollDeviceRequest{
			Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
				MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
					Signature: sig,
				},
			},
		}); err != nil {
			return err
		}
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		solution, err := dev.SolveTPMEnrollChallenge(resp.GetTpmChallenge(), false /* debug */)
		if err != nil {
			return err
		}
		if err := stream.Send(&devicepb.EnrollDeviceRequest{
			Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
				TpmChallengeResponse: solution,
			},
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized device os type %q", osType)
	}

	// 3. Success.
	resp, err = stream.Recv()
	switch {
	case err != nil:
		return fmt.Errorf("challenge response Recv: %w", err)
	case resp.GetSuccess() == nil:
		return fmt.Errorf("success response is nil, got %T instead", resp.Payload)
	}
	return nil
}
