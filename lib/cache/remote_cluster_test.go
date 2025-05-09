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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestRemoteClusters tests remote clusters caching
func TestRemoteClusters(t *testing.T) {
	t.Parallel()

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
			list: func(ctx context.Context) ([]types.RemoteCluster, error) {
				return p.trustS.GetRemoteClusters(ctx)
			},
			cacheGet: func(ctx context.Context, name string) (types.RemoteCluster, error) {
				return p.cache.GetRemoteCluster(ctx, name)
			},
			cacheList: func(ctx context.Context) ([]types.RemoteCluster, error) {
				return p.cache.GetRemoteClusters(ctx)
			},
			update: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.UpdateRemoteCluster(ctx, rc)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.trustS.DeleteAllRemoteClusters(ctx)
			},
		})
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
			list: func(ctx context.Context) ([]types.RemoteCluster, error) {
				return p.trustS.GetRemoteClusters(ctx)
			},
			cacheGet: func(ctx context.Context, name string) (types.RemoteCluster, error) {
				return p.cache.GetRemoteCluster(ctx, name)
			},
			cacheList: func(ctx context.Context) ([]types.RemoteCluster, error) {
				clusters, _, err := p.cache.ListRemoteClusters(ctx, 0, "")
				return clusters, err
			},
			update: func(ctx context.Context, rc types.RemoteCluster) error {
				_, err := p.trustS.UpdateRemoteCluster(ctx, rc)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.trustS.DeleteAllRemoteClusters(ctx)
			},
		})
	})
}

// TestTunnelConnections tests tunnel connections caching
func TestTunnelConnections(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	clusterName := "example.com"
	testResources(t, p, testFuncs[types.TunnelConnection]{
		newResource: func(name string) (types.TunnelConnection, error) {
			return types.NewTunnelConnection(name, types.TunnelConnectionSpecV2{
				ClusterName:   clusterName,
				ProxyName:     "p1",
				LastHeartbeat: time.Now().UTC(),
			})
		},
		create: modifyNoContext(p.trustS.UpsertTunnelConnection),
		list: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.trustS.GetAllTunnelConnections()
		},
		cacheList: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.cache.GetAllTunnelConnections()
		},
		update: modifyNoContext(p.trustS.UpsertTunnelConnection),
		deleteAll: func(ctx context.Context) error {
			return p.trustS.DeleteAllTunnelConnections()
		},
	})

	for i := 0; i < 17; i++ {
		tunnel, err := types.NewTunnelConnection("conn"+strconv.Itoa(i+1), types.TunnelConnectionSpecV2{
			ClusterName:   clusterName,
			ProxyName:     "p1",
			LastHeartbeat: time.Now().UTC(),
		})
		require.NoError(t, err)

		require.NoError(t, p.trustS.UpsertTunnelConnection(tunnel))
	}

	for i := 0; i < 3; i++ {
		tunnel, err := types.NewTunnelConnection("conn"+strconv.Itoa(i+100), types.TunnelConnectionSpecV2{
			ClusterName:   "other-cluster",
			ProxyName:     "p1",
			LastHeartbeat: time.Now().UTC(),
		})
		require.NoError(t, err)
		require.NoError(t, p.trustS.UpsertTunnelConnection(tunnel))
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		tunnels, err := p.cache.GetAllTunnelConnections()
		assert.NoError(tt, err)
		assert.Len(tt, tunnels, 20)

	}, 15*time.Second, 100*time.Millisecond)

	tunnels, err := p.cache.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Len(t, tunnels, 17)

	tunnels, err = p.cache.GetTunnelConnections("other-cluster")
	require.NoError(t, err)
	require.Len(t, tunnels, 3)
}
