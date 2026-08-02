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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func pulseTestServer(name string) types.Server {
	return &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: name,
		},
	}
}

func pulseTestCollection(t *testing.T, fetched *[]types.Server) *collection[types.Server, nodeIndex] {
	t.Helper()
	return &collection[types.Server, nodeIndex]{
		store: newStore(
			types.KindNode,
			types.Server.DeepCopy,
			map[nodeIndex]func(types.Server) string{
				nodeNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			return *fetched, nil
		},
		watch: types.WatchKind{Kind: types.KindNode},
	}
}

func requireClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatal("expected change pulse to have fired")
	}
}

func requireNotClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("change pulse fired unexpectedly")
	default:
	}
}

func TestCollectionChangePulse(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fetched := []types.Server{pulseTestServer("seed")}
	col := pulseTestCollection(t, &fetched)

	// Unlistened commits are a no-op for the pulse machinery.
	require.NoError(t, col.OnPuts([]types.Resource{pulseTestServer("a")}))
	require.Nil(t, col.changed.Load())

	// An armed channel fires on a put.
	ch := col.changedSignal()
	requireNotClosed(t, ch)
	require.NoError(t, col.OnPuts([]types.Resource{pulseTestServer("b")}))
	requireClosed(t, ch)

	// Multiple listeners share one channel and are woken together; a burst
	// of commits collapses into a single fire.
	ch1 := col.changedSignal()
	ch2 := col.changedSignal()
	require.Equal(t, ch1, ch2)
	require.NoError(t, col.OnPuts([]types.Resource{pulseTestServer("c")}))
	require.NoError(t, col.OnDeletes([]types.Resource{pulseTestServer("b")}))
	requireClosed(t, ch1)
	requireClosed(t, ch2)

	// After firing, the pulse must be re-armed: commits with no listener do
	// not accumulate.
	require.Nil(t, col.changed.Load())

	// Deletes fire an armed channel.
	ch = col.changedSignal()
	require.NoError(t, col.OnDeletes([]types.Resource{pulseTestServer("c")}))
	requireClosed(t, ch)

	// The fetch/replace path (cache init and reset) fires as well.
	ch = col.changedSignal()
	apply, err := col.Fetch(ctx, true /* cacheOK */)
	require.NoError(t, err)
	requireNotClosed(t, ch)
	require.NoError(t, apply(ctx))
	requireClosed(t, ch)

	// The clear path (kind not cached in this generation) fires too.
	ch = col.changedSignal()
	apply, err = col.Fetch(ctx, false /* cacheOK */)
	require.NoError(t, err)
	require.NoError(t, apply(ctx))
	requireClosed(t, ch)
}
