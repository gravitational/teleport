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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
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
				Name: "foo",
			}, types.AppSpecV3{
				URI: "localhost",
			})
		},
		create:    p.apps.CreateApp,
		list:      p.apps.GetApps,
		cacheGet:  p.cache.GetApp,
		cacheList: p.cache.GetApps,
		update:    p.apps.UpdateApp,
		deleteAll: p.apps.DeleteAllApps,
	})
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
			cacheList: func(ctx context.Context) ([]types.AppServer, error) {
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
			cacheList: func(ctx context.Context) ([]types.AppServer, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindAppServer,
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
