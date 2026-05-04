/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

func runDeleteBatch(t *testing.T, newBackend Constructor) {
	t.Helper()

	bk, _, err := newBackend()
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	deleter, ok := bk.(backend.BatchDeleter)
	if !ok {
		t.Skip("backend does not implement DeleteBatch; skipping DeleteBatch suite")
	}

	prefix := MakePrefix()
	rangeStart := prefix("")
	rangeEnd := backend.RangeEnd(prefix(""))

	putItems := func(t *testing.T, items []backend.Item) {
		t.Helper()
		for _, item := range items {
			_, err := bk.Put(t.Context(), item)
			require.NoError(t, err)
		}
	}

	newTestItems := func() []backend.Item {
		return []backend.Item{
			{Key: prefix("a"), Value: []byte("A"), Expires: time.Now().Add(1 * time.Hour)},
			{Key: prefix("b"), Value: []byte("B")},
			{Key: prefix("c"), Value: []byte("C"), Expires: time.Now().Add(2 * time.Hour)},
		}
	}

	t.Run("delete batch items should be propagated in event stream", func(t *testing.T) {
		items := newTestItems()
		putItems(t, items)

		w, err := bk.NewWatcher(t.Context(), backend.Watch{})
		require.NoError(t, err)
		t.Cleanup(func() { w.Close() })

		select {
		case <-w.Done():
			t.Fatal("watcher closed immediately")
		case ev := <-w.Events():
			require.Equal(t, types.OpInit, ev.Type)
		}

		keys := make([]backend.Key, 0, len(items))
		for _, item := range items {
			keys = append(keys, item.Key)
		}

		err = deleter.DeleteBatch(t.Context(), keys)
		require.NoError(t, err)

		got := waitForDeleteEvents(t, w, len(keys), watchEventTimeout)
		require.Len(t, got, len(keys))
		for i, key := range keys {
			require.Equal(t, 0, key.Compare(got[i]))
		}

		// Confirm nothing remains.
		res, err := bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)
		require.Empty(t, res.Items)
	})

	t.Run("delete-then-verify-empty", func(t *testing.T) {
		items := newTestItems()
		putItems(t, items)

		// Verify items exist.
		res, err := bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)
		require.Len(t, res.Items, len(items))

		keys := make([]backend.Key, 0, len(items))
		for _, item := range items {
			keys = append(keys, item.Key)
		}

		err = deleter.DeleteBatch(t.Context(), keys)
		require.NoError(t, err)

		// Verify items are gone.
		res, err = bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)
		require.Empty(t, res.Items)
	})

	t.Run("delete-nonexistent-keys", func(t *testing.T) {
		keys := []backend.Key{
			prefix("nonexistent1"),
			prefix("nonexistent2"),
			prefix("nonexistent3"),
		}

		// Deleting keys that don't exist should not error.
		err := deleter.DeleteBatch(t.Context(), keys)
		require.NoError(t, err)
	})

	t.Run("delete-empty-batch", func(t *testing.T) {
		err := deleter.DeleteBatch(t.Context(), nil)
		require.NoError(t, err)

		err = deleter.DeleteBatch(t.Context(), []backend.Key{})
		require.NoError(t, err)
	})

	t.Run("delete-partial-existing", func(t *testing.T) {
		items := newTestItems()
		putItems(t, items)

		// Delete a mix of existing and non-existing keys.
		keys := []backend.Key{
			items[0].Key,
			prefix("nonexistent"),
			items[2].Key,
		}

		err := deleter.DeleteBatch(t.Context(), keys)
		require.NoError(t, err)

		// Only the middle item should remain.
		res, err := bk.GetRange(t.Context(), rangeStart, rangeEnd, backend.NoLimit)
		require.NoError(t, err)
		require.Len(t, res.Items, 1)
		require.Equal(t, 0, items[1].Key.Compare(res.Items[0].Key))

		require.NoError(t, bk.DeleteRange(t.Context(), rangeStart, rangeEnd))
	})
}

func waitForDeleteEvents(t *testing.T, w backend.Watcher, wantCount int, timeout time.Duration) []backend.Key {
	t.Helper()

	var out []backend.Key
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for len(out) < wantCount {
		select {
		case ev, ok := <-w.Events():
			if !ok {
				t.Fatalf("watcher closed before receiving all events: got=%d want=%d", len(out), wantCount)
			}
			if ev.Type == types.OpDelete {
				out = append(out, ev.Item.Key)
			}
		case <-deadline.C:
			t.Fatalf("timed out waiting for delete events: got=%d want=%d", len(out), wantCount)
		case <-w.Done():
			t.Fatalf("watcher done before receiving all events: got=%d want=%d", len(out), wantCount)
		}
	}
	return out
}
