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
	"sync"

	"github.com/google/btree"
)

// Config configures a [SortCache].
type Config[T any] struct {
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
	Indexes map[string]func(T) string
}

// SortCache is a helper for storing values that must be sortable across
// multiple indexes simultaneously. It has an internal read-write lock
// and is safe for concurrent use, but the supplied configuration must not
// be modified, and it is generally best to never modify stored resources.
type SortCache[T any] struct {
	rw      sync.RWMutex
	indexes map[string]func(T) string
	trees   map[string]*btree.BTreeG[entry]
	values  map[uint64]T
	counter uint64
}

type entry struct {
	key string
	ref uint64
}

// New sets up a new [SortCache] based on the provided configuration.
func New[T any](cfg Config[T]) *SortCache[T] {
	const (
		// bTreeDegree of 8 is standard across most of the teleport codebase
		bTreeDegree = 8
	)

	trees := make(map[string]*btree.BTreeG[entry], len(cfg.Indexes))

	for index := range cfg.Indexes {
		trees[index] = btree.NewG(bTreeDegree, func(a, b entry) bool {
			return a.key < b.key
		})
	}

	return &SortCache[T]{
		indexes: cfg.Indexes,
		trees:   trees,
		values:  make(map[uint64]T),
	}
}

// Get loads the value associated with the given index and key. ok will be false if either
// the index does not exist or no value maps to the provided key on that index. Note that
// mutating a value such that any of its index keys change is not permissible and will result
// in permanently bad state. To avoid this, any implementation that might mutate returned
// values must clone them.
func (c *SortCache[T]) Get(index, key string) (value T, ok bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	ref, ok := c.lookup(index, key)
	if !ok {
		return value, false
	}

	return c.values[ref], true
}

// HasIndex checks if the specified index is present in this cache.
func (c *SortCache[T]) HasIndex(name string) bool {
	// index map is treated as immutable once created, so no lock is required.
	_, ok := c.indexes[name]
	return ok
}

// KeyOf gets the key of the supplied value on the given index.
func (c *SortCache[T]) KeyOf(index string, value T) string {
	// index map is treated as immutable once created, so no lock is required.
	fn, ok := c.indexes[index]
	if !ok {
		return ""
	}
	return fn(value)
}

// lookup is an internal helper that finds the unique reference id for a value given a specific
// index/key. Must be called either under read or write lock.
func (c *SortCache[T]) lookup(index, key string) (ref uint64, ok bool) {
	tree, exists := c.trees[index]
	if !exists {
		return 0, false
	}

	entry, exists := tree.Get(entry{key: key})
	if !exists {
		return 0, false
	}

	return entry.ref, true
}

// Put inserts a value into the sort cache, removing any existing values that collide with it. Since all indexes
// are required to be unique, a single Put can end up evicting up to N existing values where N is the number of
// indexes if it happens to collide with a different value on each index. implementations that expect evictions
// to always either be zero or one (e.g. caches of resources with unique IDs) should check the returned eviction
// count to help detect bugs.
func (c *SortCache[T]) Put(value T) (evicted int) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.counter++
	c.values[c.counter] = value

	for index, fn := range c.indexes {
		key := fn(value)
		// ensure previous entry in this index is deleted if it exists. note that we are fully
		// deleting it, not just overwriting the reference for this index.
		if prev, ok := c.lookup(index, key); ok {
			c.deleteValue(prev)
			evicted++
		}
		c.trees[index].ReplaceOrInsert(entry{
			key: key,
			ref: c.counter,
		})
	}
	return
}

// Delete deletes the value associated with the specified index/key if one exists.
func (c *SortCache[T]) Delete(index, key string) {
	c.rw.Lock()
	defer c.rw.Unlock()

	ref, ok := c.lookup(index, key)
	if !ok {
		return
	}

	c.deleteValue(ref)
}

