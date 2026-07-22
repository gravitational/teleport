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
