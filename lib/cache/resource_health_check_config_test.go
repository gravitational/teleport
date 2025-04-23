/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
)

func newHealthCheckConfig(t *testing.T, name string) *healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	cfg, err := healthcheckconfig.NewHealthCheckConfig(name,
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabelsExpression: "labels.env == `test`",
			},
		},
	)
	require.NoError(t, err)
	return cfg
}

func TestHealthCheckConfig(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForAuth)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*healthcheckconfigv1.HealthCheckConfig]{
		newResource: func(name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
			return newHealthCheckConfig(t, name), nil
		},
		create: func(ctx context.Context, cfg *healthcheckconfigv1.HealthCheckConfig) error {
			_, err := p.healthCheckConfig.CreateHealthCheckConfig(ctx, cfg)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*healthcheckconfigv1.HealthCheckConfig, error) {
			items, _, err := p.healthCheckConfig.ListHealthCheckConfigs(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		update: func(ctx context.Context, cfg *healthcheckconfigv1.HealthCheckConfig) error {
			_, err := p.healthCheckConfig.UpdateHealthCheckConfig(ctx, cfg)
			return trace.Wrap(err)
		},
		deleteAll: p.healthCheckConfig.DeleteAllHealthCheckConfigs,
		cacheList: func(ctx context.Context) ([]*healthcheckconfigv1.HealthCheckConfig, error) {
			items, _, err := p.cache.ListHealthCheckConfigs(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheGet: p.cache.GetHealthCheckConfig,
	})
}