// deleteValue is an internal helper that completely deletes the value associated with the specified
// unique reference id, including removing all of its associated index entries.
func (c *SortCache[T]) deleteValue(ref uint64) {
	value, ok := c.values[ref]
	if !ok {
		return
	}
	delete(c.values, ref)

	for idx, fn := range c.indexes {
		c.trees[idx].Delete(entry{key: fn(value)})
	}
}

// Ascend iterates the specified range from least to greatest. iteration is terminated early if the
// supplied closure returns false. if this method is being used to read a range, it is strongly recommended
// that all values retained be cloned. any mutation that results in changing a value's index keys will put
// the sort cache into a permanently bad state. empty strings are treated as "open" bounds. passing an empty
// string for both the start and stop bounds iterates all values.
//
// NOTE: ascending ranges are equivalent to the default range logic used across most of teleport, so
// common helpers like `backend.RangeEnd` will function as expected with this method.
func (c *SortCache[T]) Ascend(index, start, stop string, iterator func(T) bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	tree, ok := c.trees[index]
	if !ok {
		return
	}

	fn := func(ent entry) bool {
		return iterator(c.values[ent.ref])
	}

	// select the appropriate ascend variant based on wether or not
	// start/stop points were specified.
	switch {
	case start == "" && stop == "":
		tree.Ascend(fn)
	case start == "":
		tree.AscendLessThan(entry{key: stop}, fn)
	case stop == "":
		tree.AscendGreaterOrEqual(entry{key: start}, fn)
	default:
		tree.AscendRange(entry{key: start}, entry{key: stop}, fn)
	}
}

// AscendPaginated returns a page from a range of items in the sortcache in ascending order, and the nextKey.
func (c *SortCache[T]) AscendPaginated(index, startKey string, endKey string, pageSize int) ([]T, string) {
	page := make([]T, 0, pageSize+1)

	c.Ascend(index, startKey, endKey, func(r T) bool {
		page = append(page, r)
		return len(page) <= pageSize
	})

	var nextKey string
	if len(page) > pageSize {
		nextKey = c.KeyOf(index, page[pageSize])
		page = page[:pageSize]
	}

	return page, nextKey
}

// Descend iterates the specified range from greatest to least. iteration is terminated early if the
// supplied closure returns false. if this method is being used to read a range, it is strongly recommended
// that all values retained be cloned. any mutation that results in changing a value's index keys will put
// the sort cache into a permanently bad state. empty strings are treated as "open" bounds. passing an empty
// string for both the start and stop bounds iterates all values.
//
// NOTE: descending sort order is the *opposite* of what most teleport range-based logic uses, meaning that
// many common patterns need to be inverted when using this method (e.g. `backend.RangeEnd` actually gives
// you the start position for descending ranges).
func (c *SortCache[T]) Descend(index, start, stop string, iterator func(T) bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	tree, ok := c.trees[index]
	if !ok {
		return
	}

	fn := func(ent entry) bool {
		return iterator(c.values[ent.ref])
	}

	// select the appropriate descend variant based on wether or not
	// start/stop points were specified.
	switch {
	case start == "" && stop == "":
		tree.Descend(fn)
	case start == "":
		tree.DescendGreaterThan(entry{key: stop}, fn)
	case stop == "":
		tree.DescendLessOrEqual(entry{key: start}, fn)
	default:
		tree.DescendRange(entry{key: start}, entry{key: stop}, fn)
	}
}

// DescendPaginated returns a page from a range of items in the sortcache in descending order, and the nextKey.
func (c *SortCache[T]) DescendPaginated(index, startKey string, endKey string, pageSize int) ([]T, string) {
	page := make([]T, 0, pageSize+1)

	c.Descend(index, startKey, endKey, func(r T) bool {
		page = append(page, r)
		return len(page) <= pageSize
	})

	var nextKey string
	if len(page) > pageSize {
		nextKey = c.KeyOf(index, page[pageSize])
		page = page[:pageSize]
	}

	return page, nextKey
}

// Len returns the number of values currently stored.
func (c *SortCache[T]) Len() int {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return len(c.values)
}
