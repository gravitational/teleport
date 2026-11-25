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
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewIntegrationClient returns an integration client.
func NewIntegrationClient(c *client.Client) IntegrationClient {
	return IntegrationClient{client: c}
}

// IntegrationClient manages integration resources.
type IntegrationClient struct {
	client *client.Client
}

// Get reads an integration by name and scope.
func (r IntegrationClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*types.IntegrationV1, error) {
	ig, err := r.client.GetIntegrationV2(ctx, integrationpb.GetIntegrationRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected integration type: %T", ig)
	}

	return igV1, nil
}

// Create creates an integration.
func (r IntegrationClient) Create(ctx context.Context, ig *types.IntegrationV1) error {
	if _, err := r.client.CreateIntegration(ctx, ig); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an integration.
func (r IntegrationClient) Upsert(ctx context.Context, ig *types.IntegrationV1) error {
	if _, err := r.client.UpdateIntegration(ctx, ig); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an integration by name and scope.
func (r IntegrationClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteIntegrationV2(ctx, integrationpb.DeleteIntegrationRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
