// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
