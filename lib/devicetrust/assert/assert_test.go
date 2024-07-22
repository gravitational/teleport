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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestCeremony(t *testing.T) {
	t.Parallel()

	deviceID := uuid.NewString()

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

			dev := test.dev

			// Create an enrolled device.
			devpb, pubKey, err := testenv.CreateEnrolledDevice(deviceID, dev)
			require.NoError(t, err, "CreateEnrolledDevice errored")

			env := testenv.MustNew(
				testenv.WithAutoCreateDevice(true),
				// Register the enrolled device with the service.
				testenv.WithPreEnrolledDevice(devpb, pubKey),
			)

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

			clientToServer := make(chan *devicepb.AssertDeviceRequest)
			serverToClient := make(chan *devicepb.AssertDeviceResponse)

			group, ctx := errgroup.WithContext(ctx)

			// Run the client side of the ceremony.
			group.Go(func() error {
				err := assertC.Run(ctx, &assertStreamClientAdapter{
					ctx:            ctx,
					clientToServer: clientToServer,
					serverToClient: serverToClient,
				})
				return trace.Wrap(err, "server AssertDevice errored")
			})

			serverAssertC, err := env.Service.CreateAssertCeremony()
			require.NoError(t, err, "CreateAssertCeremony errored")
			// Run the server side of the ceremony.
			group.Go(func() error {
				_, err := serverAssertC.AssertDevice(ctx, &assertStreamServerAdapter{
					ctx:            ctx,
					clientToServer: clientToServer,
					serverToClient: serverToClient,
				})
				return trace.Wrap(err, "server AssertDevice errored")
			})

			err = group.Wait()
			require.NoError(t, err, "group.Wait errored")
		})
	}
}

type assertStreamClientAdapter struct {
	ctx            context.Context
	clientToServer chan *devicepb.AssertDeviceRequest
	serverToClient chan *devicepb.AssertDeviceResponse
}

func (s *assertStreamClientAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	select {
	case resp := <-s.serverToClient:
		return resp, nil
	case <-s.ctx.Done():
		return nil, trace.Wrap(s.ctx.Err())
	}
}

func (s *assertStreamClientAdapter) Send(req *devicepb.AssertDeviceRequest) error {
	select {
	case s.clientToServer <- req:
		return nil
	case <-s.ctx.Done():
		return trace.Wrap(s.ctx.Err())
	}
}

type assertStreamServerAdapter struct {
	ctx            context.Context
	clientToServer chan *devicepb.AssertDeviceRequest
	serverToClient chan *devicepb.AssertDeviceResponse
}

func (s *assertStreamServerAdapter) Recv() (*devicepb.AssertDeviceRequest, error) {
	select {
	case req := <-s.clientToServer:
		return req, nil
	case <-s.ctx.Done():
		return nil, trace.Wrap(s.ctx.Err())
	}
}

func (s *assertStreamServerAdapter) Send(resp *devicepb.AssertDeviceResponse) error {
	select {
	case s.serverToClient <- resp:
		return nil
	case <-s.ctx.Done():
		return trace.Wrap(s.ctx.Err())
	}
}
