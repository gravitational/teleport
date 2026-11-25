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
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewHealthCheckConfigClient returns a health check config client.
func NewHealthCheckConfigClient(c *client.Client) HealthCheckConfigClient {
	return HealthCheckConfigClient{client: c}
}

// HealthCheckConfigClient manages health check config resources.
type HealthCheckConfigClient struct {
	client *client.Client
}

// Get reads a health check config by name.
func (r HealthCheckConfigClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*healthcheckconfigv1.HealthCheckConfig, error) {
	healthCheckConfig, err := r.client.GetHealthCheckConfig(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return healthCheckConfig, nil
}

// Create creates a health check config.
func (r HealthCheckConfigClient) Create(ctx context.Context, healthCheckConfig *healthcheckconfigv1.HealthCheckConfig) error {
	if _, err := r.client.CreateHealthCheckConfig(ctx, healthCheckConfig); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a health check config.
func (r HealthCheckConfigClient) Upsert(ctx context.Context, healthCheckConfig *healthcheckconfigv1.HealthCheckConfig) error {
	if _, err := r.client.UpsertHealthCheckConfig(ctx, healthCheckConfig); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a health check config by name.
func (r HealthCheckConfigClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteHealthCheckConfig(ctx, id.Name))
}
