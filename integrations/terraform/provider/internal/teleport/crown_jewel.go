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
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewCrownJewelClient returns a crown jewel client.
func NewCrownJewelClient(c *client.Client) CrownJewelClient {
	return CrownJewelClient{client: c}
}

// CrownJewelClient manages crown jewel resources.
type CrownJewelClient struct {
	client *client.Client
}

// Get reads a crown jewel by name.
func (r CrownJewelClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*crownjewelv1.CrownJewel, error) {
	crownJewel, err := r.client.CrownJewelServiceClient().GetCrownJewel(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return crownJewel, nil
}

// Create creates a crown jewel.
func (r CrownJewelClient) Create(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) error {
	if _, err := r.client.CrownJewelServiceClient().CreateCrownJewel(ctx, crownJewel); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a crown jewel.
//
// UpsertCrownJewel is used rather than UpdateCrownJewel because the latter is
// conditional on the resource revision, which is excluded from the Terraform
// schema and so is never present in the plan.
func (r CrownJewelClient) Upsert(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) error {
	if _, err := r.client.CrownJewelServiceClient().UpsertCrownJewel(ctx, crownJewel); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a crown jewel by name.
func (r CrownJewelClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.CrownJewelServiceClient().DeleteCrownJewel(ctx, id.Name))
}
