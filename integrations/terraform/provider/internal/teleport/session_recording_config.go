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
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewSessionRecordingConfigClient returns a Teleport client to interact with the session recording configuration.
func NewSessionRecordingConfigClient(c *client.Client) SessionRecordingConfigClient {
	return SessionRecordingConfigClient{client: c}
}

// SessionRecordingConfigClient manages the Teleport session recording configuration.
type SessionRecordingConfigClient struct {
	client *client.Client
}

// Get reads the Teleport session recording configuration.
func (r SessionRecordingConfigClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*types.SessionRecordingConfigV2, error) {
	cfg, err := r.client.ClusterConfigClient().GetSessionRecordingConfig(ctx, &clusterconfigv1.GetSessionRecordingConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}

// Create creates the Teleport session recording configuration.
func (r SessionRecordingConfigClient) Create(ctx context.Context, cfg *types.SessionRecordingConfigV2) error {
	if _, err := r.client.UpsertSessionRecordingConfig(ctx, cfg); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates the Teleport session recording configuration.
func (r SessionRecordingConfigClient) Upsert(ctx context.Context, cfg *types.SessionRecordingConfigV2) error {
	if _, err := r.client.UpsertSessionRecordingConfig(ctx, cfg); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete resets the Teleport session recording configuration.
func (r SessionRecordingConfigClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.ResetSessionRecordingConfig(ctx))
}
