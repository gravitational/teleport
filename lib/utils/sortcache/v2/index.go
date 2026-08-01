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

package sortcache

import (
	"cmp"
	"fmt"
	"iter"

	"github.com/google/btree"
)

// Index is a typed handle describing one sort order of a [SortCache]. An
// Index is created once (typically as a package-level variable alongside the
// cache that uses it), registered with a cache via [Config.Indexes], and then
// used to perform typed reads against snapshots of that cache.
//
// An Index handle may be registered with multiple caches only if it occupies
// the same position in every cache's Config.Indexes slice; registration at a
// conflicting position panics.
type Index[T any, K any] struct {
	name string
	key  func(T) K
	less func(K, K) bool
	pos  int
}

// NewIndex creates an index whose key type has a natural ordering.
func NewIndex[T any, K cmp.Ordered](name string, key func(T) K) *Index[T, K] {
	return NewIndexFunc(name, key, func(a, b K) bool { return a < b })
}

// NewIndexFunc creates an index with a caller-supplied ordering. Prefer
// [NewIndex] or the tuple index constructors; this is the escape hatch for
// key types with bespoke comparison semantics.
func NewIndexFunc[T any, K any](name string, key func(T) K, less func(K, K) bool) *Index[T, K] {
	return &Index[T, K]{
		name: name,
		key:  key,
		less: less,
		pos:  -1,
	}
}

// Name returns the display name the index was created with.
func (ix *Index[T, K]) Name() string { return ix.name }

// KeyOf computes the key of the supplied value on this index.
func (ix *Index[T, K]) KeyOf(v T) K { return ix.key(v) }

// Get loads the value associated with the given key, if any.
func (ix *Index[T, K]) Get(s *Snapshot[T], key K) (value T, ok bool) {
	e, ok := ix.treeOf(s).bt.Get(entry[T, K]{key: key})
	return e.val, ok
}

// Bound describes one end of a range read. The zero value is an open bound.
// See [Open], [Inclusive], and [Exclusive].
type Bound[K any] struct {
	key  K
	kind boundKind
}

type boundKind int8

const (
	boundOpen boundKind = iota
	boundInclusive
	boundExclusive
)

// Open returns an unbounded range end.
func Open[K any]() Bound[K] { return Bound[K]{} }

// Inclusive returns a bound that includes the given key.
func Inclusive[K any](key K) Bound[K] { return Bound[K]{key: key, kind: boundInclusive} }

// Exclusive returns a bound that excludes the given key.
func Exclusive[K any](key K) Bound[K] { return Bound[K]{key: key, kind: boundExclusive} }

// Ascend iterates the snapshot from least to greatest between start and stop.
// The snapshot is immutable, so iteration is consistent, lock-free, and the
// yield function may be arbitrarily slow without blocking writes.
func (ix *Index[T, K]) Ascend(s *Snapshot[T], start, stop Bound[K]) iter.Seq[T] {
	tr := ix.treeOf(s)
	return func(yield func(T) bool) {
		emit := func(e entry[T, K]) bool { return yield(e.val) }

		// fast paths matching btree's native range primitives (also the
		// combinations used by all teleport range/pagination conventions).
		switch {
		case start.kind == boundOpen && stop.kind == boundOpen:
			tr.bt.Ascend(emit)
			return
		case start.kind == boundOpen && stop.kind == boundExclusive:
			tr.bt.AscendLessThan(entry[T, K]{key: stop.key}, emit)
			return
		case start.kind == boundInclusive && stop.kind == boundOpen:
			tr.bt.AscendGreaterOrEqual(entry[T, K]{key: start.key}, emit)
			return
		case start.kind == boundInclusive && stop.kind == boundExclusive:
			tr.bt.AscendRange(entry[T, K]{key: start.key}, entry[T, K]{key: stop.key}, emit)
			return
		}

		// general path for exclusive-start and/or inclusive-stop.
		check := func(e entry[T, K]) bool {
			switch stop.kind {
			case boundInclusive:
				if ix.less(stop.key, e.key) {
					return false
				}
			case boundExclusive:
				if !ix.less(e.key, stop.key) {
					return false
				}
			}
			return yield(e.val)
		}
		switch start.kind {
		case boundOpen:
			tr.bt.Ascend(check)
		case boundInclusive:
			tr.bt.AscendGreaterOrEqual(entry[T, K]{key: start.key}, check)
		case boundExclusive:
			skipped := false
			tr.bt.AscendGreaterOrEqual(entry[T, K]{key: start.key}, func(e entry[T, K]) bool {
				if !skipped {
					skipped = true
					if !ix.less(start.key, e.key) {
						// entry equal to the exclusive start bound.
						return true
					}
				}
				return check(e)
			})
		}
	}
}

