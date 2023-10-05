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

package enroll_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestCeremony_RunAdmin(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

	devices := env.DevicesClient
	fakeService := env.Service
	ctx := context.Background()

	nonExistingDev, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	registeredDev, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	// Create the device corresponding to registeredDev.
	_, err = devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   registeredDev.GetDeviceOSType(),
			AssetTag: registeredDev.SerialNumber,
		},
	})
	require.NoError(t, err, "CreateDevice(registeredDev) failed")

	tests := []struct {
		name                string
		devicesLimitReached bool
		dev                 testenv.FakeDevice
		wantOutcome         enroll.RunAdminOutcome
		wantErr             string
	}{
		{
			name:        "non-existing device",
			dev:         nonExistingDev,
			wantOutcome: enroll.DeviceRegisteredAndEnrolled,
		},
		{
			name:        "registered device",
			dev:         registeredDev,
			wantOutcome: enroll.DeviceEnrolled,
		},
		// https://github.com/gravitational/teleport/issues/31816.
		{
			name:                "non-existing device, enrollment error",
			devicesLimitReached: true,
			dev: func() testenv.FakeDevice {
				dev, err := testenv.NewFakeMacOSDevice()
				require.NoError(t, err, "NewFakeMacOSDevice failed")
				return dev
			}(),
			wantErr:     "device limit",
			wantOutcome: enroll.DeviceRegistered,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.devicesLimitReached {
				fakeService.SetDevicesLimitReached(true)
				defer fakeService.SetDevicesLimitReached(false) // reset
			}

			c := &enroll.Ceremony{
				GetDeviceOSType:         test.dev.GetDeviceOSType,
				EnrollDeviceInit:        test.dev.EnrollDeviceInit,
				SignChallenge:           test.dev.SignChallenge,
				SolveTPMEnrollChallenge: test.dev.SolveTPMEnrollChallenge,
			}

			enrolled, outcome, err := c.RunAdmin(ctx, devices, false /* debug */)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "RunAdmin error mismatch")
			} else {
				assert.NoError(t, err, "RunAdmin failed")
			}
			assert.NotNil(t, enrolled, "RunAdmin returned nil device")
			assert.Equal(t, test.wantOutcome, outcome, "RunAdmin outcome mismatch")
		})
	}
}

func TestCeremony_Run(t *testing.T) {
	env := testenv.MustNew(
		testenv.WithAutoCreateDevice(true),
	)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")
	windowsDev1 := testenv.NewFakeWindowsDevice()

	tests := []struct {
		name            string
		dev             testenv.FakeDevice
		assertErr       func(t *testing.T, err error)
		assertGotDevice func(t *testing.T, device *devicepb.Device)
	}{
		{
			name: "macOS device succeeds",
			dev:  macOSDev1,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err, "RunCeremony returned an error")
			},
			assertGotDevice: func(t *testing.T, d *devicepb.Device) {
				assert.NotNil(t, d, "RunCeremony returned nil device")
			},
		},
		{
			name: "windows device succeeds",
			dev:  windowsDev1,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err, "RunCeremony returned an error")
			},
			assertGotDevice: func(t *testing.T, d *devicepb.Device) {
				require.NotNil(t, d, "RunCeremony returned nil device")
				require.NotNil(t, d.Credential, "device credential is nil")
				assert.Equal(t, windowsDev1.CredentialID, d.Credential.Id, "device credential mismatch")
			},
		},
		{
			name: "linux device fails",
			dev:  testenv.NewFakeLinuxDevice(),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(
					t, trace.IsBadParameter(err), "RunCeremony did not return a BadParameter error",
				)
				assert.ErrorContains(t, err, "linux", "RunCeremony error mismatch")
			},
			assertGotDevice: func(t *testing.T, d *devicepb.Device) {
				assert.Nil(t, d, "RunCeremony returned an unexpected, non-nil device")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &enroll.Ceremony{
				GetDeviceOSType:         test.dev.GetDeviceOSType,
				EnrollDeviceInit:        test.dev.EnrollDeviceInit,
				SignChallenge:           test.dev.SignChallenge,
				SolveTPMEnrollChallenge: test.dev.SolveTPMEnrollChallenge,
			}

			got, err := c.Run(ctx, devices, false /* debug */, testenv.FakeEnrollmentToken)
			test.assertErr(t, err)
			test.assertGotDevice(t, got)
		})
	}
}
