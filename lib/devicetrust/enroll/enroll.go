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

package enroll

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"runtime"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
)

// RunCeremony performs the client-side device enrollment ceremony.
func RunCeremony(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, enrollToken string) (*devicepb.Device, error) {
	// Start by checking the OSType, this lets us exit early with a nicer message
	// for non-supported OSes.
	osType := getOSType()
	if !slices.Contains([]devicepb.OSType{
		devicepb.OSType_OS_TYPE_MACOS,
		devicepb.OSType_OS_TYPE_WINDOWS,
	}, osType) {
		return nil, trace.BadParameter(
			"device enrollment not supported for current OS (%s)",
			types.ResourceOSTypeToString(osType),
		)
	}

	init, err := enrollInit()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	init.Token = enrollToken

	// 1. Init.
	stream, err := devicesClient.EnrollDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: init,
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
	switch osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		if err := enrollDeviceMacOS(stream, resp); err != nil {
			return nil, trace.Wrap(err)
		}
	case devicepb.OSType_OS_TYPE_WINDOWS:
		if err := enrollDeviceTPM(stream, resp); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		// This should be caught by the OSType guard at start of function.
		panic("no enrollment function provided for os")
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Success.
	successResp := resp.GetSuccess()
	if successResp == nil {
		return nil, trace.BadParameter("unexpected success payload from server: %T", resp.Payload)
	}
	return successResp.Device, nil
}

func enrollDeviceMacOS(stream devicepb.DeviceTrustService_EnrollDeviceClient, resp *devicepb.EnrollDeviceResponse) error {
	chalResp := resp.GetMacosChallenge()
	if chalResp == nil {
		return trace.BadParameter("unexpected challenge payload from server: %T", resp.Payload)
	}
	sig, err := signChallenge(chalResp.Challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	})
	return trace.Wrap(err)
}

func enrollDeviceTPM(stream devicepb.DeviceTrustService_EnrollDeviceClient, resp *devicepb.EnrollDeviceResponse) error {
	challenge := resp.GetTpmChallenge()
	switch {
	case challenge == nil:
		return trace.BadParameter("unexpected challenge payload from server: %T", resp.Payload)
	case challenge.EncryptedCredential == nil:
		return trace.BadParameter("missing encrypted_credential in challenge from server")
	case len(challenge.AttestationNonce) == 0:
		return trace.BadParameter("missing attestation_nonce in challenge from server")
	}

	challengeResponse, err := solveTPMEnrollChallenge(challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: challengeResponse,
		},
	})
	return trace.Wrap(err)
}

func getDeviceOSType() devicepb.OSType {
	switch runtime.GOOS {
	case "darwin":
		return devicepb.OSType_OS_TYPE_MACOS
	case "linux":
		return devicepb.OSType_OS_TYPE_LINUX
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}
}
