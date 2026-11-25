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
