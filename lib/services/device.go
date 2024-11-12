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
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// DevicesGetter allows to list all registered devices from storage.
type DevicesGetter interface {
	ListDevices(ctx context.Context, pageSize int, pageToken string, view devicepb.DeviceView) (devices []*devicepb.Device, nextPageToken string, err error)
}

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

var (
	// unmarshalDeviceFromBackendItemConv is a convenience function that converts
	// a backend.Item to a *devicepb.Device.
	// It's populated when e/lib/devicetrust/storage/ is initialized.
	unmarshalDeviceFromBackendItemConv func(item backend.Item) (*devicepb.Device, error)
)

// SetUnmarshalDeviceFromBackendItemConv allows to set a custom conversion function for
// unmarshaling a [devicepb.Device] from a [backend.Item].
// This function must be called in the init() function of the package that owns the conversion logic.
// It's not safe to call this function concurrently.
func SetUnmarshalDeviceFromBackendItemConv(conv func(item backend.Item) (*devicepb.Device, error)) {
	unmarshalDeviceFromBackendItemConv = conv
}

// UnmarshalDeviceFromBackendItem unmarshals a devicepb.Device from a backend.Item.
// It's a convenience function that uses UnmarshalDeviceFromBackendItemConv because
// the storage package uses an internal representation of devicepb.Device when storing
// it in the backend.
func UnmarshalDeviceFromBackendItem(item backend.Item) (*devicepb.Device, error) {
	if unmarshalDeviceFromBackendItemConv == nil {
		return nil, trace.BadParameter("UnmarshalDeviceFromBackendItemConv is not set")
	}
	res, err := unmarshalDeviceFromBackendItemConv(item)
	return res, trace.Wrap(err)
}
