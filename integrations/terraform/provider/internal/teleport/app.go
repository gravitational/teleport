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

// NewAppClient returns an app client.
func NewAppClient(c *client.Client) AppClient {
	return AppClient{client: c}
}

// AppClient manages app resources.
type AppClient struct {
	client *client.Client
}

// Get reads an app by name.
func (r AppClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.AppV3, error) {
	app, err := r.client.GetApp(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appv3, ok := app.(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("unexpected application type: %T", app)
	}

	return appv3, nil
}

// Create creates an app.
func (r AppClient) Create(ctx context.Context, app *types.AppV3) error {
	if err := r.client.CreateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an app.
func (r AppClient) Upsert(ctx context.Context, app *types.AppV3) error {
	if err := r.client.UpdateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an app by name.
func (r AppClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteApp(ctx, id.Name))
}
