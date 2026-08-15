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
