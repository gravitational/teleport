package jamf

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// GetComputersInventoryRequest is a list request for computer inventory.
// It is compatible with both v1 and v2 Jamf API versions.
//
// * https://developer.jamf.com/jamf-pro/reference/get_v2-computers-inventory
// * https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory
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
// [GetComputersInventoryRequest].
// It is compatible with both v1 and v2 Jamf API versions.
type GetComputersInventoryResponse struct {
	// TotalCount is the total inventory count, regardless of the requested page.
	TotalCount int                  `json:"totalCount"`
	Results    []*ComputerInventory `json:"results"`
}

// GetComputersInventoryByIDRequest is a request for a single computer inventory
// entry.
// It is compatible with both v1 and v2 Jamf API versions.
//
// * https://developer.jamf.com/jamf-pro/reference/get_v2-computers-inventory-id
// * https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory-id
type GetComputersInventoryByIDRequest struct {
	// ID is the computer inventory identifier.
	ID string `json:"-"`
	// Section is the slice of sections to query.
	// If empty, the general section is returned.
	Section []string `json:"-"`
}

// ComputerInventory is a computer inventory entry.
// It is compatible with both v1 and v2 Jamf API versions.
//
// An inventory entry has many, many fields. Only the fields actively used by
// Teleport are mapped.
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
//
// It'll choose between /v2/computers-inventory and /v1/computers-inventory
// depending on API availability, preferring the former.
//
// * https://developer.jamf.com/jamf-pro/reference/get_v2-computers-inventory
// * https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory
func (c *Client) GetComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest) (*GetComputersInventoryResponse, error) {
	if c.useComputersInventoryV2 {
		return c.getV2ComputersInventory(ctx, req)
	} else {
		return c.getV1ComputersInventory(ctx, req)
	}
}

func (c *Client) getV2ComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest) (*GetComputersInventoryResponse, error) {
	return c.getComputersInventory(ctx, req, 2 /* apiVersion */)
}

func (c *Client) getV1ComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest) (*GetComputersInventoryResponse, error) {
	return c.getComputersInventory(ctx, req, 1 /* apiVersion */)
}

func (c *Client) getComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest, apiVersion int,
) (*GetComputersInventoryResponse, error) {
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

	url := c.endpoint(fmt.Sprintf("/v%d/computers-inventory", apiVersion))
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	getReq.URL.RawQuery = q.Encode()

	resp := &GetComputersInventoryResponse{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}

// GetComputersInventoryByID returns a single computer.
//
// It'll choose between /v2/computers-inventory and /v1/computers-inventory
// depending on API availability, preferring the former.
//
// * https://developer.jamf.com/jamf-pro/reference/get_v2-computers-inventory-id
// * https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory-id
func (c *Client) GetComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest) (*ComputerInventory, error) {
	if c.useComputersInventoryV2 {
		return c.getV2ComputersInventoryByID(ctx, req)
	} else {
		return c.getV1ComputersInventoryByID(ctx, req)
	}
}

func (c *Client) getV2ComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest) (*ComputerInventory, error) {
	return c.getComputersInventoryByID(ctx, req, 2 /* apiVersion */)
}

func (c *Client) getV1ComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest) (*ComputerInventory, error) {
	return c.getComputersInventoryByID(ctx, req, 1 /* apiVersion */)
}

func (c *Client) getComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest, apiVersion int,
) (*ComputerInventory, error) {
	switch {
	case req == nil:
		return nil, trace.BadParameter("req required")
	case req.ID == "":
		// Don't query without an ID, the response is the same as
		// listing/GetComputersInventory.
		return nil, trace.BadParameter("id required")
	}

	if err := validateDeviceID(req.ID); err != nil {
		return nil, trace.Wrap(err)
	}

	q := make(url.Values)
	for _, s := range req.Section {
		q.Add("section", s)
	}

	url := c.endpoint(fmt.Sprintf("/v%d/computers-inventory/%s", apiVersion, req.ID))
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	getReq.URL.RawQuery = q.Encode()

	resp := &ComputerInventory{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}

// GetMobileDevicesDetail returns paginated mobile device inventory records.
// The endpoint was added in Jamf Pro 10.48.0. https://developer.jamf.com/jamf-pro/changelog/10480-additions
//
// https://developer.jamf.com/jamf-pro/reference/get_v2-mobile-devices-detail
func (c *Client) GetMobileDevicesDetail(
	ctx context.Context, req *GetMobileDevicesDetailRequest) (*GetMobileDevicesDetailResponse, error) {
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

	url := c.endpoint("/v2/mobile-devices/detail")
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	getReq.URL.RawQuery = q.Encode()

	resp := &GetMobileDevicesDetailResponse{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}

// GetMobileDeviceByID returns a single mobile device.
//
// https://developer.jamf.com/jamf-pro/reference/get_v2-mobile-devices-id-detail
func (c *Client) GetMobileDeviceByID(
	ctx context.Context, req *GetMobileDeviceByIDRequest) (*MobileDeviceDetails, error) {
	switch {
	case req == nil:
		return nil, trace.BadParameter("req required")
	case req.ID == "":
		return nil, trace.BadParameter("id required")
	}

	if err := validateDeviceID(req.ID); err != nil {
		return nil, trace.Wrap(err)
	}

	url := c.endpoint(fmt.Sprintf("/v2/mobile-devices/%s/detail", req.ID))
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &MobileDeviceDetails{}
	return resp, trace.Wrap(c.doAuthnJSONRequest(getReq, resp))
}

// validateDeviceID prevents malicious request IDs from making requests to other APIs.
func validateDeviceID(deviceID string) error {
	switch {
	case url.PathEscape(deviceID) != deviceID:
		return trace.BadParameter("invalid device ID %q", deviceID)
	case deviceID == ".." || strings.Contains(deviceID, "../"):
		return trace.BadParameter("invalid device ID %q", deviceID)
	default:
		return nil
	}
}
