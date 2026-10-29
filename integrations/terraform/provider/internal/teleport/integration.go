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
