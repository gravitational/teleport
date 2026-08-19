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
