// Copyright 2023 Gravitational, Inc
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestAutoEnrollCeremony_Run(t *testing.T) {
	env := testenv.MustNew(
		testenv.WithAutoCreateDevice(true),
	)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	tests := []struct {
		name string
		dev  testenv.FakeDevice
	}{
		{
			name: "macOS device",
			dev:  macOSDev1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := enroll.AutoEnrollCeremony{
				Ceremony: &enroll.Ceremony{
					GetDeviceOSType:         test.dev.GetDeviceOSType,
					EnrollDeviceInit:        test.dev.EnrollDeviceInit,
					SignChallenge:           test.dev.SignChallenge,
					SolveTPMEnrollChallenge: test.dev.SolveTPMEnrollChallenge,
				},
			}

			dev, err := c.Run(ctx, devices)
			require.NoError(t, err, "AutoEnroll failed")
			assert.NotNil(t, dev, "AutoEnroll returned nil device")
		})
	}
}

func TestAutoEnroll_disabledByEnv(t *testing.T) {
	t.Setenv("TELEPORT_DEVICE_AUTO_ENROLL_DISABLED", "1")

	_, err := enroll.AutoEnroll(context.Background(), nil /* devicesClient */)
	assert.ErrorIs(t, err, enroll.ErrAutoEnrollDisabled, "AutoEnroll() error mismatch")
}
