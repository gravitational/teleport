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

// NewUserClient returns a user client.
func NewUserClient(c *client.Client) UserClient {
	return UserClient{client: c}
}

// UserClient manages user resources.
type UserClient struct {
	client *client.Client
}

// Get reads a user by name.
func (r UserClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.UserV2, error) {
	user, err := r.client.GetUser(ctx, id.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userv2, ok := user.(*types.UserV2)
	if !ok {
		return nil, trace.BadParameter("unexpected user type: %T", user)
	}

	return userv2, nil
}

// Create creates a user.
func (r UserClient) Create(ctx context.Context, user *types.UserV2) error {
	if _, err := r.client.CreateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a user.
func (r UserClient) Upsert(ctx context.Context, user *types.UserV2) error {
	if _, err := r.client.UpsertUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a user by name.
func (r UserClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteUser(ctx, id.Name))
}
