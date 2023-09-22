// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	env := testenv.MustNew()
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a fake device and enroll it, so the fake server has the necessary
	// data to verify challenge signatures.
	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")
	require.NoError(t, enrollDevice(ctx, devices, macOSDev1), "enrollDevice failed")

	tests := []struct {
		name  string
		dev   *testenv.FakeMacOSDevice
		certs *devicepb.UserCertificates
	}{
		{
			name: "ok",
			dev:  macOSDev1,
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
					return test.dev.DeviceCredential(), nil
				},
				CollectDeviceData: test.dev.CollectDeviceData,
				SignChallenge:     test.dev.SignChallenge,
			}

			_, err := ceremony.Run(ctx, devices, test.certs)
			// A nil error is good enough for this test.
			assert.NoError(t, err, "RunCeremony failed")
		})
	}
}

func enrollDevice(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev *testenv.FakeMacOSDevice) error {
	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return err
	}

	// 1. Init.
	cd, err := dev.CollectDeviceData()
	if err != nil {
		return err
	}
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: &devicepb.EnrollDeviceInit{
				Token:        "fake enroll token",
				CredentialId: dev.ID,
				DeviceData:   cd,
				Macos: &devicepb.MacOSEnrollPayload{
					PublicKeyDer: dev.PubKeyDER,
				},
			},
		},
	}); err != nil {
		return err
	}

	// 2. Challenge.
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("challenge Recv: %w", err)
	}
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
