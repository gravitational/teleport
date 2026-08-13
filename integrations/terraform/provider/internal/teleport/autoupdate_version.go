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
	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewAutoUpdateVersionClient returns a Teleport client to interact with the autoupdate version.
func NewAutoUpdateVersionClient(c *client.Client) AutoUpdateVersionClient {
	return AutoUpdateVersionClient{client: c}
}

// AutoUpdateVersionClient manages the Teleport autoupdate version.
type AutoUpdateVersionClient struct {
	client *client.Client
}

// Get reads the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*autoupdatev1.AutoUpdateVersion, error) {
	version, err := r.client.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// Create creates the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Create(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) error {
	if _, err := r.client.CreateAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Upsert(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) error {
	if _, err := r.client.UpsertAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete removes the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.DeleteAutoUpdateVersion(ctx))
}
