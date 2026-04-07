// Copyright 2026 Gravitational, Inc.
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

package client

import (
	"context"

	"github.com/gravitational/teleport/api/client/scopes/access"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

// AccessClient is a wrapper around the scoped access Client that unwraps gRPC
// request/response types into the simple signatures expected by terraform provider code generation.
type AccessClient struct {
	client *access.Client
}

// NewAccessClient creates a new AccessClient wrapping the given scoped access Client.
func NewAccessClient(client *access.Client) *AccessClient {
	return &AccessClient{client: client}
}

// GetScopedRole gets a scoped role by name.
func (t *AccessClient) GetScopedRole(ctx context.Context, name string) (*scopedaccessv1.ScopedRole, error) {
	res, err := t.client.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	return res.GetRole(), nil
}

// CreateScopedRole creates a new scoped role.
func (t *AccessClient) CreateScopedRole(ctx context.Context, role *scopedaccessv1.ScopedRole) (*scopedaccessv1.ScopedRole, error) {
	res, err := t.client.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: role,
	})
	if err != nil {
		return nil, err
	}
	return res.GetRole(), nil
}

// UpsertScopedRole creates or updates a scoped role.
func (t *AccessClient) UpsertScopedRole(ctx context.Context, role *scopedaccessv1.ScopedRole) (*scopedaccessv1.ScopedRole, error) {
	res, err := t.client.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: role,
	})
	if err != nil {
		return nil, err
	}
	return res.GetRole(), nil
}

// DeleteScopedRole deletes a scoped role by name.
func (t *AccessClient) DeleteScopedRole(ctx context.Context, name string) error {
	_, err := t.client.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: name,
	})
	return err
}
