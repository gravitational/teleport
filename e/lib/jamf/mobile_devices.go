package jamf

import (
	"time"
)

const (
	// MobileDeviceSectionGeneral is the GENERAL section of a [MobileDevice]
	// instance.
	MobileDeviceSectionGeneral = "GENERAL"
	// MobileDeviceSectionHardware is the HARDWARE section of a [MobileDevice]
	// instance.
	MobileDeviceSectionHardware = "HARDWARE"
)

// GetMobileDevicesDetailRequest is a list request for mobile device inventory.
//
// https://developer.jamf.com/jamf-pro/reference/get_v2-mobile-devices-detail
type GetMobileDevicesDetailRequest struct {
	// Section is the slice of sections to query.
	// If empty, the general section is returned.
	Section []string
	// Page to query, starting from zero.
	Page int
	// PageSize is the length of the returned page.
	// If zero, server defaults are used.
	PageSize int
	// Sort is the sort filter slice to use, in the form "property:asc/desc".
	// See the API docs for supported fields.
	// Example: "lastInventoryUpdateDate:asc".
	Sort []string
	// Filter is the RSQL filter applied to the query.
	// See the API docs for supported fields.
	// Example: `managed==true`.
	Filter string
}

// GetMobileDevicesDetailResponse is the response for
// [GetMobileDevicesDetailRequest].
type GetMobileDevicesDetailResponse struct {
	// TotalCount is the total inventory count, regardless of the requested page.
	TotalCount int             `json:"totalCount"`
	Results    []*MobileDevice `json:"results"`
}

// GetMobileDeviceByIDRequest is a request for a single mobile device.
//
// https://developer.jamf.com/jamf-pro/reference/get_v2-mobile-devices-id-detail
type GetMobileDeviceByIDRequest struct {
	// ID is the mobile device identifier.
	ID string
}

// MobileDevice is a mobile device inventory entry from the paginated list
// endpoint (/v2/mobile-devices/detail).
//
// An inventory entry has many fields. Only the fields actively used by Teleport
// are mapped.
type MobileDevice struct {
	// MobileDeviceID is the mobile device identifier used by Jamf.
	MobileDeviceID string `json:"mobileDeviceId"`
	// DeviceType is the device type. Example: "iOS".
	// Note that "iOS" is used for both iPhones and iPads. Use
	// Hardware.ModelIdentifier to distinguish between them.
	DeviceType string                       `json:"deviceType"`
	General    *MobileDeviceGeneralSection  `json:"general"`
	Hardware   *MobileDeviceHardwareSection `json:"hardware"`
}

// MobileDeviceGeneralSection is the general section of a [MobileDevice].
type MobileDeviceGeneralSection struct {
	// OSVersion is the operating system version.
	// Example: "26.3.1".
	OSVersion string `json:"osVersion"`
	// OSBuild is the operating system build.
	// Example: "23D8133".
	OSBuild string `json:"osBuild"`
	// OSSupplementalBuildVersion is the supplemental build of the operating
	// system. May match rapid security response builds more closely.
	// Example: "23D771330a".
	OSSupplementalBuildVersion string `json:"osSupplementalBuildVersion"`
	// LastInventoryUpdateDate is the same as "Last Inventory Update" in the
	// Jamf UI.
	LastInventoryUpdateDate time.Time `json:"lastInventoryUpdateDate"`
	// LastEnrolledDate is the same as "Last Enrollment" in the Jamf UI.
	LastEnrolledDate time.Time `json:"lastEnrolledDate"`
}

// MobileDeviceHardwareSection is the hardware section of a [MobileDevice].
type MobileDeviceHardwareSection struct {
	// ModelIdentifier is the technical model identifier.
	// Example: "iPad15,7".
	ModelIdentifier string `json:"modelIdentifier"`
	// SerialNumber is the device serial number.
	SerialNumber string `json:"serialNumber"`
}

// MobileDeviceDetails is the response from the single mobile device endpoint
// (GET /v2/mobile-devices/{id}/detail).
//
// This has a different shape than the paginated list response [MobileDevice].
// Only the fields needed by Teleport are mapped.
type MobileDeviceDetails struct {
	// ID is the mobile device identifier.
	ID string `json:"id"`
	// SerialNumber is the device serial number.
	SerialNumber string `json:"serialNumber"`
	// Type is the device type, e.g., "ios", "tvos".
	// Note: this is lowercase, unlike [MobileDevice.DeviceType] which is "iOS".
	Type string `json:"type"`
	// IOS contains iOS-specific details. Non-nil for iOS/iPadOS devices.
	IOS *MobileDeviceDetailsIOS `json:"ios"`
}

// MobileDeviceDetailsIOS contains iOS-specific fields from the single mobile
// device response.
type MobileDeviceDetailsIOS struct {
	// ModelIdentifier is the technical model identifier.
	// Example: "iPad15,7".
	ModelIdentifier string `json:"modelIdentifier"`
}
