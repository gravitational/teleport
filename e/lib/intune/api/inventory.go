package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

// ListManagedDevicesRequest is the request struct for [Client.ListManagedDevices].
type ListManagedDevicesRequest struct {
	// NextLink is the URL to the next page of results from an earlier response. If provided, any
	// other field in this struct is ignored.
	//
	// As an additional precaution, only the path and the query of NextLink are used for the
	// subsequent request – the client always sends the request to GraphEndpoint from [Config]. But
	// the Graph API should always respond with a NextLink with the same host anyway.
	NextLink string
	// LastSyncDateTime limits the returned devices to only those that were last sync only after the
	// provided time.
	LastSyncDateTime time.Time
	// Top limits the number of returned results.
	Top int
}

func (r *ListManagedDevicesRequest) query() url.Values {
	q := make(url.Values)
	// Select only the fields that we need.
	// https://learn.microsoft.com/en-us/graph/best-practices-concept#use-projections
	q.Set("$select", selectManagedDevice)

	// Filter by the time of last sync of a device with Intune.
	// https://learn.microsoft.com/en-us/graph/filter-query-parameter
	// https://learn.microsoft.com/en-us/graph/api/resources/intune-devices-manageddevice?view=graph-rest-1.0
	// As noted in the docs, DateTimeOffset values aren't enclosed in quotes in $filter expressions.
	if !r.LastSyncDateTime.IsZero() {
		q.Set("$filter", fmt.Sprintf("lastSyncDateTime gt %s", r.LastSyncDateTime.UTC().Format(time.RFC3339)))
	}

	// Limit the number of results.
	// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#top
	if r.Top > 0 {
		q.Set("$top", strconv.Itoa(r.Top))
	}

	return q
}

// ListManagedDevicesResponse is the response for
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-list?view=graph-rest-1.0
type ListManagedDevicesResponse struct {
	// ManagedDevices is a list of devices returned by the endpoint.
	ManagedDevices []*ManagedDevice `json:"value"`
	// NextLink is the URL to the next page of results. Empty if there's no more pages to fetch.
	NextLink string `json:"@odata.nextLink"`
}

// ManagedDevice represents a device from Intune's inventory.
type ManagedDevice struct {
	// ID is the unique ID of the device within Intune.
	ID string `json:"id"`
	// LastSyncDateTime is the time that the device last completed a successful sync with Intune.
	LastSyncDateTime time.Time `json:"lastSyncDateTime"`
	// DeviceRegistrationState describes whether a device was fully enrolled within Intune.
	// Possible values are: notRegistered, registered, revoked, keyConflict, approvalPending,
	// certificateReset, notRegisteredPendingEnrollment, unknown.
	DeviceRegistrationState string `json:"deviceRegistrationState"`
	SerialNumber            string `json:"serialNumber"`
	Model                   string `json:"model"`
	// OperatingSystem is the OS of the device, e.g. "Windows", "macOS", "Linux (ubuntu)".
	OperatingSystem string `json:"operatingSystem"`
	// OSVersion is the version of the OS, e.g. "10.0.26100.4351" (Windows), "15.5 (24F74)" (macOS),
	// "24.04" (Linux).
	OSVersion string `json:"osVersion"`
}

// selectManagedDevice is the value for the $select query param when fetching managed devices so
// that the client fetches only the fields that it needs.
const selectManagedDevice = "id,lastSyncDateTime,deviceRegistrationState,operatingSystem,serialNumber,model,osVersion"

const (
	// DeviceRegistrationStateRegistered is DeviceRegistrationState value of [ManagedDevice]
	// set after the device is fully enrolled into Intune.
	DeviceRegistrationStateRegistered = "registered"
)

const managedDevicesPath = "/v1.0/deviceManagement/managedDevices"

// ListManagedDevices returns a list of managed devices within Intune.
// The app authenticated with the Intune API must have at least the
// DeviceManagementManagedDevices.Read.All permission to access this endpoint.
// It automatically paginates results. [ListManagedDevicesResponse] has the NextLink field present
// if there are more records to fetch. The caller is then expected to pass it to
// [ListManagedDevicesRequest] under the same field.
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-list?view=graph-rest-1.0
func (c *Client) ListManagedDevices(ctx context.Context, req *ListManagedDevicesRequest) (*ListManagedDevicesResponse, error) {
	var reqURL string
	if req.NextLink != "" {
		reqURL = req.NextLink
	} else {
		u := c.graphURL.JoinPath(managedDevicesPath)
		u.RawQuery = req.query().Encode()
		reqURL = u.String()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp := &ListManagedDevicesResponse{}
	return resp, trace.Wrap(c.doGraphRequest(httpReq, resp))
}

// GetManagedDevice returns a single device by ID.
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-get?view=graph-rest-1.0
func (c *Client) GetManagedDevice(ctx context.Context, id string) (*ManagedDevice, error) {
	u := c.graphURL.JoinPath(managedDevicesPath, id)
	q := u.Query()
	q.Set("$select", selectManagedDevice)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &ManagedDevice{}
	return resp, trace.Wrap(c.doGraphRequest(httpReq, resp))
}
