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
	"errors"
	"os"
	"strconv"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// ErrAutoEnrollDisabled signifies that auto-enroll is disabled in the current
// device.
// Setting the TELEPORT_DEVICE_AUTO_ENROLL_DISABLED=1 environment disables
// auto-enroll.
var ErrAutoEnrollDisabled = errors.New("auto-enroll disabled")

// AutoEnrollCeremony is the auto-enrollment version of [Ceremony].
type AutoEnrollCeremony struct {
	*Ceremony
}

// NewAutoEnrollCeremony creates a new [AutoEnrollCeremony] based on the regular
// ceremony provided by [NewCeremony].
func NewAutoEnrollCeremony() *AutoEnrollCeremony {
	return &AutoEnrollCeremony{
		Ceremony: NewCeremony(),
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
	const autoEnrollDisabledKey = "TELEPORT_DEVICE_AUTO_ENROLL_DISABLED"
	if disabled, _ := strconv.ParseBool(os.Getenv(autoEnrollDisabledKey)); disabled {
		return nil, trace.Wrap(ErrAutoEnrollDisabled)
	}

	// Creating the init message straight away aborts the process cleanly if the
	// device cannot create the device key (for example, if it lacks a TPM).
	// This avoids a situation where we ask for escalation, like a sudo prompt or
	// admin credentials, then fail a few steps after the prompt.
	init, err := c.EnrollDeviceInit()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := devicesClient.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
		DeviceData: init.DeviceData,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating auto-token")
	}
	init.Token = token.Token

	dev, err := c.run(ctx, devicesClient, false /* debug */, init)
	return dev, trace.Wrap(err)
}
