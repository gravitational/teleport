package jamf

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

const (
	// SectionGeneral is the GENERAL section of a [ComputerInventory] instance.
	SectionGeneral = "GENERAL"
	// SectionHardware is the HARDWARE section of a [ComputerInventory] instance.
	SectionHardware = "HARDWARE"
	// SectionLocalUserAccounts is the LOCAL_USER_ACCOUNTS section of a
	// [ComputerInventory] instance.
	SectionLocalUserAccounts = "LOCAL_USER_ACCOUNTS"
	// SectionOperatingSystem is the OPERATING_SYSTEM section of a
	// [ComputerInventory] instance.
	SectionOperatingSystem = "OPERATING_SYSTEM"
)

// GetComputersInventoryRequest is the request for
// https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory.
type GetComputersInventoryRequest struct {
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
	// Example: "general.name:asc".
	Sort []string
	// Filter is the RSQL filter applied to the query.
	// See the API docs for supported fields.
	// Example: `general.name=="Orchard"`.
	Filter string
}

// GetComputersInventoryResponse is the response for
// https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory.
type GetComputersInventoryResponse struct {
	// TotalCount is the total inventory count, regardless of the requested page.
	TotalCount int                  `json:"totalCount"`
	Results    []*ComputerInventory `json:"results"`
}

// GetComputersInventoryByIDRequest is the request for
// https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory-id.
type GetComputersInventoryByIDRequest struct {
	// ID is the computer inventory identifier.
	ID string `json:"-"`
	// Section is the slice of sections to query.
	// If empty, the general section is returned.
	Section []string `json:"-"`
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
	// Name is the descriptive name of the computer.
	// Example: "llama's MacBook".
	Name string `json:"name"`
	// JamfBinaryVersion is the jamf binary version, if present.
	// Example: "9.27".
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
	// SupplementalBuildVersion is the supplemental build of the operating system.
	// May match `sw_vers` BuildVersion more closely in certain situations, like
	// macOS rapid security response builds.
	// Example: "13A953".
	SupplementalBuildVersion string `json:"supplementalBuildVersion"`
	// RapidSecurityResponse is the rapid security response denominator.
	// Matches `sw_vers` ProductVersionExtra.
	// Example: "(a)".
	RapidSecurityResponse string `json:"rapidSecurityResponse"`
}

// GetComputersInventory returns paginated computer inventory records.
// See https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory.
func (c *Client) GetComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest) (*GetComputersInventoryResponse, error) {
	if req == nil {
		return nil, trace.BadParameter("req required")
	}

	q := make(url.Values)
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	if req.PageSize > 0 {
		q.Set("page-size", strconv.Itoa(req.PageSize))
	}
	for _, s := range req.Section {
		if s != "" {
			q.Add("section", s)
		}
	}
	for _, s := range req.Sort {
		if s != "" {
			q.Add("sort", s)
		}
	}
	if req.Filter != "" {
		q.Set("filter", req.Filter)
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/v1/computers-inventory"), nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	getReq.URL.RawQuery = q.Encode()

	resp := &GetComputersInventoryResponse{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}

// GetComputersInventoryByID returns a single computer.
// See https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory-id.
func (c *Client) GetComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest) (*ComputerInventory, error) {
	switch {
	case req == nil:
		return nil, trace.BadParameter("req required")
	case req.ID == "":
		// Don't query without an ID, the response is the same as
		// listing/GetComputersInventory.
		return nil, trace.BadParameter("id required")
	}

	q := make(url.Values)
	for _, s := range req.Section {
		q.Add("section", s)
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/v1/computers-inventory/"+req.ID), nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	getReq.URL.RawQuery = q.Encode()

	resp := &ComputerInventory{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}
