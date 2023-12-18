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

	CollectDeviceData func(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error)
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
	cd, err := c.CollectDeviceData(native.CollectedDataAlwaysEscalate)
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
