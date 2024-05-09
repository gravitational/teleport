// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/lib/utils"
)

type getClusterConfigFunc = func(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error)

type clusterConfigCache struct {
	get      getClusterConfigFunc
	ttlCache *utils.FnCache
}

func newClusterConfigCache(get getClusterConfigFunc) (*clusterConfigCache, error) {
	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 5 * time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterConfigCache{
		get:      get,
		ttlCache: ttlCache,
	}, nil
}

func (c *clusterConfigCache) getVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error) {
	k := clusterCacheKey{
		profileName:     profileName,
		leafClusterName: leafClusterName,
	}
	vnetConfig, err := utils.FnCacheGet(ctx, c.ttlCache, k, func(ctx context.Context) (*vnet.VnetConfig, error) {
		return c.get(ctx, profileName, leafClusterName)
	})
	if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
		// Default to the empty config on NotFound for NotImplemented.
		return &vnet.VnetConfig{}, nil
	}
	return vnetConfig, trace.Wrap(err)
}

type clusterCacheKey struct {
	profileName     string
	leafClusterName string
}
