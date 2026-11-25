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

// Upsert creates or replaces the BeamsConfig singleton.
func (r BeamsConfigClient) Upsert(ctx context.Context, config *beamsv1.BeamsConfig) error {
	_, err := r.client.BeamsConfigServiceClient().UpsertBeamsConfig(ctx, &beamsv1.UpsertBeamsConfigRequest{
		BeamsConfig: config,
	})
	return trace.Wrap(err)
}

// Delete deletes the BeamsConfig singleton. NotFound is treated as success
// in case the resource was already deleted out of band.
func (r BeamsConfigClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	_, err := r.client.BeamsConfigServiceClient().DeleteBeamsConfig(ctx, &beamsv1.DeleteBeamsConfigRequest{})
	if trace.IsNotFound(err) {
		return nil
	}
	return trace.Wrap(err)
}

// PrepareUpdate copies the revision from the existing resource before update.
func (r BeamsConfigClient) PrepareUpdate(resourceBefore, resourceNew *beamsv1.BeamsConfig) error {
	if meta := resourceNew.GetMetadata(); meta != nil {
		meta.SetRevision(resourceBefore.GetMetadata().GetRevision())
	}
	return nil
}
