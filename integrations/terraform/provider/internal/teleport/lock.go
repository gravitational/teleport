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

// NewLockClient returns a lock client.
func NewLockClient(c *client.Client) LockClient {
	return LockClient{client: c}
}

// LockClient manages lock resources.
type LockClient struct {
	client *client.Client
}

// Get reads a lock by name.
func (r LockClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.LockV2, error) {
	lock, err := r.client.GetLock(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lockv2, ok := lock.(*types.LockV2)
	if !ok {
		return nil, trace.BadParameter("unexpected lock type: %T", lock)
	}

	return lockv2, nil
}

// Create creates a lock.
func (r LockClient) Create(ctx context.Context, lock *types.LockV2) error {
	if err := r.client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a lock.
func (r LockClient) Upsert(ctx context.Context, lock *types.LockV2) error {
	if err := r.client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a lock by name.
func (r LockClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteLock(ctx, id.Name))
}
