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
