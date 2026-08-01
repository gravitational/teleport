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

// typedStore persists cached resources directly in memory, indexed by a
// collection-defined set of typed index handles.
//
// It is backed by the snapshot-based sortcache/v2: reads operate on immutable
// snapshots and are wait-free, writes are serialized copy-on-write commits.
// IX is a collection-defined struct holding the collection's index handles;
// read paths reach them via the indexes field and read against snapshot(),
// wrapping range reads with [typedStore.resources] and point reads with
// [getByIndex] to apply the store's metric instrumentation.
type typedStore[T any, IX any] struct {
	kind    string
	cache   *sortcache.SortCache[T]
	clone   func(T) T
	indexes IX
}

// newTypedStore creates a store indexed by the provided handle set.
// registrations must list every index in the set, in a deterministic order:
// deletes resolve collisions in registration order, so the primary (truly
// unique) index must come first.
func newTypedStore[T any, IX any](kind string, clone func(T) T, indexes IX, registrations ...sortcache.AnyIndex[T]) *typedStore[T, IX] {
	return &typedStore[T, IX]{
		kind:    kind,
		clone:   clone,
		indexes: indexes,
		cache:   sortcache.New(sortcache.Config[T]{Indexes: registrations}),
	}
}

// resources wraps a range read built from one of the store's index handles
// (e.g. indexes.user.AscendPrefixFrom(...)) with the store's metric
// instrumentation.
//
// It is the responsibility of the caller to clone the resources
// before propagating them further.
func (s *typedStore[T, IX]) resources(seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		defer func() {
			backendmetrics.StreamingRequests.WithLabelValues("cache").Inc()
			backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
		}()

		for t := range seq {
			backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
			if !yield(t) {
				return
			}
		}
	}
}

// getByIndex returns the item matching the provided typed index and key from
// the guard's snapshot, with the store's metric instrumentation applied, or a
// [trace.NotFoundError] if no match was found. The index must belong to the
// guard's store.
//
// It is the responsibility of the caller to clone the resource
// before propagating it further.
func getByIndex[T any, IX any, K any](rg readGuard[T, *typedStore[T, IX]], index *sortcache.Index[T, K], key K) (T, error) {
	start := time.Now()
	t, ok := index.Get(rg.snapshot, key)
	backendmetrics.ReadLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.ReadRequests.WithLabelValues("cache").Inc()
	backendmetrics.Requests.WithLabelValues("cache", rg.store.kind, "false").Inc()
	if !ok {
		backendmetrics.ReadRequestsFailed.WithLabelValues("cache").Inc()
		return t, trace.NotFound("%q does not exist", rg.store.kind)
	}

	return t, nil
}

// snapshot returns the current immutable view of the store. Acquiring a
// snapshot is wait-free, and it remains valid and consistent for as long as
// it is held. Reads within one request should share a single snapshot.
func (s *typedStore[T, IX]) snapshot() *sortcache.Snapshot[T] {
	return s.cache.Snapshot()
}

// clear removes all items from the store.
func (s *typedStore[T, IX]) clear() error {
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
func (s *typedStore[T, IX]) replace(items []T) error {
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

// put adds or updates the provided items as a single commit. The variadic
// slice is treated as scratch space (items are replaced by their clones).
func (s *typedStore[T, IX]) put(items ...T) error {
	start := time.Now()
	for i, t := range items {
		items[i] = s.clone(t)
	}
	s.cache.Put(items...)
	backendmetrics.WriteLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.WriteRequests.WithLabelValues("cache").Add(float64(len(items)))
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()
	return nil
}

// delete removes the provided items, if any of the indexes match, as a
// single commit.
func (s *typedStore[T, IX]) delete(items ...T) error {
	start := time.Now()
	s.cache.Delete(items...)
	backendmetrics.WriteLatencies.WithLabelValues("cache").Observe(time.Since(start).Seconds())
	backendmetrics.WriteRequests.WithLabelValues("cache").Add(float64(len(items)))
	backendmetrics.Requests.WithLabelValues("cache", s.kind, "false").Inc()

	return nil
}

// len returns the number of values currently stored.
func (s *typedStore[T, IX]) len() int {
	return s.cache.Len()
}

// store is the legacy string-keyed store shape: indexes are identified by a
// constant of type I and every index key is a string. It exists to keep
// unconverted collections working unchanged; collections migrate to
// [typedStore] (whole collection at a time), and this type is deleted once
// the last one moves.
type store[T any, I comparable] struct {
	*typedStore[T, map[I]*sortcache.Index[T, string]]
}

// newStore creates a store that will index the resource
// based on the provided indexes.
func newStore[T any, I comparable](kind string, clone func(T) T, indexes map[I]func(T) string) *store[T, I] {
	handles := make(map[I]*sortcache.Index[T, string], len(indexes))
	registrations := make([]sortcache.AnyIndex[T], 0, len(indexes))
	for index, key := range indexes {
		handle := sortcache.NewIndex(fmt.Sprint(index), key)
		handles[index] = handle
		registrations = append(registrations, handle)
	}

	return &store[T, I]{
		typedStore: &typedStore[T, map[I]*sortcache.Index[T, string]]{
			kind:    kind,
			clone:   clone,
			indexes: handles,
			cache:   sortcache.New(sortcache.Config[T]{Indexes: registrations}),
		},
	}
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
		t, ok = handle.Get(s.snapshot(), key)
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

		for t := range seq(s.snapshot(), startBound, stopBound) {
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
	return handle.Count(s.snapshot(), startBound, stopBound)
}
