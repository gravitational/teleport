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

// Package sortcache is a prototype v2 of lib/utils/sortcache: a multi-index,
// in-memory cache of sortable values.
//
// Differences from v1:
//
//   - Reads operate on immutable snapshots published through an atomic
//     pointer. Taking a snapshot is wait-free, iteration holds no locks, and
//     a snapshot observes a single consistent generation regardless of
//     concurrent writes (v1's chunked iteration can miss or duplicate items
//     whose keys move mid-iteration).
//
//   - Writes are serialized and use btree copy-on-write clones: each commit
//     clones the published trees (O(1) per tree), applies its mutations
//     (copying the O(log n) node path per mutation), and atomically publishes
//     the result. Batched writes ([SortCache.Put] is variadic) amortize the
//     per-commit overhead.
//
//   - Indexes are typed handles ([Index]) with typed keys, including compound
//     tuple keys ([Tuple2], [Tuple3]) that replace collision-prone string
//     concatenation, and typed prefix bounds ([Prefix2]) that replace
//     NextKey/trailing-separator conventions.
//
//   - Values are stored directly in tree entries; the v1 ref-counter/values
//     map indirection is gone.
//
// The uniqueness/eviction contract is unchanged from v1: all indexes are
// unique, and a Put that collides with existing values on any index fully
// evicts those values. Secondary indexes should include the primary key as a
// trailing tuple component to make cross-value collisions impossible.
package sortcache

import (
	"iter"
	"sync"
	"sync/atomic"
)

// Config configures a [SortCache].
type Config[T any] struct {
	// Indexes lists the indexes maintained by the cache, in order. The first
	// index is conventionally the primary (truly unique) index. The slice and
	// the index handles must not be modified after New.
	Indexes []AnyIndex[T]
}

// SortCache is a snapshot-based multi-index cache. Values that overlap in
// *any* index are treated as unique: a Put that matches an existing value on
// any index evicts the existing value entirely. Stored values are shared with
// readers and must be treated as immutable.
type SortCache[T any] struct {
	// mu serializes writers. Readers never take it.
	mu      sync.Mutex
	indexes []AnyIndex[T]
	snap    atomic.Pointer[Snapshot[T]]
}

// Snapshot is an immutable point-in-time view of a [SortCache]. It remains
// valid (and consistent) indefinitely; holding an old snapshot retains the
// memory of its generation's divergence from the current one. Reads against a
// Snapshot are performed through the typed [Index] handles registered with
// the cache.
type Snapshot[T any] struct {
	trees []indexTree[T]
}

// Len returns the number of values in the snapshot.
func (s *Snapshot[T]) Len() int {
	if len(s.trees) == 0 {
		return 0
	}
	// every value has exactly one entry in every index.
	return s.trees[0].len()
}

// New sets up a SortCache with the provided configuration.
func New[T any](cfg Config[T]) *SortCache[T] {
	if len(cfg.Indexes) == 0 {
		panic("sortcache: at least one index is required")
	}
	c := &SortCache[T]{indexes: cfg.Indexes}
	trees := make([]indexTree[T], len(cfg.Indexes))
	for i, ix := range cfg.Indexes {
		ix.setPos(i)
		trees[i] = ix.newTree()
	}
	c.snap.Store(&Snapshot[T]{trees: trees})
	return c
}

// Snapshot returns the current published view of the cache. Wait-free.
func (c *SortCache[T]) Snapshot() *Snapshot[T] {
	return c.snap.Load()
}

// Len returns the number of values in the current published view.
func (c *SortCache[T]) Len() int {
	return c.Snapshot().Len()
}

// Put inserts values as a single atomic commit, evicting any existing values
// that collide with them on any index. The evicted values are returned;
// callers that expect evictions per item to be zero (new value) or one
// (update) should treat additional evictions as a bug signal, exactly as with
// v1's eviction count.
func (c *SortCache[T]) Put(values ...T) (evicted []T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.cloneLocked()
	for _, v := range values {
		evicted = append(evicted, putTrees(next.trees, v)...)
	}
	c.snap.Store(next)
	return evicted
}

// Delete removes the stored values matching the given values' keys, if any,
// as a single atomic commit. Matching is attempted index by index in
// registration order; the first index that locates a stored value identifies
// it, and it is then removed from all indexes. Deleting by a partially
// populated value therefore works as long as the value produces a correct key
// for at least the first index it matches on.
func (c *SortCache[T]) Delete(values ...T) (deleted []T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.cloneLocked()
	for _, v := range values {
		for _, tr := range next.trees {
			if prev, ok := tr.collide(v); ok {
				for _, tr2 := range next.trees {
					tr2.remove(prev)
				}
				deleted = append(deleted, prev)
				break
			}
		}
	}
	c.snap.Store(next)
	return deleted
}

// Replace atomically replaces the entire contents of the cache with the
// provided values. The new generation is built aside and published with a
// single swap: readers observe either the complete old state or the complete
// new state, never an intermediate one. This is the bulk-load path for
// (re-)initialization.
func (c *SortCache[T]) Replace(values iter.Seq[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	trees := make([]indexTree[T], len(c.indexes))
	for i, ix := range c.indexes {
		trees[i] = ix.newTree()
	}
	for v := range values {
		putTrees(trees, v)
	}
	c.snap.Store(&Snapshot[T]{trees: trees})
}

// Clear atomically resets the cache to empty.
func (c *SortCache[T]) Clear() {
	c.Replace(func(func(T) bool) {})
}

// cloneLocked produces a mutable copy-on-write clone of the published
// snapshot. Must be called with c.mu held.
func (c *SortCache[T]) cloneLocked() *Snapshot[T] {
	cur := c.snap.Load()
	trees := make([]indexTree[T], len(cur.trees))
	for i, tr := range cur.trees {
		trees[i] = tr.clone()
	}
	return &Snapshot[T]{trees: trees}
}

// putTrees inserts v into every tree, fully evicting any existing values
// whose key on any index collides with v's. Insertion and collision detection
// share a single btree descent per index via insertReplacing.
//
// Cleanup of a displaced value skips every tree on which the displaced value
// shares v's key: in those trees the entry is (or will be) overwritten in
// place by insertReplacing, so an explicit delete would be pure waste — and
// COW deletion is the most expensive btree mutation (rebalancing clones
// sibling nodes). For the common case of an update that moves no index keys,
// the entire commit is one in-place ReplaceOrInsert per index and no deletes.
//
// The skip is also what makes cleanup safe: a removal only happens where the
// displaced value's key differs from v's, so it can never delete an entry
// just written for v.
func putTrees[T any](trees []indexTree[T], v T) (evicted []T) {
	for i, tr := range trees {
		prev, ok := tr.insertReplacing(v)
		if !ok {
			continue
		}
		for j, tr2 := range trees {
			if j != i && !tr2.sameKey(prev, v) {
				tr2.remove(prev)
			}
		}
		// a value that collides with v on several indexes is displaced once
		// per index; report it only once. keys are unique per index, so
		// primary-key equality identifies the same stored value.
		dup := false
		for _, e := range evicted {
			if trees[0].sameKey(e, prev) {
				dup = true
				break
			}
		}
		if !dup {
			evicted = append(evicted, prev)
		}
	}
	return evicted
}
