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
