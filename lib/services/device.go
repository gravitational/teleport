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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalDevice unmarshals a DeviceV1 resource and runs CheckAndSetDefaults.
func UnmarshalDevice(raw []byte) (*types.DeviceV1, error) {
	dev := &types.DeviceV1{}
	if err := utils.FastUnmarshal(raw, dev); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, trace.Wrap(dev.CheckAndSetDefaults())
}

// MarshalDevice marshals a DeviceV1 resource.
func MarshalDevice(dev *types.DeviceV1) ([]byte, error) {
	devBytes, err := utils.FastMarshal(dev)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return devBytes, nil
}
