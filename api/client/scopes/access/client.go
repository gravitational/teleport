// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package access

import (
	"context"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

// Client is a scoped access client that conforms to the following lib/services interfaces:
// * services.ScopedAccess
type Client struct {
	grpcClient scopedaccessv1.ScopedAccessServiceClient
}

// NewClient creates a new Access List client.
func NewClient(grpcClient scopedaccessv1.ScopedAccessServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetScopedRole gets a scoped role by name.
func (c *Client) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	return c.grpcClient.GetScopedRole(ctx, req)
}

// ListScopedRoles returns a paginated list of scoped roles.
func (c *Client) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	return c.grpcClient.ListScopedRoles(ctx, req)
}

// CreateScopedRole creates a new scoped role.
func (c *Client) CreateScopedRole(ctx context.Context, req *scopedaccessv1.CreateScopedRoleRequest) (*scopedaccessv1.CreateScopedRoleResponse, error) {
	return c.grpcClient.CreateScopedRole(ctx, req)
}

// UpdateScopedRole updates a scoped role.
func (c *Client) UpdateScopedRole(ctx context.Context, req *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error) {
	return c.grpcClient.UpdateScopedRole(ctx, req)
}

// DeleteScopedRole deletes a scoped role.
func (c *Client) DeleteScopedRole(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error) {
	return c.grpcClient.DeleteScopedRole(ctx, req)
}

// GetScopedRoleAssignment gets a scoped role assignment by name.
func (c *Client) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	return c.grpcClient.GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
func (c *Client) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	return c.grpcClient.ListScopedRoleAssignments(ctx, req)
}

// CreateScopedRoleAssignment creates a new scoped role assignment.
func (c *Client) CreateScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error) {
	return c.grpcClient.CreateScopedRoleAssignment(ctx, req)
}

// DeleteScopedRoleAssignment deletes a scoped role assignment.
func (c *Client) DeleteScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error) {
	return c.grpcClient.DeleteScopedRoleAssignment(ctx, req)
}
