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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
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

	p, err := newPack(t, ForAuth)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*healthcheckconfigv1.HealthCheckConfig]{
		newResource: func(name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
			return newHealthCheckConfig(t, name), nil
		},
		create: func(ctx context.Context, cfg *healthcheckconfigv1.HealthCheckConfig) error {
			_, err := p.healthCheckConfig.CreateHealthCheckConfig(ctx, cfg)
			return trace.Wrap(err)
		},
		list: filterHealthCfgNonVirtual(p.healthCheckConfig.ListHealthCheckConfigs),
		update: func(ctx context.Context, cfg *healthcheckconfigv1.HealthCheckConfig) error {
			_, err := p.healthCheckConfig.UpdateHealthCheckConfig(ctx, cfg)
			return trace.Wrap(err)
		},
		deleteAll: p.healthCheckConfig.DeleteAllHealthCheckConfigs,
		cacheList: filterHealthCfgNonVirtual(p.cache.ListHealthCheckConfigs),
		cacheGet:  p.cache.GetHealthCheckConfig,
	}, withSkipPaginationTest())
}

type listHealthCfgFunc func(ctx context.Context, pageSize int, pageToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error)

// filterHealthCfgNonVirtual excludes virtual defaults while maintaining pagination.
func filterHealthCfgNonVirtual(listFn listHealthCfgFunc) listHealthCfgFunc {
	return func(ctx context.Context, pageSize int, pageToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
		var allNonVirtual []*healthcheckconfigv1.HealthCheckConfig
		var token string
		for {
			items, nextPageToken, err := listFn(ctx, defaults.DefaultChunkSize, token)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			for _, item := range items {
				if !isVirtualDefaultHealthCheckConfig(item.GetMetadata().GetName()) {
					allNonVirtual = append(allNonVirtual, item)
				}
			}
			if nextPageToken == "" {
				break
			}
			token = nextPageToken
		}
		page, nextPageToken := pageHealthCfg(allNonVirtual, pageSize, pageToken)
		return page, nextPageToken, nil
	}
}

// pageHealthCfg creates a page from a slice.
func pageHealthCfg(
	items []*healthcheckconfigv1.HealthCheckConfig,
	pageSize int,
	pageToken string,
) ([]*healthcheckconfigv1.HealthCheckConfig, string) {
	if len(items) == 0 {
		return nil, ""
	}
	// look for the start index
	var idxStart int
	if pageToken != "" {
		for n, item := range items {
			if item.GetMetadata().GetName() == pageToken {
				idxStart = n + 1
				break
			}
		}
	}
	if idxStart >= len(items) {
		return nil, ""
	}
	// look for the end index
	idxEnd := len(items)
	if pageSize > 0 && idxStart+pageSize < len(items) {
		idxEnd = idxStart + pageSize
	}
	page := items[idxStart:idxEnd]
	var nextPageToken string
	if idxEnd < len(items) {
		nextPageToken = page[len(page)-1].GetMetadata().GetName()
	}
	return page, nextPageToken
}

func isVirtualDefaultHealthCheckConfig(name string) bool {
	switch name {
	case teleport.VirtualDefaultHealthCheckConfigDBName,
		teleport.VirtualDefaultHealthCheckConfigKubeName:
		return true
	}
	return false
}
