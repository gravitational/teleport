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

package sortmap

import (
	"cmp"
	"iter"

	"github.com/google/btree"
)

// Map is a very thin wrapper around btree.BTreeG that provides a more intuitive
// map-like interface and avoids the need to define a separate entry type
// and comparison function. It also provides a more intuitive form of start key
// for resumption/pagination where zero values always mean "get everything". Handy
// for cases where you want to add ordered/resumable iteration to existing logic that
// currently relies on a map.
//
// Note that this type performs no internal synchronization. Its read methods are safe for
// concurrent use, but its write methods are not.
type Map[K cmp.Ordered, V any] btree.BTreeG[entry[K, V]]

type entry[K cmp.Ordered, V any] struct {
	key   K
	value V
}

func compare[K cmp.Ordered, V any](a, b entry[K, V]) bool {
	return a.key < b.key
}

// New creates a new empty [Map] instance.
func New[K cmp.Ordered, V any]() *Map[K, V] {
	const (
		// bTreeDegree of 8 is standard across most of the teleport codebase
		bTreeDegree = 8
	)

	return (*Map[K, V])(btree.NewG[entry[K, V]](bTreeDegree, compare))
}

// Get retrieves the value associated with the given key if one exists.
func (m *Map[K, V]) Get(key K) (V, bool) {
	if m == nil {
		return *new(V), false
	}
	entry, exists := (*btree.BTreeG[entry[K, V]])(m).Get(entry[K, V]{key: key})
	return entry.value, exists
}

// GetOrCreate retrieves the value associated with the given key if one exists,
// falling back to creating and inserting a new value using the provided function.
func (m *Map[K, V]) GetOrCreate(key K, fn func() V) V {
	if val, ok := m.Get(key); ok {
		return val
	}
	val := fn()
	m.Set(key, val)
	return val
}

// Set sets the value associated with the given key, replacing any existing value.
func (m *Map[K, V]) Set(key K, value V) {
	(*btree.BTreeG[entry[K, V]])(m).ReplaceOrInsert(entry[K, V]{key: key, value: value})
}

// Del removes the entry associated with the given key.
func (m *Map[K, V]) Del(key K) {
	(*btree.BTreeG[entry[K, V]])(m).Delete(entry[K, V]{key: key})
}

// Descend returns a sequence of key-value pairs in descending order. If the provided start key
// is nonzero, iteration will start from the provided key position (inclusive).
func (m *Map[K, V]) Descend(start K) iter.Seq2[K, V] {
	if m == nil {
		return func(yield func(K, V) bool) {}
	}

	if start == *new(K) {
		return func(yield func(K, V) bool) {
			(*btree.BTreeG[entry[K, V]])(m).Descend(func(ent entry[K, V]) bool {
				return yield(ent.key, ent.value)
			})
		}
	}

	return func(yield func(K, V) bool) {
		(*btree.BTreeG[entry[K, V]])(m).DescendLessOrEqual(entry[K, V]{key: start}, func(ent entry[K, V]) bool {
			return yield(ent.key, ent.value)
		})
	}
}

// Ascend returns a sequence of key-value pairs in ascending order. If the provided 'start' key
// is nonzero, iteration will start from the provided key position (inclusive).
func (m *Map[K, V]) Ascend(start K) iter.Seq2[K, V] {
	if m == nil {
		return func(yield func(K, V) bool) {}
	}
	if start == *new(K) {
		return func(yield func(K, V) bool) {
			(*btree.BTreeG[entry[K, V]])(m).Ascend(func(ent entry[K, V]) bool {
				return yield(ent.key, ent.value)
			})
		}
	}

	return func(yield func(K, V) bool) {
		(*btree.BTreeG[entry[K, V]])(m).AscendGreaterOrEqual(entry[K, V]{key: start}, func(ent entry[K, V]) bool {
			return yield(ent.key, ent.value)
		})
	}
}

// Len returns the number of entries in the map.
func (m *Map[K, V]) Len() int {
	if m == nil {
		return 0
	}

	return (*btree.BTreeG[entry[K, V]])(m).Len()
}
