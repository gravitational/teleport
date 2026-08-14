package ui

import (
	"time"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// Device represents a devicepb.Device in the Web UI.
type Device struct {
	ID           string        `json:"id,omitempty"`
	AssetTag     string        `json:"assetTag,omitempty"`
	OSType       string        `json:"osType,omitempty"`
	EnrollStatus string        `json:"enrollStatus,omitempty"`
	Owner        string        `json:"owner,omitempty"`
	CreateTime   time.Time     `json:"createTime"`
	Source       *DeviceSource `json:"source,omitempty"`
}

// DeviceSource is a serialized form of [devicepb.DeviceSource].
type DeviceSource struct {
	Name   string                `json:"name"`
	Origin devicepb.DeviceOrigin `json:"origin"`
}

// ListDevicesResponse is similar to types.ListResourcesResponse
type ListDevicesResponse struct {
	// Items is a list of devices retrieved.
	Items []Device `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey,omitempty"`
}
