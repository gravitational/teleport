package intune

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
)

// ListManagedDevices returns a list of managed devices within Intune.
// The app authenticated with the Intune API must have at least the
// DeviceManagementManagedDevices.Read.All permission to access this endpoint.
// https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-list?view=graph-rest-1.0
func (c *Client) ListManagedDevices(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/v1.0/deviceManagement/managedDevices"), nil /* body */)
	if err != nil {
		return trace.Wrap(err)
	}
	var resp map[string]any
	return trace.Wrap(c.doAuthnRequest(req, &resp))
}
