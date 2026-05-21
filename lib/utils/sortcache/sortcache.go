/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"iter"
	"sync"

	"github.com/google/btree"
)

// Config configures a [SortCache].
type Config[T comparable, I comparable] struct {
	// Indexes is a map of index name to key constructor, and defines the set of indexes
	// upon which lookups can be made. Values that overlap in *any* indexes are treated
	// as unique. A Put operation for a value that matches an existing value on *any* index
	// results in the existing value being evicted. This means that special care must be taken
	// to ensure that values that are not meant to overlap one another do not produce identical
	// keys across any index.  The simplest way to achieve this is to pick one index to be truly
	// unique, and then use that value as a suffix for all other indexes.  For example, if one wanted
	// to store node resources in a [SortCache], one might create a ServerID index, and then use
	// ServerID as a suffix for all other indexes (e.g. hostname). The Indexes mapping and its
	// members *must* be treated as immutable once passed to [New]. Since there is no default index,
	// at least one index must be supplied for [SortCache] to be usable.
	Indexes map[I]func(T) string
}

// SortCache is a helper for storing values that must be sortable across
// multiple indexes simultaneously. It has an internal read-write lock
// and is safe for concurrent use, but the supplied configuration must not
// be modified, and it is generally best to never modify stored resources.
type SortCache[T comparable, I comparable] struct {
	rw      sync.RWMutex
	indexes map[I]func(T) string
	trees   map[I]*btree.BTreeG[entry[T]]
	// Because the design of SortCache doesn't have a privileged index that
	// represents the identity of an item (i.e. a primary key) we have to make
	// up our own primary key by requiring that T is comparable and storing the
	// values once more, keyed by the value itself (which is almost universally
	// going to be a pointer or an interface storing a pointer.
	values map[T]struct{}
}

type entry[T comparable] struct {
	key   string
	value T
}

// New sets up a new [SortCache] based on the provided configuration.
func New[T comparable, I comparable](cfg Config[T, I]) *SortCache[T, I] {
	const (
		// bTreeDegree of 8 is standard across most of the teleport codebase
		bTreeDegree = 8
	)

	trees := make(map[I]*btree.BTreeG[entry[T]], len(cfg.Indexes))

	for index := range cfg.Indexes {
		trees[index] = btree.NewG(bTreeDegree, func(a, b entry[T]) bool {
			return a.key < b.key
		})
	}

	return &SortCache[T, I]{
		indexes: cfg.Indexes,
		trees:   trees,
		values:  make(map[T]struct{}),
	}
}

// Get loads the value associated with the given index and key. ok will be false if either
// the index does not exist or no value maps to the provided key on that index. Note that
// mutating a value such that any of its index keys change is not permissible and will result
// in permanently bad state. To avoid this, any implementation that might mutate returned
// values must clone them.
func (c *SortCache[T, I]) Get(index I, key string) (value T, ok bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.getLocked(index, key)
}

// HasIndex checks if the specified index is present in this cache.
func (c *SortCache[T, I]) HasIndex(name I) bool {
	// index map is treated as immutable once created, so no lock is required.
	_, ok := c.indexes[name]
	return ok
}

// KeyOf gets the key of the supplied value on the given index.
func (c *SortCache[T, I]) KeyOf(index I, value T) string {
	// index map is treated as immutable once created, so no lock is required.
	fn, ok := c.indexes[index]
	if !ok {
		return ""
	}
	return fn(value)
}

func (c *SortCache[T, I]) getLocked(index I, key string) (T, bool) {
	tree, exists := c.trees[index]
	if !exists {
		return *new(T), false
	}

	entry, exists := tree.Get(entry[T]{key: key})
	if !exists {
		return *new(T), false
	}

	return entry.value, true
}

// Put inserts a value into the sort cache, removing any existing values that collide with it. Since all indexes
// are required to be unique, a single Put can end up evicting up to N existing values where N is the number of
// indexes if it happens to collide with a different value on each index. implementations that expect evictions
// to always either be zero or one (e.g. caches of resources with unique IDs) should check the returned eviction
// count to help detect bugs.
func (c *SortCache[T, I]) Put(value T) (evicted int) {
	c.rw.Lock()
	defer c.rw.Unlock()

	if _, alreadyStored := c.values[value]; alreadyStored {
		return 0
	}
	c.values[value] = struct{}{}

	for index, fn := range c.indexes {
		key := fn(value)
		// ensure previous entry in this index is deleted if it exists
		if prev, ok := c.getLocked(index, key); ok {
			c.deleteValueLocked(prev)
			evicted++
		}
		c.trees[index].ReplaceOrInsert(entry[T]{
			key:   key,
			value: value,
		})
	}
	return
}

