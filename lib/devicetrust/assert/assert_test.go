// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package assert_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestCeremony(t *testing.T) {
	t.Parallel()

	env := testenv.MustNew(
		testenv.WithAutoCreateDevice(true),
	)

	devicesClient := env.DevicesClient
	ctx := context.Background()

	macDev, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice errored")

	linuxDev := testenv.NewFakeLinuxDevice()

	tests := []struct {
		name string
		dev  testenv.FakeDevice
	}{
		{
			name: "ok (macOs)",
			dev:  macDev,
		},
		{
			name: "ok (TPM)",
			dev:  linuxDev,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Enroll the device before we attempt authentication (auto creates device
			// as part of ceremony)
			dev := test.dev
			enrollC := enroll.Ceremony{
				GetDeviceOSType:         dev.GetDeviceOSType,
				EnrollDeviceInit:        dev.EnrollDeviceInit,
				SignChallenge:           dev.SignChallenge,
				SolveTPMEnrollChallenge: dev.SolveTPMEnrollChallenge,
			}
			_, err := enrollC.Run(ctx, devicesClient, false, testenv.FakeEnrollmentToken)
			require.NoError(t, err, "EnrollDevice errored")

			assertC, err := assert.NewCeremony(assert.WithNewAuthnCeremonyFunc(func() *authn.Ceremony {
				return &authn.Ceremony{
					GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
						return dev.GetDeviceCredential(), nil
					},
					CollectDeviceData:            dev.CollectDeviceData,
					SignChallenge:                dev.SignChallenge,
					SolveTPMAuthnDeviceChallenge: dev.SolveTPMAuthnDeviceChallenge,
					GetDeviceOSType:              dev.GetDeviceOSType,
				}
			}))
			require.NoError(t, err, "NewCeremony errored")

			authnStream, err := devicesClient.AuthenticateDevice(ctx)
			require.NoError(t, err, "AuthenticateDevice errored")

			// Typically this would be some other, non-DeviceTrustService stream, but
			// here this is a good way to test it (as it runs actual fake device
			// authn)
			if err := assertC.Run(ctx, &assertStreamAdapter{
				stream: authnStream,
			}); err != nil {
				t.Errorf("Run returned err=%q, want nil", err)
			}
		})
	}
}

type assertStreamAdapter struct {
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient
}

func (s *assertStreamAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	switch resp.Payload.(type) {
	case *devicepb.AuthenticateDeviceResponse_Challenge:
		return &devicepb.AssertDeviceResponse{
			Payload: &devicepb.AssertDeviceResponse_Challenge{
				Challenge: resp.GetChallenge(),
			},
		}, nil
	case *devicepb.AuthenticateDeviceResponse_TpmChallenge:
		return &devicepb.AssertDeviceResponse{
			Payload: &devicepb.AssertDeviceResponse_TpmChallenge{
				TpmChallenge: resp.GetTpmChallenge(),
			},
		}, nil
	case *devicepb.AuthenticateDeviceResponse_UserCertificates:
		// UserCertificates means success.
		return &devicepb.AssertDeviceResponse{
			Payload: &devicepb.AssertDeviceResponse_DeviceAsserted{
				DeviceAsserted: &devicepb.DeviceAsserted{},
			},
		}, nil
	default:
		return nil, trace.BadParameter("unexpected authenticate response payload: %T", resp.Payload)
	}
}

func (s *assertStreamAdapter) Send(req *devicepb.AssertDeviceRequest) error {
	authnReq := &devicepb.AuthenticateDeviceRequest{}
	switch req.Payload.(type) {
	case *devicepb.AssertDeviceRequest_Init:
		init := req.GetInit()
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				CredentialId: init.CredentialId,
				DeviceData:   init.DeviceData,
			},
		}
	case *devicepb.AssertDeviceRequest_ChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: req.GetChallengeResponse(),
		}
	case *devicepb.AssertDeviceRequest_TpmChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: req.GetTpmChallengeResponse(),
		}
	default:
		return trace.BadParameter("unexpected assert request payload: %T", req.Payload)
	}

	return s.stream.Send(authnReq)
}
