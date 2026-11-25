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
