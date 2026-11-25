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
