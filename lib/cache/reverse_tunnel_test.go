// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestReverseTunnels tests reverse tunnels caching
func TestReverseTunnels(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.ReverseTunnel]{
		newResource: func(name string) (types.ReverseTunnel, error) {
			return types.NewReverseTunnel(name, []string{"example.com:2023"})
		},
		create: func(ctx context.Context, tunnel types.ReverseTunnel) error {
			_, err := p.presenceS.UpsertReverseTunnel(ctx, tunnel)
			return err
		},
		list: func(ctx context.Context) ([]types.ReverseTunnel, error) {
			items, _, err := p.presenceS.ListReverseTunnels(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		update: func(ctx context.Context, tunnel types.ReverseTunnel) error {
			_, err := p.presenceS.UpsertReverseTunnel(ctx, tunnel)
			return err
		},
		deleteAll: p.presenceS.DeleteAllReverseTunnels,

		cacheList: func(ctx context.Context) ([]types.ReverseTunnel, error) {
			items, _, err := p.cache.ListReverseTunnels(ctx, 0, "")
			return items, trace.Wrap(err)
		},
	})
}
