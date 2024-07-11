package storage

import (
	"encoding/json"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

func init() {
	// Set the UnmarshalDeviceFromBackendItemConv function to the deviceFromBackendItem function.
	// This function is used to convert a backend.Item to a *devicepb.Device.
	// It's needed because the storage package uses an internal representation of devicepb.Device
	// when storing it in the backend.
	services.SetUnmarshalDeviceFromBackendItemConv(deviceFromBackendItem)
}

func deviceFromBackendItem(item backend.Item) (*devicepb.Device, error) {
	deviceID := deviceIDFromKey(item.Key)

	stored := &storedDevice{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return nil, trace.Wrap(err, "unmarshal device")
	}
	res := storedToDevice(deviceID, stored)
	return res, nil
}
