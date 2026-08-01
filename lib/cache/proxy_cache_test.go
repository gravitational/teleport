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

package cache

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

// TestProxyCache verifies the one-step proxy topology constructor: the view
// replicates from the upstream services and serves reads across its composed
// collections on both the cached and the upstream-fallback paths.
func TestProxyCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// the pack provides the upstream services; its own cache is not used.
	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	cfg := p.cacheConfig(ctx)
	cfg.EventsC = nil // the pack's cache owns the events channel

	pc, err := NewProxyCache(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() })

	// singleton collection reads.
	authPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		AllowLocalAuth:  types.NewBoolOption(true),
		MessageOfTheDay: "proxy cache view",
	})
	require.NoError(t, err)
	authPref, err = p.clusterConfigS.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	netCfg, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout: types.NewDuration(7 * time.Minute),
	})
	require.NoError(t, err)
	netCfg, err = p.clusterConfigS.UpsertClusterNetworkingConfig(ctx, netCfg)
	require.NoError(t, err)

	// non-singleton collection read (nodes).
	server := NewServer(types.KindNode, "proxy-cache-node", "127.0.0.1:2022", apidefaults.Namespace)
	_, err = p.presenceS.UpsertNode(ctx, server)
	require.NoError(t, err)

	ignoreRevision := cmpopts.IgnoreFields(types.Metadata{}, "Revision")

	// healthy path: the view reads from cache snapshots once replication
	// catches up.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		gotPref, err := pc.GetAuthPreference(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(authPref, gotPref, ignoreRevision))

		gotNet, err := pc.GetClusterNetworkingConfig(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(netCfg, gotNet, ignoreRevision))

		gotNode, err := pc.GetNode(ctx, apidefaults.Namespace, "proxy-cache-node")
		require.NoError(t, err)
		require.Equal(t, "proxy-cache-node", gotNode.GetName())
	}, 15*time.Second, 100*time.Millisecond)

	// unhealthy path: the view falls back to the upstream exactly like the
	// legacy read surface.
	pc.cache.setReadOK(false)
	gotPref, err := pc.GetAuthPreference(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(authPref, gotPref, ignoreRevision))

	gotNode, err := pc.GetNode(ctx, apidefaults.Namespace, "proxy-cache-node")
	require.NoError(t, err)
	require.Equal(t, "proxy-cache-node", gotNode.GetName())
	pc.cache.setReadOK(true)
}

// TestComposeProxyCacheRequiresWatchedKinds verifies the drift guard between
// the proxy composition and its watch set: composing the view over a cache
// missing a required collection is a construction-time error naming the
// missing collection. This cannot happen through NewProxyCache, which applies
// the proxy watch set itself.
func TestComposeProxyCacheRequiresWatchedKinds(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, func(cfg Config) Config {
		cfg.target = "test"
		cfg.Watches = []types.WatchKind{
			{Kind: types.KindClusterName},
		}
		return cfg
	})
	t.Cleanup(p.Close)

	_, err := composeProxyCache(p.cache)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.Contains(t, err.Error(), "does not watch")
}
