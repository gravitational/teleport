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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetter(t *testing.T) {
	t.Parallel()
	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	store := newStore(
		func(role types.Role) types.Role {
			return role
		},
		map[string]func(types.Role) string{
			"default": types.Role.GetName,
		})

	require.NoError(t, store.put(&types.RoleV6{Metadata: types.Metadata{Name: "a"}}))

	var upstreamRead bool
	g := genericGetter[types.Role, string]{
		cache: p.cache,
		index: "default",
		upstreamGet: func(ctx context.Context, s string) (types.Role, error) {
			upstreamRead = true
			return &types.RoleV6{Metadata: types.Metadata{Name: "upstream-" + s}}, nil
		},
		collection: &collection[types.Role, string]{
			store: store,
			fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Role, error) {
				return []types.Role{
					&types.RoleV6{Metadata: types.Metadata{Name: "a"}},
					&types.RoleV6{Metadata: types.Metadata{Name: "b"}},
					&types.RoleV6{Metadata: types.Metadata{Name: "c"}},
				}, nil
			},
			headerTransform: func(hdr *types.ResourceHeader) types.Role {
				return &types.RoleV6{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				}
			},
			watch: types.WatchKind{Kind: types.KindRole},
		},
	}

	out, err := g.get(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, "a", out.GetName())
	assert.False(t, upstreamRead)

	p.cache.ok = false

	out, err = g.get(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, "upstream-a", out.GetName())
	assert.True(t, upstreamRead)
}

func TestLister(t *testing.T) {
	t.Parallel()
	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	store := newStore(
		func(role types.Role) types.Role {
			return role
		},
		map[string]func(types.Role) string{
			"default": types.Role.GetName,
		})

	require.NoError(t, store.put(&types.RoleV6{Metadata: types.Metadata{Name: "a"}}))

	var upstreamRead bool
	g := genericLister[types.Role, string]{
		cache: p.cache,
		index: "default",
		upstreamList: func(ctx context.Context, limit int, start string) ([]types.Role, string, error) {
			upstreamRead = true
			return []types.Role{&types.RoleV6{Metadata: types.Metadata{Name: "upstream-role"}}}, "", nil
		},
		collection: &collection[types.Role, string]{
			store: store,
			fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Role, error) {
				return []types.Role{
					&types.RoleV6{Metadata: types.Metadata{Name: "a"}},
					&types.RoleV6{Metadata: types.Metadata{Name: "b"}},
					&types.RoleV6{Metadata: types.Metadata{Name: "c"}},
				}, nil
			},
			headerTransform: func(hdr *types.ResourceHeader) types.Role {
				return &types.RoleV6{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				}
			},
			watch: types.WatchKind{Kind: types.KindRole},
		},
	}

	out, next, err := g.list(context.Background(), 10, "")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "a", out[0].GetName())
	assert.Empty(t, next)
	assert.False(t, upstreamRead)

	p.cache.ok = false

	out, next, err = g.list(context.Background(), 10, "")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "upstream-role", out[0].GetName())
	assert.Empty(t, next)
	assert.True(t, upstreamRead)
}
