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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestWindowsDesktops tests that CRUD operations on
// windows desktop resources are replicated from the backend to the cache.
func TestWindowsDesktop(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.WindowsDesktop]{
		newResource: func(name string) (types.WindowsDesktop, error) {
			return types.NewWindowsDesktopV3(
				name,
				nil,
				types.WindowsDesktopSpecV3{
					Addr:   "localhost:123",
					HostID: "123e",
				},
			)
		},
		create: p.windowsDesktops.CreateWindowsDesktop,
		list: func(ctx context.Context) ([]types.WindowsDesktop, error) {
			var start string
			var desktops []types.WindowsDesktop
			for {
				req := types.ListWindowsDesktopsRequest{
					StartKey: start,
				}

				resp, err := p.windowsDesktops.ListWindowsDesktops(ctx, req)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				desktops = append(desktops, resp.Desktops...)
				start = resp.NextKey
				if resp.NextKey == "" {
					break
				}
			}

			return desktops, nil
		},
		cacheGet: func(ctx context.Context, s string) (types.WindowsDesktop, error) {
			desktops, err := p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: s})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if len(desktops) == 0 {
				return nil, trace.NotFound("desktop %q not found", s)
			}

			return desktops[0], nil

		},
		cacheList: func(ctx context.Context) ([]types.WindowsDesktop, error) {
			var start string
			var desktops []types.WindowsDesktop
			for {
				req := types.ListWindowsDesktopsRequest{
					StartKey: start,
				}

				resp, err := p.cache.ListWindowsDesktops(ctx, req)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				desktops = append(desktops, resp.Desktops...)
				start = resp.NextKey
				if resp.NextKey == "" {
					break
				}
			}

			return desktops, nil
		},
		update:    p.windowsDesktops.UpdateWindowsDesktop,
		deleteAll: p.windowsDesktops.DeleteAllWindowsDesktops,
	})

	wd1, err := types.NewWindowsDesktopV3(
		"test",
		nil,
		types.WindowsDesktopSpecV3{
			Addr:   "localhost:123",
			HostID: "b",
		},
	)
	require.NoError(t, err)

	wd2, err := types.NewWindowsDesktopV3(
		"test",
		nil,
		types.WindowsDesktopSpecV3{
			Addr:   "localhost:123",
			HostID: "a",
		},
	)
	require.NoError(t, err)

	wd3, err := types.NewWindowsDesktopV3(
		"fox",
		nil,
		types.WindowsDesktopSpecV3{
			Addr:   "localhost:123",
			HostID: "a",
		},
	)
	require.NoError(t, err)

	require.NoError(t, p.windowsDesktops.CreateWindowsDesktop(t.Context(), wd1))
	require.NoError(t, p.windowsDesktops.CreateWindowsDesktop(t.Context(), wd2))
	require.NoError(t, p.windowsDesktops.CreateWindowsDesktop(t.Context(), wd3))

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := p.cache.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Desktops, 3)
	}, 10*time.Second, 100*time.Millisecond)

	out, err := p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, out, 3)

	out, err = p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{HostID: "a", Name: "test"})
	require.NoError(t, err)
	require.Len(t, out, 1)

	out, err = p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{HostID: "a"})
	require.NoError(t, err)
	require.Len(t, out, 2)

	out, err = p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{HostID: "b"})
	require.NoError(t, err)
	require.Len(t, out, 1)

	out, err = p.cache.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: "test"})
	require.NoError(t, err)
	require.Len(t, out, 2)
}

// TestWindowsDesktopService tests that CRUD operations on
// windows desktop service resources are replicated from the backend to the cache.
func TestWindowsDesktopService(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	t.Run("GetWindowsDesktopServices", func(t *testing.T) {
		testResources(t, p, testFuncs[types.WindowsDesktopService]{
			newResource: func(name string) (types.WindowsDesktopService, error) {
				return types.NewWindowsDesktopServiceV3(
					types.Metadata{
						Name: name,
					},
					types.WindowsDesktopServiceSpecV3{
						Addr:            "localhost:123",
						TeleportVersion: "1.2.3",
					},
				)
			},
			create:    withKeepalive(p.presenceS.UpsertWindowsDesktopService),
			list:      p.presenceS.GetWindowsDesktopServices,
			cacheGet:  p.cache.GetWindowsDesktopService,
			cacheList: p.cache.GetWindowsDesktopServices,
			update:    withKeepalive(p.presenceS.UpsertWindowsDesktopService),
			deleteAll: p.presenceS.DeleteAllWindowsDesktopServices,
		})
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.WindowsDesktopService]{
			newResource: func(name string) (types.WindowsDesktopService, error) {
				return types.NewWindowsDesktopServiceV3(
					types.Metadata{
						Name: name,
					},
					types.WindowsDesktopServiceSpecV3{
						Addr:            "localhost:123",
						TeleportVersion: "1.2.3",
					},
				)
			},
			create: withKeepalive(p.presenceS.UpsertWindowsDesktopService),
			list: func(ctx context.Context) ([]types.WindowsDesktopService, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindWindowsDesktopService,
				}

				var out []types.WindowsDesktopService
				for {
					resp, err := p.presenceS.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.WindowsDesktopService))
					}

					req.StartKey = resp.NextKey
					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			cacheGet: p.cache.GetWindowsDesktopService,
			cacheList: func(ctx context.Context) ([]types.WindowsDesktopService, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindWindowsDesktopService,
				}

				var out []types.WindowsDesktopService
				for {
					resp, err := p.cache.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.WindowsDesktopService))
					}

					req.StartKey = resp.NextKey
					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			update:    withKeepalive(p.presenceS.UpsertWindowsDesktopService),
			deleteAll: p.presenceS.DeleteAllWindowsDesktopServices,
		})
	})
}

func TestDynamicWindowsDesktop(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.DynamicWindowsDesktop]{
		newResource: func(name string) (types.DynamicWindowsDesktop, error) {
			return types.NewDynamicWindowsDesktopV1(name,
				nil,
				types.DynamicWindowsDesktopSpecV1{
					Addr: "localhost:123",
				},
			)
		},
		create: func(ctx context.Context, dwd types.DynamicWindowsDesktop) error {
			_, err := p.dynamicWindowsDesktops.CreateDynamicWindowsDesktop(ctx, dwd)
			return err
		},
		list: func(ctx context.Context) ([]types.DynamicWindowsDesktop, error) {
			return stream.Collect(clientutils.Resources(ctx, p.dynamicWindowsDesktops.ListDynamicWindowsDesktops))
		},
		cacheGet: p.cache.GetDynamicWindowsDesktop,
		cacheList: func(ctx context.Context) ([]types.DynamicWindowsDesktop, error) {
			return stream.Collect(clientutils.Resources(ctx, p.cache.ListDynamicWindowsDesktops))
		},
		update: func(ctx context.Context, dwd types.DynamicWindowsDesktop) error {
			_, err := p.dynamicWindowsDesktops.UpdateDynamicWindowsDesktop(ctx, dwd)
			return err
		},
		deleteAll: p.dynamicWindowsDesktops.DeleteAllDynamicWindowsDesktops,
	})
}