// Clear wipes all items from the cache and returns
// the cache to its initial empty state.
func (c *SortCache[T, I]) Clear() {
	c.rw.Lock()
	defer c.rw.Unlock()

	for _, tree := range c.trees {
		tree.Clear(true)
	}

	clear(c.values)
}

// Delete deletes the value associated with the specified index/key if one exists.
func (c *SortCache[T, I]) Delete(index I, key string) {
	c.rw.Lock()
	defer c.rw.Unlock()

	value, ok := c.getLocked(index, key)
	if !ok {
		return
	}

	c.deleteValueLocked(value)
}

func (c *SortCache[T, I]) deleteValueLocked(value T) {
	if _, ok := c.values[value]; !ok {
		return
	}
	delete(c.values, value)

	for idx, fn := range c.indexes {
		c.trees[idx].Delete(entry[T]{key: fn(value)})
	}
}

// readItems populates out with up to pageSize entries from the tree between start and stop.
// The returned values indicate how many entries were read and what the next key in the
// sequence is.
func (c *SortCache[T, I]) readItems(tree *btree.BTreeG[entry[T]], start, stop string, desc bool, out []T) (n int, next string) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	fn := func(ent entry[T]) bool {
		if n == len(out) {
			next = ent.key
			return false
		}

		out[n] = ent.value
		n++
		return true
	}

	// select the appropriate iteration variant based on direction
	// and whether start/stop points were specified.
	if desc {
		switch {
		case start == "" && stop == "":
			tree.Descend(fn)
		case start == "":
			tree.DescendGreaterThan(entry[T]{key: stop}, fn)
		case stop == "":
			tree.DescendLessOrEqual(entry[T]{key: start}, fn)
		default:
			tree.DescendRange(entry[T]{key: start}, entry[T]{key: stop}, fn)
		}
	} else {
		switch {
		case start == "" && stop == "":
			tree.Ascend(fn)
		case start == "":
			tree.AscendLessThan(entry[T]{key: stop}, fn)
		case stop == "":
			tree.AscendGreaterOrEqual(entry[T]{key: start}, fn)
		default:
			tree.AscendRange(entry[T]{key: start}, entry[T]{key: stop}, fn)
		}
	}

	return
}

// items returns an iterator of entries for an index between start and stop.
// To avoid holding the read lock for the duration of iteration, chunks of
// entries are consumed from the tree and then provided to the yield function.
// While this may increase the cost of using iterators, it prevents callers
// from performing expensive operations while the read lock is held which
// would prevent any writes to the cache.
func (c *SortCache[T, I]) items(index I, start, stop string, desc bool) iter.Seq[T] {
	tree, ok := c.trees[index]
	if !ok {
		return func(yield func(T) bool) {}
	}

	const pageSize = 1000
	return func(yield func(T) bool) {
		items := make([]T, pageSize)
		for {
			n, next := c.readItems(tree, start, stop, desc, items)
			for _, item := range items[:n] {
				if !yield(item) {
					return
				}
			}

			if n == 0 || next == "" {
				return
			}
			start = next
		}
	}
}

// Ascend iterates the specified range from least to greatest. if this method is being used to read a range,
// it is strongly recommended that all values retained be cloned. any mutation that results in changing a
// value's index keys will put the sort cache into a permanently bad state. empty strings are treated as
// "open" bounds. passing an empty string for both the start and stop bounds iterates all values.
//
// NOTE: ascending ranges are equivalent to the default range logic used across most of teleport, so
// common helpers like `backend.RangeEnd` will function as expected with this method.
func (c *SortCache[T, I]) Ascend(index I, start, stop string) iter.Seq[T] {
	return c.items(index, start, stop, false)
}

// Descend iterates the specified range from greatest to least. if this method is being used to read a range,
// it is strongly recommended that all values retained be cloned. any mutation that results in changing a
// value's index keys will put the sort cache into a permanently bad state. empty strings are treated as
// "open" bounds. passing an empty string for both the start and stop bounds iterates all values.
//
// NOTE: descending sort order is the *opposite* of what most teleport range-based logic uses, meaning that
// many common patterns need to be inverted when using this method (e.g. `backend.RangeEnd` actually gives
// you the start position for descending ranges).
func (c *SortCache[T, I]) Descend(index I, start, stop string) iter.Seq[T] {
	return c.items(index, start, stop, true)
}

// Len returns the number of values currently stored.
func (c *SortCache[T, I]) Len() int {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return len(c.values)
}
