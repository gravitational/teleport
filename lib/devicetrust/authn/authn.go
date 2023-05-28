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

package authn

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// vars below are used to swap native methods for fakes in tests.
var (
	getDeviceCredential         = native.GetDeviceCredential
	collectDeviceData           = native.CollectDeviceData
	signChallenge               = native.SignChallenge
	solveTPMAuthDeviceChallenge = native.SolveTPMAuthDeviceChallenge
	getDeviceOSType             = native.GetDeviceOSType
)

// RunCeremony performs the client-side device authentication ceremony.
//
// Device authentication requires a previously registered and enrolled device
// (see the lib/devicetrust/enroll package).
//
// The outcome of the authentication ceremony is a pair of user certificates
// augmented with device extensions.
func RunCeremony(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, certs *devicepb.UserCertificates) (*devicepb.UserCertificates, error) {
	switch {
	case devicesClient == nil:
		return nil, trace.BadParameter("devicesClient required")
	case certs == nil:
		return nil, trace.BadParameter("certs required")
	}
	// Start by checking the OSType, this lets us exit early with a nicer message
	// for unsupported OSes.
	osType := getDeviceOSType()
	if !slices.Contains([]devicepb.OSType{
		devicepb.OSType_OS_TYPE_MACOS,
		devicepb.OSType_OS_TYPE_WINDOWS,
	}, osType) {
		return nil, trace.BadParameter(
			"device authentication not supported for current OS (%s)",
			types.ResourceOSTypeToString(osType),
		)
	}

	stream, err := devicesClient.AuthenticateDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}

	// 1. Init.
	cred, err := getDeviceCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cd, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
	switch osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = authenticateDeviceMacOS(stream, resp)
		// err handled below
	case devicepb.OSType_OS_TYPE_WINDOWS:
		err = authenticateDeviceWindows(stream, resp)
		// err handled below
	default:
		// This should be caught by the OSType guard at start of function.
		panic("no authentication function provided for os")
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

func authenticateDeviceMacOS(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
) error {
	chalResp := resp.GetChallenge()
	if chalResp == nil {
		return trace.BadParameter("unexpected payload from server, expected AuthenticateDeviceChallenge: %T", resp.Payload)
	}
	sig, err := signChallenge(chalResp.Challenge)
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

func authenticateDeviceWindows(
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	resp *devicepb.AuthenticateDeviceResponse,
) error {
	challenge := resp.GetTpmChallenge()
	if challenge == nil {
		return trace.BadParameter("unexpected payload from server, expected TPMAuthenticateDeviceChallenge: %T", resp.Payload)
	}
	challengeResponse, err := solveTPMAuthDeviceChallenge(challenge)
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
