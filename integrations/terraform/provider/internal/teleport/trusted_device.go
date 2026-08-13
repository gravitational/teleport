// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewTrustedDeviceClient returns a trusted device client.
func NewTrustedDeviceClient(c *client.Client) TrustedDeviceClient {
	return TrustedDeviceClient{client: c}
}

// TrustedDeviceClient manages trusted device resources.
type TrustedDeviceClient struct {
	client *client.Client
}

// Get reads a trusted device by name.
func (r TrustedDeviceClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.DeviceV1, error) {
	trustedDevice, err := r.client.GetDeviceResource(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedDevice, nil
}

// Create creates a trusted device.
func (r TrustedDeviceClient) Create(ctx context.Context, trustedDevice *types.DeviceV1) error {
	if _, err := r.client.UpsertDeviceResource(ctx, trustedDevice); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a trusted device.
func (r TrustedDeviceClient) Upsert(ctx context.Context, trustedDevice *types.DeviceV1) error {
	if _, err := r.client.UpsertDeviceResource(ctx, trustedDevice); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a trusted device by name.
func (r TrustedDeviceClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteDeviceResource(ctx, id.Name))
}
