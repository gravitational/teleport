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

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// TestRemoteClusters tests remote clusters caching
func TestRemoteClusters(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetRemoteClusters", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.RemoteCluster]{
			newResource: func(name string) (types.RemoteCluster, error) {
				return types.NewRemoteCluster(name)
			},
			create: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.CreateRemoteCluster(ctx, rc)
				return err
			},
			list:      getAllAdapter(p.trustS.GetRemoteClusters),
			cacheGet:  p.cache.GetRemoteCluster,
			cacheList: getAllAdapter(p.cache.GetRemoteClusters),
			update: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.UpdateRemoteCluster(ctx, rc)
				return err
			},
			deleteAll: p.trustS.DeleteAllRemoteClusters,
		}, withSkipPaginationTest())
	})

	t.Run("ListRemoteClusters", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.RemoteCluster]{
			newResource: func(name string) (types.RemoteCluster, error) {
				return types.NewRemoteCluster(name)
			},
			create: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.CreateRemoteCluster(ctx, rc)
				return err
			},
			list:      getAllAdapter(p.trustS.GetRemoteClusters),
			cacheGet:  p.cache.GetRemoteCluster,
			cacheList: getAllAdapter(p.cache.GetRemoteClusters),
			update: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.UpdateRemoteCluster(ctx, rc)
				return err
			},
			deleteAll: p.trustS.DeleteAllRemoteClusters,
		}, withSkipPaginationTest())

		// TODO(smallinsky): Remove this once pagination tests covering this case for each resource type
		// have been merged into v17.
		t.Run("test cluster get/update", func(t *testing.T) {
			item, err := types.NewRemoteCluster("test-cluster")
			require.NoError(t, err)

			_, err = p.trustS.CreateRemoteCluster(context.Background(), item)
			require.NoError(t, err)

			var itemFromCache types.RemoteCluster
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				var err error
				itemFromCache, err = p.cache.GetRemoteCluster(context.Background(), "test-cluster")
				require.NoError(t, err)
			}, 2*time.Second, time.Millisecond*40)

			itemFromCache.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
			_, err = p.trustS.UpdateRemoteCluster(context.Background(), itemFromCache)
			require.NoError(t, err)
		})
	})
}
