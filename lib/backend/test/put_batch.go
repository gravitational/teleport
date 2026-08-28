/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package test

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	watchInitTimeout  = 10 * time.Second
	watchEventTimeout = 3 * time.Second
)

func runPutBatch(t *testing.T, newBackend Constructor) {
	bk, _, err := newBackend()
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	batcher, ok := bk.(backend.BatchPutter)
	if !ok {
		t.Skip("backend does not implement PutBatch; skipping PutBatch suite")
	}

	itemEqual := func(a, b backend.Item) bool {
		return a.Key.Compare(b.Key) == 0 &&
			a.Revision == b.Revision &&
			bytes.Equal(a.Value, b.Value) &&
			a.Expires.Sub(b.Expires) < time.Second && b.Expires.Sub(a.Expires) < time.Second
	}

	assertItemsEqual := func(t *testing.T, want, got []backend.Item) {
		t.Helper()
		require.Len(t, want, len(got))
		for i := range want {
			require.True(t, itemEqual(want[i], got[i]))
		}
	}

	buildWant := func(items []backend.Item, rev []string) []backend.Item {
		out := make([]backend.Item, 0, len(items))
		for i, it := range items {
			out = append(out, backend.Item{
				Key:      it.Key,
				Value:    it.Value,
				Revision: rev[i],
				Expires:  it.Expires,
			})
		}
		return out
	}

	newTestItems := func(prefix func(components ...string) backend.Key) []backend.Item {
		return []backend.Item{
			{Key: prefix("a"), Value: []byte("A"), Expires: time.Now().Add(1 * time.Hour)},
			{Key: prefix("b"), Value: []byte("B")},
			{Key: prefix("c"), Value: []byte("C"), Expires: time.Now().Add(2 * time.Hour)},
		}
	}
	t.Run("put batch items should be propagated in event stream", func(t *testing.T) {
		prefix := MakePrefix()
		rangeStart := prefix("")
		rangeEnd := backend.RangeEnd(rangeStart)

		w, err := bk.NewWatcher(t.Context(), backend.Watch{})
		require.NoError(t, err)
		t.Cleanup(func() { w.Close() })

		select {
		case <-w.Done():
			t.Fatal("watcher closed immediately")
		case ev := <-w.Events():
			require.Equal(t, types.OpInit, ev.Type)
		}

		items := newTestItems(prefix)
		slices.SortFunc(items, func(a, b backend.Item) int { return a.Key.Compare(b.Key) })
		rev, err := batcher.PutBatch(t.Context(), items)
		require.NoError(t, err)
		require.NotEmpty(t, rev)

		got := waitForEvents(t, w, len(items), rangeStart, rangeEnd, watchEventTimeout)
		slices.SortFunc(got, func(a, b backend.Item) int { return a.Key.Compare(b.Key) })
		want := buildWant(items, rev)
		assertItemsEqual(t, want, got)
		require.NoError(t, bk.DeleteRange(t.Context(), rangeStart, rangeEnd))
	})

	t.Run("put-create-update", func(t *testing.T) {
		prefix := MakePrefix()
		rangeStart := prefix("")
		rangeEnd := backend.RangeEnd(rangeStart)

		items := newTestItems(prefix)
		rev1, err := batcher.PutBatch(t.Context(), items)
		require.NoError(t, err)
		require.NotEmpty(t, rev1)

		res, err := bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)

		want := buildWant(items, rev1)
		got := res.Items
		assertItemsEqual(t, want, got)

		items[0].Value = []byte("A2")
		items[1].Value = []byte("B2")
		items[2].Value = []byte("C2")

		rev2, err := batcher.PutBatch(t.Context(), items)
		require.NoError(t, err)
		require.NotEmpty(t, rev2)
		require.NotEqual(t, rev1, rev2)

		res, err = bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)

		want = buildWant(items, rev2)
		got = res.Items
		assertItemsEqual(t, want, got)

		require.NoError(t, bk.DeleteRange(t.Context(), rangeStart, rangeEnd))
	})

	t.Run("over-chunk-size", func(t *testing.T) {
		prefix := MakePrefix()
		rangeStart := prefix("")
		rangeEnd := backend.RangeEnd(prefix(""))

		const itemCount = 1000
		const payloadSize = 300 * 1024 // 300 KiB
		items := make([]backend.Item, 0, itemCount)
		for i := 0; i < itemCount; i++ {
			items = append(items, backend.Item{
				Key:     prefix(fmt.Sprintf("item/%04d", i)),
				Value:   bytes.Repeat([]byte("A"), payloadSize),
				Expires: time.Now().Add(5 * time.Minute),
			})
		}

		rev, err := batcher.PutBatch(t.Context(), items)
		require.NoError(t, err)
		require.NotEmpty(t, rev)

		res, err := bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)

		want := buildWant(items, rev)
		got := res.Items
		assertItemsEqual(t, want, got)

		require.NoError(t, bk.DeleteRange(t.Context(), rangeStart, rangeEnd))
	})
}

func waitForEvents(t *testing.T, w backend.Watcher, wantCount int, rangeStart backend.Key, rangeEnd backend.Key, timeout time.Duration) []backend.Item {
	t.Helper()

	var out []backend.Item
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for len(out) < wantCount {
		select {
		case ev := <-w.Events():
			if ev.Type == types.OpPut && rangeStart.Compare(ev.Item.Key) <= 0 && ev.Item.Key.Compare(rangeEnd) <= 0 {
				out = append(out, ev.Item)
			}
		case <-w.Done():
			t.Fatalf("watcher done before receiving all events: got=%d want=%d", len(out), wantCount)
		case <-deadline.C:
			t.Fatalf("hit timeout before receiving all events: got=%d want=%d", len(out), wantCount)
		}
	}
	return out
}
