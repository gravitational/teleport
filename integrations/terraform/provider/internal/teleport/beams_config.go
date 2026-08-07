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
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewBeamsConfigClient returns a beams config client.
func NewBeamsConfigClient(c *client.Client) BeamsConfigClient {
	return BeamsConfigClient{client: c}
}

// BeamsConfigClient manages the singleton BeamsConfig resource.
type BeamsConfigClient struct {
	client *client.Client
}

// Get reads the BeamsConfig singleton.
func (r BeamsConfigClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*beamsv1.BeamsConfig, error) {
	resp, err := r.client.BeamsConfigServiceClient().GetBeamsConfig(ctx, &beamsv1.GetBeamsConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetBeamsConfig(), nil
}

// Create creates the BeamsConfig singleton.
func (r BeamsConfigClient) Create(ctx context.Context, config *beamsv1.BeamsConfig) error {
	_, err := r.client.BeamsConfigServiceClient().CreateBeamsConfig(ctx, &beamsv1.CreateBeamsConfigRequest{
		BeamsConfig: config,
	})
	return trace.Wrap(err)
}

// Upsert updates the BeamsConfig singleton.
func (r BeamsConfigClient) Upsert(ctx context.Context, config *beamsv1.BeamsConfig) error {
	_, err := r.client.BeamsConfigServiceClient().UpdateBeamsConfig(ctx, &beamsv1.UpdateBeamsConfigRequest{
		BeamsConfig: config,
	})
	return trace.Wrap(err)
}

// Delete deletes the BeamsConfig singleton.
func (r BeamsConfigClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	_, err := r.client.BeamsConfigServiceClient().DeleteBeamsConfig(ctx, &beamsv1.DeleteBeamsConfigRequest{})
	return trace.Wrap(err)
}

// PrepareUpdate copies the revision from the existing resource before update.
func (r BeamsConfigClient) PrepareUpdate(resourceBefore, resourceNew *beamsv1.BeamsConfig) error {
	if meta := resourceNew.GetMetadata(); meta != nil {
		meta.SetRevision(resourceBefore.GetMetadata().GetRevision())
	}
	return nil
}
