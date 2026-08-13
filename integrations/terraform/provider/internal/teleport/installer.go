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

// NewInstallerClient returns an installer client.
func NewInstallerClient(c *client.Client) InstallerClient {
	return InstallerClient{client: c}
}

// InstallerClient manages installer resources.
type InstallerClient struct {
	client *client.Client
}

// Get reads an installer by name.
func (r InstallerClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.InstallerV1, error) {
	installer, err := r.client.GetInstaller(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installerv1, ok := installer.(*types.InstallerV1)
	if !ok {
		return nil, trace.BadParameter("unexpected installer type: %T", installer)
	}

	return installerv1, nil
}

// Create creates an installer.
func (r InstallerClient) Create(ctx context.Context, installer *types.InstallerV1) error {
	if err := r.client.SetInstaller(ctx, installer); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an installer.
func (r InstallerClient) Upsert(ctx context.Context, installer *types.InstallerV1) error {
	if err := r.client.SetInstaller(ctx, installer); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an installer by name.
func (r InstallerClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteInstaller(ctx, id.Name))
}
