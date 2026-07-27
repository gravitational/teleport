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
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewScopedRoleClient returns a scoped role client.
func NewScopedRoleClient(c *client.Client) ScopedRoleClient {
	return ScopedRoleClient{client: c}
}

// ScopedRoleClient manages scoped role resources.
type ScopedRoleClient struct {
	client *client.Client
}

// Get reads a scoped role by name.
func (r ScopedRoleClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*accessv1.ScopedRole, error) {
	resp, err := r.client.ScopedAccessServiceClient().GetScopedRole(ctx, &accessv1.GetScopedRoleRequest{
		Name:  id.Name,
		Scope: id.Scope,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.GetRole(), nil
}

// Create creates a scoped role.
func (r ScopedRoleClient) Create(ctx context.Context, role *accessv1.ScopedRole) error {
	_, err := r.client.ScopedAccessServiceClient().CreateScopedRole(ctx, &accessv1.CreateScopedRoleRequest{
		Role: role,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a scoped role.
func (r ScopedRoleClient) Upsert(ctx context.Context, role *accessv1.ScopedRole) error {
	_, err := r.client.ScopedAccessServiceClient().UpsertScopedRole(ctx, &accessv1.UpsertScopedRoleRequest{
		Role: role,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a scoped role by name.
func (r ScopedRoleClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	_, err := r.client.ScopedAccessServiceClient().DeleteScopedRole(ctx, &accessv1.DeleteScopedRoleRequest{
		Name:  id.Name,
		Scope: id.Scope,
	})
	return trace.Wrap(err)
}
