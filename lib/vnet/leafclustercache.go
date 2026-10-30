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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

type leafClusterCache struct {
	fnCache *utils.FnCache
	// last holds the last successfully fetched list per profile name, it is
	// returned when a refresh fails so that an expired profile doesn't drop
	// leaf clusters out of the VNet configuration. It is dropped when the
	// profile has no credentials at all, meaning the user logged out.
	last sync.Map // profile name -> []string
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

func (c *leafClusterCache) getLeafClusters(ctx context.Context, profileName string, app ClientApplication) ([]string, error) {
	leafClusters, err := c.fnCache.Get(ctx, profileName, func(ctx context.Context) ([]string, error) {
		var leafClusters []string
		err := app.WithCachedClient(ctx, profileName, "" /*leafClusterName*/, func(rootClient ClusterClient) error {
			var err error
			leafClusters, err = c.getLeafClustersUncached(ctx, rootClient)
			return err
		})
		if err != nil {
			return nil, trace.Wrap(err, "getting root cluster client from cache")
		}
		c.last.Store(profileName, leafClusters)
		return leafClusters, nil
	})
	if err == nil {
		return leafClusters, nil
	}
	if client.IsNoCredentialsError(err) {
		// The user logged out of the profile, remove the config.
		c.last.Delete(profileName)
		return nil, trace.Wrap(err)
	}
	last, ok := c.last.Load(profileName)
	if ok {
		log.WarnContext(ctx, "Failed to refresh leaf cluster list, using last known list", "profile", profileName, "error", err)
		return last.([]string), nil
	}
	return nil, trace.Wrap(err)
}

func (c *leafClusterCache) getLeafClustersUncached(ctx context.Context, rootClient ClusterClient) ([]string, error) {
	var leafClusters []string
	for rc, err := range clientutils.Resources(ctx, rootClient.CurrentCluster().ListRemoteClusters) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		leafClusters = append(leafClusters, rc.GetName())
	}
	return leafClusters, nil
}
