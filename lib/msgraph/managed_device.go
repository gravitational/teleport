// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package msgraph

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/trace"
)

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

// IterateManagedDevicePages iterates over managed devices, returning them page by page.
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-list?view=graph-rest-1.0
func (c *Client) IterateManagedDevicePages(ctx context.Context, f func(mds []*ManagedDevice) bool, iterateOpts ...IterateOpt) error {
	iterateOpts = append(iterateOpts, WithSelect(selectManagedDevice))
	return iteratePage(c, ctx, "deviceManagement/managedDevices", f, iterateOpts...)
}

// WithLastSyncDateTimeGt filters the result to only those whose lastSyncDateTime is greater than
// the given time.
func WithLastSyncDateTimeGt(lastSyncDateTime time.Time) IterateOpt {
	return func(ic *iterateConfig) {
		if lastSyncDateTime.IsZero() {
			return
		}
		// As noted in the docs, DateTimeOffset values aren't enclosed in quotes in $filter expressions.
		// https://learn.microsoft.com/en-us/graph/filter-query-parameter
		ic.filter = fmt.Sprintf("lastSyncDateTime gt %s", lastSyncDateTime.UTC().Format(time.RFC3339))
	}
}

// GetManagedDevice returns a single managed device.
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-get?view=graph-rest-1.0
func (c *Client) GetManagedDevice(ctx context.Context, id string) (*ManagedDevice, error) {
	uri := c.endpointURI("deviceManagement", "managedDevices", id)
	uri.RawQuery = url.Values{"$select": {selectManagedDevice}}.Encode()
	out, err := roundtrip[*ManagedDevice](ctx, c, http.MethodGet, uri.String(), nil)
	return out, trace.Wrap(err)
}
