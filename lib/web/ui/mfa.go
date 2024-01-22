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

package ui

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// MFADevice describes a mfa device
type MFADevice struct {
	// ID is the device ID.
	ID string `json:"id"`
	// Name is the device name.
	Name string `json:"name"`
	// Type is the device type.
	Type string `json:"type"`
	// LastUsed is the time the user used the device last.
	LastUsed time.Time `json:"lastUsed"`
	// AddedAt is the time the user registered the device.
	AddedAt time.Time `json:"addedAt"`
	// ResidentKey is true if the device is supports passwordless authentication.
	// This field is set only for Webauthn devices.
	ResidentKey bool `json:"residentKey"`
}

// MakeMFADevices creates a UI list of mfa devices.
func MakeMFADevices(devices []*types.MFADevice) []MFADevice {
	uiList := make([]MFADevice, 0, len(devices))

	for _, device := range devices {
		uiDevice := MFADevice{
			ID:       device.Id,
			Name:     device.GetName(),
			Type:     device.MFAType(),
			LastUsed: device.LastUsed,
			AddedAt:  device.AddedAt,
		}
		if wad := device.GetWebauthn(); wad != nil {
			uiDevice.ResidentKey = wad.ResidentKey
		}
		uiList = append(uiList, uiDevice)
	}

	return uiList
}
