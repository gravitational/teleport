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
	"github.com/gravitational/teleport/lib/scopes/access"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewScopedRoleAssignmentClient returns a scoped role assignment client.
func NewScopedRoleAssignmentClient(c *client.Client) ScopedRoleAssignmentClient {
	return ScopedRoleAssignmentClient{client: c}
}

// ScopedRoleAssignmentClient manages scoped role assignment resources.
type ScopedRoleAssignmentClient struct {
	client *client.Client
}

// Get reads a scoped role assignment by name.
func (r ScopedRoleAssignmentClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*accessv1.ScopedRoleAssignment, error) {
	resp, err := r.client.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, &accessv1.GetScopedRoleAssignmentRequest{
		Name:    id.Name,
		Scope:   id.Scope,
		SubKind: access.SubKindDynamic,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.GetAssignment(), nil
}

// Create creates a scoped role assignment.
func (r ScopedRoleAssignmentClient) Create(ctx context.Context, sra *accessv1.ScopedRoleAssignment) error {
	_, err := r.client.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, &accessv1.CreateScopedRoleAssignmentRequest{
		Assignment: sra,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a scoped role assignment.
func (r ScopedRoleAssignmentClient) Upsert(ctx context.Context, sra *accessv1.ScopedRoleAssignment) error {
	_, err := r.client.ScopedAccessServiceClient().UpsertScopedRoleAssignment(ctx, &accessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: sra,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a scoped role assignment by name.
func (r ScopedRoleAssignmentClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	_, err := r.client.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, &accessv1.DeleteScopedRoleAssignmentRequest{
		Name:    id.Name,
		Scope:   id.Scope,
		SubKind: access.SubKindDynamic,
	})

	return trace.Wrap(err)
}
