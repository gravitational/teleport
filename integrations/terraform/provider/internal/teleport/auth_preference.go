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

// NewAuthPreferenceClient returns a Teleport client to interact with the cluster auth preference.
func NewAuthPreferenceClient(c *client.Client) AuthPreferenceClient {
	return AuthPreferenceClient{client: c}
}

// AuthPreferenceClient manages the Teleport cluster auth preference..
type AuthPreferenceClient struct {
	client *client.Client
}

// Get reads a the Teleport clust auth preference.
func (r AuthPreferenceClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*types.AuthPreferenceV2, error) {
	pref, err := r.client.ClusterConfigClient().GetAuthPreference(ctx, &clusterconfigv1.GetAuthPreferenceRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pref, nil
}

// Create creates the Teleport cluster auth preference.
func (r AuthPreferenceClient) Create(ctx context.Context, pref *types.AuthPreferenceV2) error {
	if _, err := r.client.UpsertAuthPreference(ctx, pref); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates the Teleport cluster auth preference.
func (r AuthPreferenceClient) Upsert(ctx context.Context, pref *types.AuthPreferenceV2) error {
	if _, err := r.client.UpsertAuthPreference(ctx, pref); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete resets the Teleport cluster auth preference.
func (r AuthPreferenceClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.ResetAuthPreference(ctx))
}