// Descend iterates the snapshot from greatest to least between start and
// stop. Note that in descending order start is the *upper* end of the range;
// start-inclusive/stop-exclusive semantics still apply.
func (ix *Index[T, K]) Descend(s *Snapshot[T], start, stop Bound[K]) iter.Seq[T] {
	tr := ix.treeOf(s)
	return func(yield func(T) bool) {
		emit := func(e entry[T, K]) bool { return yield(e.val) }

		switch {
		case start.kind == boundOpen && stop.kind == boundOpen:
			tr.bt.Descend(emit)
			return
		case start.kind == boundOpen && stop.kind == boundExclusive:
			tr.bt.DescendGreaterThan(entry[T, K]{key: stop.key}, emit)
			return
		case start.kind == boundInclusive && stop.kind == boundOpen:
			tr.bt.DescendLessOrEqual(entry[T, K]{key: start.key}, emit)
			return
		case start.kind == boundInclusive && stop.kind == boundExclusive:
			tr.bt.DescendRange(entry[T, K]{key: start.key}, entry[T, K]{key: stop.key}, emit)
			return
		}

		check := func(e entry[T, K]) bool {
			switch stop.kind {
			case boundInclusive:
				if ix.less(e.key, stop.key) {
					return false
				}
			case boundExclusive:
				if !ix.less(stop.key, e.key) {
					return false
				}
			}
			return yield(e.val)
		}
		switch start.kind {
		case boundOpen:
			tr.bt.Descend(check)
		case boundInclusive:
			tr.bt.DescendLessOrEqual(entry[T, K]{key: start.key}, check)
		case boundExclusive:
			skipped := false
			tr.bt.DescendLessOrEqual(entry[T, K]{key: start.key}, func(e entry[T, K]) bool {
				if !skipped {
					skipped = true
					if !ix.less(e.key, start.key) {
						return true
					}
				}
				return check(e)
			})
		}
	}
}

// Count returns the number of values in the given range. O(range).
func (ix *Index[T, K]) Count(s *Snapshot[T], start, stop Bound[K]) int {
	var n int
	for range ix.Ascend(s, start, stop) {
		n++
	}
	return n
}

func (ix *Index[T, K]) treeOf(s *Snapshot[T]) *tree[T, K] {
	if ix.pos < 0 || ix.pos >= len(s.trees) {
		panic(fmt.Sprintf("sortcache: index %q is not registered with this cache", ix.name))
	}
	tr, ok := s.trees[ix.pos].(*tree[T, K])
	if !ok || tr.ix != ix {
		panic(fmt.Sprintf("sortcache: index %q does not belong to this cache", ix.name))
	}
	return tr
}

// AnyIndex is the type-erased view of an [Index] used at cache construction.
// It is implemented only by *Index.
type AnyIndex[T any] interface {
	newTree() indexTree[T]
	setPos(int)
}

func (ix *Index[T, K]) setPos(p int) {
	if ix.pos != -1 && ix.pos != p {
		panic(fmt.Sprintf("sortcache: index %q registered with conflicting positions %d and %d (an index handle may only be shared by caches with identical index layouts)", ix.name, ix.pos, p))
	}
	ix.pos = p
}

func (ix *Index[T, K]) newTree() indexTree[T] {
	const bTreeDegree = 8 // standard across the teleport codebase
	return &tree[T, K]{
		ix: ix,
		bt: btree.NewG(bTreeDegree, func(a, b entry[T, K]) bool {
			return ix.less(a.key, b.key)
		}),
	}
}

// entry is a single item in an index tree: the precomputed key and the value.
// Unlike v1 there is no ref/values-map indirection; the value is stored (by
// reference, for pointer-typed T) once per index.
type entry[T, K any] struct {
	key K
	val T
}

// indexTree is the type-erased view of one index's btree that the cache core
// uses to drive writes without knowing the index's key type.
type indexTree[T any] interface {
	// clone returns a lazy copy-on-write copy of the tree.
	clone() indexTree[T]
	// insertReplacing adds v to the tree under its computed key, returning
	// the existing value it displaced, if any.
	insertReplacing(v T) (T, bool)
	// collide returns the existing value occupying v's key, if any.
	collide(v T) (T, bool)
	// sameKey reports whether a and b produce equal keys on this index.
	sameKey(a, b T) bool
	// remove deletes the entry at v's computed key.
	remove(v T)
	// len returns the number of entries.
	len() int
}

type tree[T any, K any] struct {
	ix *Index[T, K]
	bt *btree.BTreeG[entry[T, K]]
}

func (t *tree[T, K]) clone() indexTree[T] {
	return &tree[T, K]{ix: t.ix, bt: t.bt.Clone()}
}

func (t *tree[T, K]) insertReplacing(v T) (T, bool) {
	prev, ok := t.bt.ReplaceOrInsert(entry[T, K]{key: t.ix.key(v), val: v})
	return prev.val, ok
}

func (t *tree[T, K]) collide(v T) (T, bool) {
	e, ok := t.bt.Get(entry[T, K]{key: t.ix.key(v)})
	return e.val, ok
}

func (t *tree[T, K]) sameKey(a, b T) bool {
	ka, kb := t.ix.key(a), t.ix.key(b)
	return !t.ix.less(ka, kb) && !t.ix.less(kb, ka)
}

func (t *tree[T, K]) remove(v T) {
	t.bt.Delete(entry[T, K]{key: t.ix.key(v)})
}

func (t *tree[T, K]) len() int {
	return t.bt.Len()
}
