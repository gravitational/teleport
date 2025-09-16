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
	"cmp"
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestApps tests that CRUD operations on application resources are
// replicated from the backend to the cache.
func TestApps(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Application]{
		newResource: func(name string) (types.Application, error) {
			return types.NewAppV3(types.Metadata{
				Name: name,
			}, types.AppSpecV3{
				URI: "localhost",
			})
		},
		create: p.apps.CreateApp,
		list: func(ctx context.Context) ([]types.Application, error) {
			return stream.Collect(p.apps.Apps(ctx, "", ""))
		},
		cacheGet: p.cache.GetApp,
		cacheList: func(ctx context.Context, pageSize int) ([]types.Application, error) {
			return stream.Collect(p.cache.Apps(ctx, "", ""))
		},
		update:    p.apps.UpdateApp,
		deleteAll: p.apps.DeleteAllApps,
	})
}

func TestApplicationPagination(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	var expected []types.Application
	for i := range 1324 {
		app, err := types.NewAppV3(types.Metadata{
			Name: "app" + strconv.Itoa(i+1),
		}, types.AppSpecV3{
			URI: "localhost",
		})
		require.NoError(t, err)

		require.NoError(t, p.apps.CreateApp(t.Context(), app))
		expected = append(expected, app)
	}
	slices.SortFunc(expected, func(a, b types.Application) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})

	// Drain events to prevent deadlocking. Required because the number
	// of applications exceeds the default buffer size for the channel.
	drainEvents(p.eventsC)

	// Wait for all the applications to be replicated to the cache.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.Equal(t, len(expected), p.cache.collections.apps.store.len())
	}, 15*time.Second, 100*time.Millisecond)

	out, err := p.cache.GetApps(t.Context())
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, gocmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	page1, page2Start, err := p.cache.ListApps(t.Context(), 0, "")
	require.NoError(t, err)
	assert.Len(t, page1, 1000)
	assert.NotEmpty(t, page2Start)

	page2, next, err := p.cache.ListApps(t.Context(), 1000, page2Start)
	require.NoError(t, err)
	assert.Len(t, page2, len(expected)-1000)
	assert.Empty(t, next)

	listed := append(page1, page2...)
	assert.Empty(t, gocmp.Diff(expected, listed,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	var iterOut []types.Application
	for app, err := range p.cache.Apps(t.Context(), "", page2Start) {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	assert.Len(t, iterOut, len(page1))
	assert.Empty(t, gocmp.Diff(page1, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	iterOut = nil
	for app, err := range p.cache.Apps(t.Context(), "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	assert.Len(t, iterOut, len(expected))
	assert.Empty(t, gocmp.Diff(expected, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	iterOut = nil
	for app, err := range p.cache.Apps(t.Context(), page2Start, "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}

	assert.Len(t, iterOut, len(expected)-1000)
	assert.Empty(t, gocmp.Diff(expected, append(page1, iterOut...),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
}

// TestApplicationServers tests that CRUD operations on app servers are
// replicated from the backend to the cache.
func TestApplicationServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetApplicationServers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.AppServer]{
			newResource: func(name string) (types.AppServer, error) {
				app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "localhost"})
				require.NoError(t, err)
				return types.NewAppServerV3FromApp(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertApplicationServer),
			list: func(ctx context.Context) ([]types.AppServer, error) {
				return p.presenceS.GetApplicationServers(ctx, apidefaults.Namespace)
			},
			cacheList: func(ctx context.Context, pageSize int) ([]types.AppServer, error) {
				return p.cache.GetApplicationServers(ctx, apidefaults.Namespace)
			},
			update: withKeepalive(p.presenceS.UpsertApplicationServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		})
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.AppServer]{
			newResource: func(name string) (types.AppServer, error) {
				app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "localhost"})
				require.NoError(t, err)
				return types.NewAppServerV3FromApp(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertApplicationServer),
			list: func(ctx context.Context) ([]types.AppServer, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindAppServer,
				}

				var out []types.AppServer
				for {
					resp, err := p.presenceS.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.AppServer))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			cacheList: func(ctx context.Context, pageSize int) ([]types.AppServer, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindAppServer,
					Limit:        int32(pageSize),
				}

				var out []types.AppServer
				for {
					resp, err := p.cache.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.AppServer))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil

			},
			update: withKeepalive(p.presenceS.UpsertApplicationServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		})
	})

}
