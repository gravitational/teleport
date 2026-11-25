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
func (r ScopedTokenClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*joiningv1.ScopedToken, error) {
	scopedToken, err := r.client.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name:       id.Name,
		Scope:      id.Scope,
		WithSecret: true,
	}.Build())
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
func (r ScopedTokenClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
