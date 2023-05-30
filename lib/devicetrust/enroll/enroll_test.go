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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
)

func TestCeremony_Run(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	macOSDev1, err := testenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	tests := []struct {
		name string
		dev  fakeDevice
	}{
		{
			name: "macOS device",
			dev:  macOSDev1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &enroll.Ceremony{
				GetDeviceOSType:  test.dev.GetOSType,
				EnrollDeviceInit: test.dev.EnrollDeviceInit,
				SignChallenge:    test.dev.SignChallenge,
			}

			got, err := c.Run(ctx, devices, "faketoken")
			assert.NoError(t, err, "RunCeremony failed")
			assert.NotNil(t, got, "RunCeremony returned nil device")
		})
	}
}

type fakeDevice interface {
	GetOSType() devicepb.OSType
	EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error)
	SignChallenge(chal []byte) (sig []byte, err error)
}
