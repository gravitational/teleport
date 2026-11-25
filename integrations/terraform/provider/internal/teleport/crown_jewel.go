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
