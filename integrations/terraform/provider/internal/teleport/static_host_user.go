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
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewStaticHostUserClient returns a static host user client.
func NewStaticHostUserClient(c *client.Client) StaticHostUserClient {
	return StaticHostUserClient{client: c}
}

// StaticHostUserClient manages static host user resources.
type StaticHostUserClient struct {
	client *client.Client
}

// Get reads a static host user by name.
func (r StaticHostUserClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*userprovisioningv2.StaticHostUser, error) {
	staticHostUser, err := r.client.StaticHostUserClient().GetStaticHostUser(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return staticHostUser, nil
}

// Create creates a static host user.
func (r StaticHostUserClient) Create(ctx context.Context, staticHostUser *userprovisioningv2.StaticHostUser) error {
	if _, err := r.client.StaticHostUserClient().CreateStaticHostUser(ctx, staticHostUser); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a static host user.
func (r StaticHostUserClient) Upsert(ctx context.Context, staticHostUser *userprovisioningv2.StaticHostUser) error {
	if _, err := r.client.StaticHostUserClient().UpsertStaticHostUser(ctx, staticHostUser); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a static host user by name.
func (r StaticHostUserClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.StaticHostUserClient().DeleteStaticHostUser(ctx, id.Name))
}
