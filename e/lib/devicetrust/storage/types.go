package storage

import "time"

// storedDeviceCredential represents a devicepb.DeviceCredential in storage.
type storedDeviceCredential struct {
	ID           string `json:"id"`             // Required.
	PublicKeyDER []byte `json:"public_key_der"` // Required.
}

// storedDevice (partially) represents a devicepb.Device in storage.
// Written under "device/id/<ID>" keys.
type storedDevice struct {
	OSType       int                     `json:"os_type"`                 // Required. Same as devicepb.OSType.
	AssetTag     string                  `json:"asset_tag"`               // Required.
	CreateTime   time.Time               `json:"create_time"`             // Required.
	UpdateTime   time.Time               `json:"update_time"`             // Required.
	EnrollStatus int                     `json:"enroll_status,omitempty"` // Same as devicepb.EnrollStatus.
	Credential   *storedDeviceCredential `json:"credential,omitempty"`
}

// deviceRef is stored as reference to a device in manually managed indexes.
type deviceRef struct {
	DeviceID string `json:"device_id"` // Required.
	OSType   int    `json:"os_type"`   // Required. Same as devicepb.OSType.
}

// devicesRef is a stored reference to N devices, used by manually managed
// indexes.
type devicesRef struct {
	Devices []*deviceRef `json:"devices,omitempty"`
}
