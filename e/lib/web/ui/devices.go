package ui

import "time"

// Device represents a devicepb.Device in the Web UI.
type Device struct {
	ID           string    `json:"id,omitempty"`
	AssetTag     string    `json:"assetTag,omitempty"`
	OSType       string    `json:"osType,omitempty"`
	EnrollStatus string    `json:"enrollStatus,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	CreateTime   time.Time `json:"createTime,omitempty"`
}

// ListDevicesResponse is similar to types.ListResourcesResponse
type ListDevicesResponse struct {
	// Items is a list of devices retrieved.
	Items []Device `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey,omitempty"`
}
