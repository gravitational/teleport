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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

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
			desktops, _, err := p.dynamicWindowsDesktops.ListDynamicWindowsDesktops(ctx, 0, "")
			return desktops, err
		},
		cacheGet: p.cache.GetDynamicWindowsDesktop,
		cacheList: func(ctx context.Context) ([]types.DynamicWindowsDesktop, error) {
			desktops, _, err := p.cache.ListDynamicWindowsDesktops(ctx, 0, "")
			return desktops, err
		},
		update: func(ctx context.Context, dwd types.DynamicWindowsDesktop) error {
			_, err := p.dynamicWindowsDesktops.UpdateDynamicWindowsDesktop(ctx, dwd)
			return err
		},
		deleteAll: p.dynamicWindowsDesktops.DeleteAllDynamicWindowsDesktops,
	})
}
