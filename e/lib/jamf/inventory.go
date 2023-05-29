package jamf

import "time"

// GetComputersInventoryResponse is the response for
// https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory.
type GetComputersInventoryResponse struct {
	// TotalCount is the total inventory count, regardless of the requested page.
	TotalCount int                  `json:"totalCount"`
	Results    []*ComputerInventory `json:"results"`
}

// ComputerInventory is a computer inventory entry.
// An inventory entry has many, many fields. Only the fields actively used by
// Teleport are mapped.
// See [GetComputersInventoryResponse].
type ComputerInventory struct {
	ID                string                          `json:"id"`
	UDID              string                          `json:"udid"`
	General           *ComputerGeneralSection         `json:"general"`
	Hardware          *ComputerHardwareSection        `json:"hardware"`
	LocalUserAccounts []*LocalUserAccount             `json:"localUserAccounts"`
	OperatingSystem   *ComputerOperatingSystemSection `json:"operatingSystem"`
}

type ComputerGeneralSection struct {
	JamfBinaryVersion string `json:"jamfBinaryVersion"`
	// Platform is the computer platform.
	// Example: "Mac".
	Platform string `json:"platform"`
	// ReportDate is the same as the "Last Inventory Update" in the Jamf UI.
	ReportDate time.Time `json:"reportDate"`
	// LastContactTime is the same as "Last Check-in" in the Jamf UI.
	LastContactTime time.Time `json:"lastContactTime"`
	// LastEnrolledDate is the same as "Last Enrollment" in the Jamf UI.
	LastEnrolledDate time.Time `json:"lastEnrolledDate"`
}

type ComputerHardwareSection struct {
	// ModelIdentifier of the computer.
	// Example: "MacBookPro9,2".
	ModelIdentifier string `json:"modelIdentifier"`
	// SerialNumber is the computer serial number.
	// This is the same as the system serial number for Apple computers.
	SerialNumber string `json:"serialNumber"`
}

type LocalUserAccount struct {
	// UID is the local user UID.
	// Example: "501" (macOS).
	UID string `json:"uid"`
	// Username is the local user name.
	// Example: "llama".
	Username string `json:"username"`
	// FullName is the local user display name.
	// Example: "John Llama".
	FullName string `json:"fullName"`
}

type ComputerOperatingSystemSection struct {
	// Name is the name of the operating system.
	// Example: "Mac OS X".
	Name string `json:"name"`
	// Version is the version of the operating system.
	// Example: "10.9.5".
	Version string `json:"version"`
	// Build is the build of the operating system.
	// Example: "13A603".
	Build string `json:"build"`
}
