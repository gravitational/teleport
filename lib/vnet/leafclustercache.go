// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils"
)

type leafClusterCache struct {
	fnCache *utils.FnCache
}

func newLeafClusterCache(clock clockwork.Clock) (*leafClusterCache, error) {
	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         5 * time.Minute,
		Clock:       clock,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &leafClusterCache{
		fnCache: fnCache,
	}, nil
}

func (c *leafClusterCache) getLeafClusters(ctx context.Context, rootClient ClusterClient) ([]string, error) {
	return utils.FnCacheGet(ctx, c.fnCache, rootClient.ClusterName(), func(ctx context.Context) ([]string, error) {
		return c.getLeafClustersUncached(ctx, rootClient)
	})
}

func (c *leafClusterCache) getLeafClustersUncached(ctx context.Context, rootClient ClusterClient) ([]string, error) {
	var leafClusters []string
	nextPage := ""
	for {
		remoteClusters, nextPage, err := rootClient.CurrentCluster().ListRemoteClusters(ctx, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, rc := range remoteClusters {
			leafClusters = append(leafClusters, rc.GetName())
		}
		if nextPage == "" {
			return leafClusters, nil
		}
	}
}
