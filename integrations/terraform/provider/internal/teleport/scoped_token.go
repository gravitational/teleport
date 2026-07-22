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
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewScopedTokenClient returns a scoped token client.
func NewScopedTokenClient(c *client.Client) ScopedTokenClient {
	return ScopedTokenClient{client: c}
}

// ScopedTokenClient manages scoped token resources.
type ScopedTokenClient struct {
	client *client.Client
}

// Get reads a scoped token by name.
func (r ScopedTokenClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*joiningv1.ScopedToken, error) {
	scopedToken, err := r.client.GetScopedToken(ctx, id.Name, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return scopedToken, nil
}

// Create creates a scoped token.
func (r ScopedTokenClient) Create(ctx context.Context, id *joiningv1.ScopedToken) error {
	_, err := r.client.CreateScopedToken(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a scoped token.
func (r ScopedTokenClient) Upsert(ctx context.Context, id *joiningv1.ScopedToken) error {
	_, err := r.client.UpsertScopedToken(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a scoped token by name.
func (r ScopedTokenClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteScopedToken(ctx, id.Name))
}
