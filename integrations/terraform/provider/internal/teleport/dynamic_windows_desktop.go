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

// NewDynamicWindowsDesktopClient returns a dynamic Windows desktop client.
func NewDynamicWindowsDesktopClient(c *client.Client) DynamicWindowsDesktopClient {
	return DynamicWindowsDesktopClient{client: c}
}

// DynamicWindowsDesktopClient manages dynamic Windows desktop resources.
type DynamicWindowsDesktopClient struct {
	client *client.Client
}

// Get reads a dynamic Windows desktop by name.
func (r DynamicWindowsDesktopClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.DynamicWindowsDesktopV1, error) {
	desktop, err := r.client.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktopv1, ok := desktop.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected dynamic Windows desktop type: %T", desktop)
	}

	return desktopv1, nil
}

// Create creates a dynamic Windows desktop.
func (r DynamicWindowsDesktopClient) Create(ctx context.Context, desktop *types.DynamicWindowsDesktopV1) error {
	if _, err := r.client.DynamicDesktopClient().CreateDynamicWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a dynamic Windows desktop.
func (r DynamicWindowsDesktopClient) Upsert(ctx context.Context, desktop *types.DynamicWindowsDesktopV1) error {
	if _, err := r.client.DynamicDesktopClient().UpsertDynamicWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a dynamic Windows desktop by name.
func (r DynamicWindowsDesktopClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, id.Name))
}
