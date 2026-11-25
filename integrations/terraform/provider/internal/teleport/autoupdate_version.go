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
	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewAutoUpdateVersionClient returns a Teleport client to interact with the autoupdate version.
func NewAutoUpdateVersionClient(c *client.Client) AutoUpdateVersionClient {
	return AutoUpdateVersionClient{client: c}
}

// AutoUpdateVersionClient manages the Teleport autoupdate version.
type AutoUpdateVersionClient struct {
	client *client.Client
}

// Get reads the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*autoupdatev1.AutoUpdateVersion, error) {
	version, err := r.client.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// Create creates the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Create(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) error {
	if _, err := r.client.CreateAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Upsert(ctx context.Context, version *autoupdatev1.AutoUpdateVersion) error {
	if _, err := r.client.UpsertAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete removes the Teleport autoupdate version.
func (r AutoUpdateVersionClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.DeleteAutoUpdateVersion(ctx))
}
