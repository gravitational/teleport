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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend/backendmetrics"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

// store persists cached resources directly in memory.
type store[T any, I comparable] struct {
	kind    string
	cache   *sortcache.SortCache[T, I]
	clone   func(T) T
	indexes map[I]func(T) string
}

// newStore creates a store that will index the resource
// based on the provided indexes.
func newStore[T any, I comparable](kind string, clone func(T) T, indexes map[I]func(T) string) *store[T, I] {
	return &store[T, I]{
		kind:    kind,
		clone:   clone,
		indexes: indexes,
		cache: sortcache.New(sortcache.Config[T, I]{
			Indexes: indexes,
		}),
	}
}

// clear removes all items from the store.
func (s *store[T, I]) clear() error {
	start := time.Now()
	s.cache.Clear()
	backendmetrics.BatchWriteLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.BatchWriteRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "true").Inc()
	return nil
}

// put adds a new item, or updates an existing item.
func (s *store[T, I]) put(t T) error {
	start := time.Now()
	s.cache.Put(s.clone(t))
	backendmetrics.WriteLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.WriteRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
	return nil
}

// delete removes the provided item if any of the indexes match.
func (s *store[T, I]) delete(t T) error {
	start := time.Now()
	for idx, transform := range s.indexes {
		s.cache.Delete(idx, transform(t))
	}
	backendmetrics.WriteLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.WriteRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()

	return nil
}

// len returns the number of values currently stored.
func (s *store[T, I]) len() int {
	return s.cache.Len()
}

// get returns the item matching the provided index and item,
// or a [trace.NotFoundError] if no match was found.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T, I]) get(index I, key string) (T, error) {
	start := time.Now()
	t, ok := s.cache.Get(index, key)
	backendmetrics.ReadLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
	if !ok {
		backendmetrics.ReadRequestsFailed.WithLabelValues("cache").Inc()
		return t, trace.NotFound("%q %q does not exist", s.kind, key)
	}

	return t, nil
}

// resources returns an iterator over all items in the provided range
// for the given index in ascending order.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T, I]) resources(index I, start, stop string) iter.Seq[T] {
	return func(yield func(T) bool) {
		defer func() {
			backendmetrics.StreamingRequests.WithLabelValues("cache").Inc()
			backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
		}()

		for t := range s.cache.Ascend(index, start, stop) {
			backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
			if !yield(t) {
				return
			}
		}
	}
}

// count returns the number of items that exist in the provided range.
func (s *store[T, I]) count(index I, start, stop string) int {
	var n int
	for range s.cache.Ascend(index, start, stop) {
		n++
	}

	return n
}
