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

// NewUIConfigClient returns a Teleport client to interact with the web ui configuration.
func NewUIConfigClient(c *client.Client) UIConfigClient {
	return UIConfigClient{client: c}
}

// UIConfigClient manages the Teleport web ui configuration.
type UIConfigClient struct {
	client *client.Client
}

// Get reads the Teleport web ui configuration.
func (r UIConfigClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*types.UIConfigV1, error) {
	cfg, err := r.client.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiConfig, ok := cfg.(*types.UIConfigV1)
	if !ok {
		return nil, trace.BadParameter("unexpected UI config type %T", cfg)
	}

	return uiConfig, nil
}

// Create sets the Teleport web ui configuration.
func (r UIConfigClient) Create(ctx context.Context, cfg *types.UIConfigV1) error {
	return trace.Wrap(r.client.SetUIConfig(ctx, cfg))
}

// Upsert sets the Teleport web ui configuration.
func (r UIConfigClient) Upsert(ctx context.Context, cfg *types.UIConfigV1) error {
	return trace.Wrap(r.client.SetUIConfig(ctx, cfg))
}

// Delete removes the Teleport web ui configuration.
func (r UIConfigClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.DeleteUIConfig(ctx))
}
