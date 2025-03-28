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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/sortcache"
)

// store persists cached resources directly in memory.
type store[T any] struct {
	cache   *sortcache.SortCache[T]
	indexes map[string]func(T) string
}

// newStore creates a store that will index the resource
// based on the provided indexes.
func newStore[T any](indexes map[string]func(T) string) *store[T] {
	return &store[T]{
		indexes: indexes,
		cache: sortcache.New(sortcache.Config[T]{
			Indexes: indexes,
		}),
	}
}

// clear removes all items from the store.
func (s *store[T]) clear() error {
	s.cache.Clear()
	return nil
}

// put adds a new item, or updates an existing item.
func (s *store[T]) put(t T) error {
	s.cache.Put(t)
	return nil
}

// delete removes the provided item if any of the indexes match.
func (s *store[T]) delete(t T) error {
	for idx, transform := range s.indexes {
		s.cache.Delete(idx, transform(t))
	}

	return nil
}

// len returns the number of values currently stored.
func (s *store[T]) len() int {
	return s.cache.Len()
}

// get returns the item matching the provided index and item,
// or a [trace.NotFoundError] if no match was found.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T]) get(index, key string) (T, error) {
	t, ok := s.cache.Get(index, key)
	if !ok {
		return t, trace.NotFound("no value for key %q in index %q", key, index)
	}

	return t, nil
}

// resources returns an iterator over all items in the provided range
// for the given index in ascending order.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T]) resources(index, start, stop string) iter.Seq[T] {
	return s.cache.Ascend(index, start, stop)
}
