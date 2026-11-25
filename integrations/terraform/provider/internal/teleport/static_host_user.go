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
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewStaticHostUserClient returns a static host user client.
func NewStaticHostUserClient(c *client.Client) StaticHostUserClient {
	return StaticHostUserClient{client: c}
}

// StaticHostUserClient manages static host user resources.
type StaticHostUserClient struct {
	client *client.Client
}

// Get reads a static host user by name.
func (r StaticHostUserClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*userprovisioningv2.StaticHostUser, error) {
	staticHostUser, err := r.client.GetStaticHostUser(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return staticHostUser, nil
}

// Create creates a static host user.
func (r StaticHostUserClient) Create(ctx context.Context, staticHostUser *userprovisioningv2.StaticHostUser) error {
	if _, err := r.client.CreateStaticHostUser(ctx, staticHostUser); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a static host user.
func (r StaticHostUserClient) Upsert(ctx context.Context, staticHostUser *userprovisioningv2.StaticHostUser) error {
	if _, err := r.client.UpsertStaticHostUser(ctx, staticHostUser); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a static host user by name.
func (r StaticHostUserClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteStaticHostUser(ctx, id.Name))
}
