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
