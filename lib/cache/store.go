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
	"fmt"
	"iter"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend/backendmetrics"
	sortcache "github.com/gravitational/teleport/lib/utils/sortcache/v2"
)

// store persists cached resources directly in memory.
//
// It is backed by the snapshot-based sortcache/v2: reads operate on immutable
// snapshots and are wait-free, writes are serialized copy-on-write commits.
// The index definitions retain the legacy string-key shape
// (map[I]func(T) string) so collections require no changes; migrating
// individual collections to typed compound keys is a per-collection follow-up.
type store[T any, I comparable] struct {
	kind    string
	cache   *sortcache.SortCache[T]
	clone   func(T) T
	indexes map[I]*sortcache.Index[T, string]
}

// newStore creates a store that will index the resource
// based on the provided indexes.
func newStore[T any, I comparable](kind string, clone func(T) T, indexes map[I]func(T) string) *store[T, I] {
	handles := make(map[I]*sortcache.Index[T, string], len(indexes))
	registrations := make([]sortcache.AnyIndex[T], 0, len(indexes))
	for index, key := range indexes {
		handle := sortcache.NewIndex(fmt.Sprintf("%s/%v", kind, index), key)
		handles[index] = handle
		registrations = append(registrations, handle)
	}

	return &store[T, I]{
		kind:    kind,
		clone:   clone,
		indexes: handles,
		cache:   sortcache.New(sortcache.Config[T]{Indexes: registrations}),
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

// replace atomically resets the store to hold exactly the provided items.
// This is the bulk (re-)initialization path: the new generation is built
// aside and published with a single swap, so concurrent readers observe
// either the complete old state or the complete new state.
func (s *store[T, I]) replace(items []T) error {
	start := time.Now()
	s.cache.Replace(func(yield func(T) bool) {
		for _, item := range items {
			if !yield(s.clone(item)) {
				return
			}
		}
	})
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
	s.cache.Delete(t)
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
	var (
		t  T
		ok bool
	)
	if handle, exists := s.indexes[index]; exists {
		t, ok = handle.Get(s.cache.Snapshot(), key)
	}
	backendmetrics.ReadLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
	if !ok {
		backendmetrics.ReadRequestsFailed.WithLabelValues("cache").Inc()
		return t, trace.NotFound("%q %q does not exist", s.kind, key)
	}

	return t, nil
}

// bounds converts the legacy string range convention (empty string means
// open, start inclusive, stop exclusive) into typed bounds.
func bounds(start, stop string) (sortcache.Bound[string], sortcache.Bound[string]) {
	startBound, stopBound := sortcache.Open[string](), sortcache.Open[string]()
	if start != "" {
		startBound = sortcache.Inclusive(start)
	}
	if stop != "" {
		stopBound = sortcache.Exclusive(stop)
	}
	return startBound, stopBound
}

// resources returns an iterator over all items in the provided range
// for the given index in ascending order.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T, I]) resources(index I, start, stop string) iter.Seq[T] {
	return s.iterate(index, start, stop, false)
}

// resourcesDescending returns an iterator over all items in the provided
// range for the given index in descending order. Note that in descending
// order start is the upper end of the range.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func (s *store[T, I]) resourcesDescending(index I, start, stop string) iter.Seq[T] {
	return s.iterate(index, start, stop, true)
}

func (s *store[T, I]) iterate(index I, start, stop string, descending bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		defer func() {
			backendmetrics.StreamingRequests.WithLabelValues("cache").Inc()
			backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
		}()

		handle, exists := s.indexes[index]
		if !exists {
			return
		}

		startBound, stopBound := bounds(start, stop)
		seq := handle.Ascend
		if descending {
			seq = handle.Descend
		}

		for t := range seq(s.cache.Snapshot(), startBound, stopBound) {
			backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
			if !yield(t) {
				return
			}
		}
	}
}

// count returns the number of items that exist in the provided range.
func (s *store[T, I]) count(index I, start, stop string) int {
	handle, exists := s.indexes[index]
	if !exists {
		return 0
	}

	startBound, stopBound := bounds(start, stop)
	return handle.Count(s.cache.Snapshot(), startBound, stopBound)
}
