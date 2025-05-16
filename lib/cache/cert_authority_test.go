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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
)

// TestCA tests certificate authorities
func TestCA(t *testing.T) {
	t.Parallel()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)
	ctx := context.Background()

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ca, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	err = p.trustS.DeleteCertAuthority(ctx, ca.GetID())
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.True(t, trace.IsNotFound(err))
}

func TestNodeCAFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = p.clusterConfigS.UpsertClusterName(clusterName)
	require.NoError(t, err)

	nodeCacheBackend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeCacheBackend.Close()) })

	// this mimics a cache for a node pulling events from the auth server via WatchEvents
	nodeCache, err := New(ForNode(Config{
		Events:                  p.cache,
		Trust:                   p.cache.Trust,
		ClusterConfig:           p.cache.ClusterConfig,
		Access:                  p.cache.Access,
		DynamicAccess:           p.cache.DynamicAccess,
		Presence:                p.cache.Presence,
		Restrictions:            p.cache.Restrictions,
		SAMLIdPServiceProviders: p.cache.SAMLIdPServiceProviders,
		UserGroups:              p.cache.UserGroups,
		StaticHostUsers:         p.cache.StaticHostUsers,
		Backend:                 nodeCacheBackend,
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeCache.Close()) })

	cacheWatcher, err := nodeCache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
		{
			Kind:   types.KindCertAuthority,
			Filter: map[string]string{"host": "example.com", "user": "*"},
		},
	}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cacheWatcher.Close()) })

	fetchEvent := func() types.Event {
		var ev types.Event
		select {
		case ev = <-cacheWatcher.Events():
		case <-time.After(time.Second * 5):
			t.Fatal("watcher timeout")
		}
		return ev
	}
	require.Equal(t, types.OpInit, fetchEvent().Type)

	// upsert and delete a local host CA, we expect to see a Put and a Delete event
	localCA := suite.NewTestCA(types.HostCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, localCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, localCA.GetID()))

	ev := fetchEvent()
	require.Equal(t, types.OpPut, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.com", ev.Resource.GetName())

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.com", ev.Resource.GetName())

	// upsert and delete a nonlocal host CA, we expect to only see the Delete event
	nonlocalCA := suite.NewTestCA(types.HostCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, nonlocalCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, nonlocalCA.GetID()))

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())

	// whereas we expect to see the Put and Delete for a trusted *user* CA
	trustedUserCA := suite.NewTestCA(types.UserCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, trustedUserCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, trustedUserCA.GetID()))

	ev = fetchEvent()
	require.Equal(t, types.OpPut, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())
}

// TestCAWatcherFilters tests cache CA watchers with filters are not rejected
// by auth, even if a CA filter includes a "new" CA type.
func TestCAWatcherFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	allCAsAndNewCAFilter := makeAllKnownCAsFilter()
	// auth will never send such an event, but it won't reject the watch request
	// either since auth cache's confirmedKinds dont have a CA filter.
	allCAsAndNewCAFilter["someBackportedCAType"] = "*"

	tests := []struct {
		desc    string
		filter  types.CertAuthorityFilter
		watcher types.Watcher
	}{
		{
			desc: "empty filter",
		},
		{
			desc:   "all CAs filter",
			filter: makeAllKnownCAsFilter(),
		},
		{
			desc:   "all CAs and a new CA filter",
			filter: allCAsAndNewCAFilter,
		},
	}

	// setup watchers for each test case before we generate events.
	for i := range tests {
		test := &tests[i]
		w, err := p.cache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
			{
				Kind:   types.KindCertAuthority,
				Filter: test.filter.IntoMap(),
			},
		}})
		require.NoError(t, err)
		test.watcher = w
		t.Cleanup(func() {
			require.NoError(t, w.Close())
		})
	}

	// generate an OpPut event.
	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	const fetchTimeout = time.Second
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			event := fetchEvent(t, test.watcher, fetchTimeout)
			require.Equal(t, types.OpInit, event.Type)

			event = fetchEvent(t, test.watcher, fetchTimeout)
			require.Equal(t, types.OpPut, event.Type)
			require.Equal(t, types.KindCertAuthority, event.Resource.GetKind())
			gotCA, ok := event.Resource.(*types.CertAuthorityV2)
			require.True(t, ok)
			require.Equal(t, types.UserCA, gotCA.GetType())
		})
	}
}
