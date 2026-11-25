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

// NewRoleClient returns a role client.
func NewRoleClient(c *client.Client) RoleClient {
	return RoleClient{client: c}
}

// RoleClient manages role resources.
type RoleClient struct {
	client *client.Client
}

// Get reads a role by name.
func (r RoleClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.RoleV6, error) {
	role, err := r.client.GetRole(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rolev6, ok := role.(*types.RoleV6)
	if !ok {
		return nil, trace.BadParameter("unexpected role type: %T", role)
	}

	return rolev6, nil
}

// Create creates a role.
func (r RoleClient) Create(ctx context.Context, role *types.RoleV6) error {
	if _, err := r.client.CreateRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a role.
func (r RoleClient) Upsert(ctx context.Context, role *types.RoleV6) error {
	if _, err := r.client.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a role by name.
func (r RoleClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteRole(ctx, id.Name))
}
