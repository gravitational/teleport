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

package enroll

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// AutoEnrollCeremony is the auto-enrollment version of [Ceremony].
type AutoEnrollCeremony struct {
	*Ceremony

	CollectDeviceData func() (*devicepb.DeviceCollectedData, error)
}

// NewAutoEnrollCeremony creates a new [AutoEnrollCeremony] based on the regular
// ceremony provided by [NewCeremony].
func NewAutoEnrollCeremony() *AutoEnrollCeremony {
	return &AutoEnrollCeremony{
		Ceremony:          NewCeremony(),
		CollectDeviceData: native.CollectDeviceData,
	}
}

// AutoEnroll performs auto-enrollment for the current device.
// Equivalent to `NewAutoEnroll().Run()`.
func AutoEnroll(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient) (*devicepb.Device, error) {
	return NewAutoEnrollCeremony().Run(ctx, devicesClient)
}

// Run attempts to create an auto-enroll token via
// [devicepb.DeviceTrustServiceClient.CreateDeviceEnrollToken] and enrolls the
// device using a regular [Ceremony].
func (c *AutoEnrollCeremony) Run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient) (*devicepb.Device, error) {
	cd, err := c.CollectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err, "collecting device data")
	}

	token, err := devicesClient.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
		DeviceData: cd,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating auto-token")
	}

	dev, err := c.Ceremony.Run(ctx, devicesClient, false, token.Token)
	return dev, trace.Wrap(err)
}
