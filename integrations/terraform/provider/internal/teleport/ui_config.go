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
