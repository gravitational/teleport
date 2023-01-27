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

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// vars below are used to swap native methods for fakes in tests.
var (
	getDeviceCredential = native.GetDeviceCredential
	collectDeviceData   = native.CollectDeviceData
	signChallenge       = native.SignChallenge
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

	stream, err := devicesClient.AuthenticateDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 2. Challenge.
	chalResp := resp.GetChallenge()
	if chalResp == nil {
		return nil, trace.BadParameter("unexpected payload from server, expected AuthenticateDeviceChallenge: %T", resp.Payload)
	}
	sig, err := signChallenge(chalResp.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil {
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
