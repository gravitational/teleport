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
	"github.com/gravitational/trace"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestRunCeremony(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()
	t.Cleanup(resetNative())

	devices := env.DevicesClient
	ctx := context.Background()

	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")
	windowsDev1 := testenv.NewFakeWindowsDevice()

	tests := []struct {
		name            string
		dev             fakeDevice
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
				require.NotNil(t, d.Credential)
				assert.Equal(t, windowsDev1.CredentialID, d.Credential.Id)
			},
		},
		{
			name: "linux device fails",
			dev:  testenv.NewFakeLinuxDevice(),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(
					t, trace.IsBadParameter(err), "RunCeremony did not return an error of the correct type",
				)
				assert.ErrorContains(t, err, "linux")
			},
			assertGotDevice: func(t *testing.T, d *devicepb.Device) {
				assert.Nil(t, d)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			*enroll.GetOSType = test.dev.GetOSType
			*enroll.EnrollInit = test.dev.EnrollDeviceInit
			*enroll.SignChallenge = test.dev.SignChallenge
			*enroll.SolveTPMEnrollChallenge = test.dev.SolveTPMEnrollChallenge

			got, err := enroll.RunCeremony(ctx, devices, "faketoken")
			test.assertErr(t, err)
			test.assertGotDevice(t, got)
		})
	}
}

func resetNative() func() {
	const guardKey = "_dt_reset_native"
	if os.Getenv(guardKey) != "" {
		panic("Tests that rely on resetNative cannot run in parallel.")
	}
	os.Setenv(guardKey, "1")

	collectDeviceData := *enroll.CollectDeviceData
	enrollDeviceInit := *enroll.EnrollInit
	getOSType := *enroll.GetOSType
	signChallenge := *enroll.SignChallenge
	return func() {
		*enroll.CollectDeviceData = collectDeviceData
		*enroll.EnrollInit = enrollDeviceInit
		*enroll.GetOSType = getOSType
		*enroll.SignChallenge = signChallenge
		os.Unsetenv(guardKey)
	}
}

type fakeDevice interface {
	CollectDeviceData() (*devicepb.DeviceCollectedData, error)
	EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error)
	GetOSType() devicepb.OSType
	SignChallenge(chal []byte) (sig []byte, err error)
	SolveTPMEnrollChallenge(challenge *devicepb.TPMEnrollChallenge) (*devicepb.TPMEnrollChallengeResponse, error)
}
