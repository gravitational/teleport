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

package cache

import (
	"cmp"
	"iter"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils/sortmap"
)

// Cursor describes a starting position for a paginated iteration over the cache. A cursor is only
// considered filled in if both the key and scope are nonzero.
type Cursor[K cmp.Ordered] struct {
	// Scope is the scope to resume from.
	Scope string
	// Key is the primary key of the item to resume from.
	Key K
}

// IsZero returns true if the cursor is empty.
func (c *Cursor[K]) IsZero() bool {
	return c == nil || (c.Key == *new(K) && c.Scope == "")
}

// Option is a functional option that can be used to configure cache query behavior.
type Option[T any, K cmp.Ordered] func(*options[T, K])

type options[T any, K cmp.Ordered] struct {
	cursor Cursor[K]
	filter func(T) bool
}

// Config configures a cache.
type Config[T any, K cmp.Ordered] struct {
	// Scope is the function used to determine the scope of a value.
	Scope func(T) string
	// Key is the function used to determine the primary key of a value.
	Key func(T) K
	// Clone is an optional clone function that can be used to create deep copies
	// of values prior to yielding them.
	Clone func(T) T
}

// node is a tree node in the cache which stores values at a given scope.
type node[K cmp.Ordered] struct {
	// members is the set of values "at" this scope.
	members *sortmap.Map[K, struct{}]

	// children is the set of child scopes.
	children *sortmap.Map[string, *node[K]]
}

func newNode[K cmp.Ordered]() *node[K] {
	return &node[K]{
		members:  sortmap.New[K, struct{}](),
		children: sortmap.New[string, *node[K]](),
	}
}

// Cache is a generic scoped value cache. It constructs a basic tree structure based on scope segments
// to support efficient lookings up values based on common scoping patterns. This cache is intended to be
// a tool for building more purpose-specific wrappers and should not be used directly in access-control
// logic due to ease of misuse.
//
// NOTE: this is an early stage prototype and has a few notable limitations:
//   - Not currently safe for concurrent use.
//   - Iteration order of read methods is nondeterministic.
//   - No cleanup of empty scopes.
type Cache[T any, K cmp.Ordered] struct {
	cfg   Config[T, K]
	rw    sync.RWMutex
	items map[K]T
	root  *node[K]
}

// New builds a new cache instance based on the supplied config.
func New[T any, K cmp.Ordered](cfg Config[T, K]) (*Cache[T, K], error) {
	if cfg.Scope == nil {
		return nil, trace.BadParameter("missing required scope function for scope cache")
	}

	if cfg.Key == nil {
		return nil, trace.BadParameter("missing required key function for scope cache")
	}

	if cfg.Clone == nil {
		cfg.Clone = func(value T) T { return value }
	}

	return &Cache[T, K]{
		cfg:   cfg,
		items: make(map[K]T),
	}, nil
}

// Must is a convenience function for creating a cache instance that panics on error. Cache creation only
// panics if required configuration values are missing, so statically configured caches can confidently
// use this helper to avoid unnecessary error ceremony.
func Must[T any, K cmp.Ordered](cfg Config[T, K]) *Cache[T, K] {
	cache, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return cache
}

// KeyOf returns the primary key of the given value.
func (c *Cache[T, K]) KeyOf(value T) K {
	return c.cfg.Key(value)
}

// WithCursor specifies a cursor to use when querying the cache. If specified, the cache will
// resume iteration from position described by the cursor. Passing in a zero value cursor
// has no effect. Functional option is a method to work around go's terrible type inference.
func (c *Cache[T, K]) WithCursor(cursor Cursor[K]) Option[T, K] {
	return func(o *options[T, K]) {
		o.cursor = cursor
	}
}

// WithFilter specifies a filter to be used when querying the cache. If specified, the cache
// will skip yielding items that do not match the filter. Functional option is a method to
// work around go's terrible type inference.
func (c *Cache[T, K]) WithFilter(filter func(T) bool) Option[T, K] {
	return func(o *options[T, K]) {
		o.filter = filter
	}
}

// PoliciesApplicableToResourceScope iterates over the cached items using policy-application rules (i.e.
// a descending iteration from root, through the leaf of the specified scope).
func (c *Cache[T, K]) PoliciesApplicableToResourceScope(scope string, opts ...Option[T, K]) iter.Seq[ScopedItems[T]] {
	return func(yield func(ScopedItems[T]) bool) {
		c.rw.RLock()
		defer c.rw.RUnlock()

		var options options[T, K]
		for _, opt := range opts {
			opt(&options)
		}

		if c.root == nil {
			return
		}

		// keep track of the segments visited
		var visited []string

		// start at the root
		current := c.root

		descender := newDescender(options.cursor)
		defer descender.Stop()

		for segment := range scopes.DescendingSegments(scope) {
			if !maybeYieldScopedItems(yield, visited, &descender, current.members, c.items, c.cfg.Clone, options.filter) {
				return
			}

			// get next scope if it exists
			var ok bool
			current, ok = current.children.Get(segment)
			if !ok {
				return
			}

			// finish yielding from this scope, advance the descender
			descender.Descend(segment)

			// update visited segments
			visited = append(visited, segment)
		}

		// yield the final scope if it is non-empty
		if !maybeYieldScopedItems(yield, visited, &descender, current.members, c.items, c.cfg.Clone, options.filter) {
			return
		}
	}
}

// ResourcesSubjectToPolicyScope iterates over the cached items using resources-subjugation rules (i.e.
// an exhaustive descending iteration of the specified scope and all of its descendants).
func (c *Cache[T, K]) ResourcesSubjectToPolicyScope(scope string, opts ...Option[T, K]) iter.Seq[ScopedItems[T]] {
	return func(yield func(ScopedItems[T]) bool) {
		c.rw.RLock()
		defer c.rw.RUnlock()

		var options options[T, K]
		for _, opt := range opts {
			opt(&options)
		}

		if c.root == nil {
			return
		}

		// keep track of the segments visited
		var visited []string

		// search for start position, beginning at the root
		current := c.root

		descender := newDescender(options.cursor)
		defer descender.Stop()

		for segment := range scopes.DescendingSegments(scope) {
			// get next scope if it exists
			var ok bool
			current, ok = current.children.Get(segment)
			if !ok {
				// reached the end prior to finding the target scope,
				// nothing to yield.
				return
			}

			// advance the descender to the next scope
			descender.Descend(segment)

			// update visited segments
			visited = append(visited, segment)
		}

		// recursively yield starting position and all of its descendants
		if !recursiveYield(yield, visited, &descender, current, c.items, c.cfg.Clone, options.filter) {
			return
		}
	}
}

// recursiveYield is a helper function for recursively yielding all members of a given scope node, and all
// members of all of its children. The returned bool is the return value of the last call to yield.
func recursiveYield[T any, K cmp.Ordered](yield func(ScopedItems[T]) bool, visited []string, descender *descender[K], current *node[K], items map[K]T, clone func(T) T, filter func(T) bool) bool {
	// yield the current scope if we've finished descending to resume position and it is non-empty
	if !maybeYieldScopedItems(yield, visited, descender, current.members, items, clone, filter) {
		return false
	}

	// recursively yield all child scopes
	for segment, child := range current.children.Ascend(descender.NextSegment()) {
		descender.Descend(segment)

		// recursively yield all child and all of its descendants
		if !recursiveYield(yield, append(visited, segment), descender, child, items, clone, filter) {
			return false
		}
	}

	return true
}

// Put inserts the given value, potentially displacing an existing value with the same
// primary key.
func (c *Cache[T, K]) Put(value T) {
	c.rw.Lock()
	defer c.rw.Unlock()

	// get scope and key for this value
	scope, key := c.cfg.Scope(value), c.cfg.Key(value)

	// ensure that any previous value at this primary key has been removed
	c.deleteLocked(key)

	if c.root == nil {
		c.root = newNode[K]()
	}

	// find the node for this scope
	current := c.root
	for segment := range scopes.DescendingSegments(scope) {
		current = current.children.GetOrCreate(segment, newNode[K])
	}

	// add the value to the set of members at this scope
	current.members.Set(key, struct{}{})

	// add the value to the set of members at this primary key
	c.items[key] = value
}

// Del deletes the value associated with the given primary key.
func (c *Cache[T, K]) Del(key K) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.deleteLocked(key)
}

func (c *Cache[T, K]) deleteLocked(key K) {
	// get the value associated with this primary key
	value, ok := c.items[key]
	if !ok {
		return
	}

	// determine the scope for this value
	scope := c.cfg.Scope(value)

	// get the node for this scope
	current := c.root
	for segment := range scopes.DescendingSegments(scope) {
		current, _ = current.children.Get(segment)
	}

	// remove the value from the set of members at this scope
	current.members.Del(key)

	// remove the value from the set of members at this primary key
	delete(c.items, key)
}

// Len gets the total number of unique items stored in the cache.
func (c *Cache[T, K]) Len() int {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return len(c.items)
}

// ScopedItems provides the canonical representation of a scope and an iterator over the items within it. Typically
// used as the item of an outer iterator across multiple scopes. ScopedItems instances are not safe for use outside
// of the context of the iterator that yielded them.
type ScopedItems[T any] struct {
	segments []string
	items    func() iter.Seq[T]
}

// Scope returns the canonical representation of the scope to which the items belong. Calling code should
// prefer to retain the returned value rather than call this method multiple times, as the returned
// value is lazily constructed. Note that it is theoretically possible for the returned value to be
// different than the scope value of any particular item in the iterator.
func (s *ScopedItems[T]) Scope() string {
	return scopes.Join(s.segments...)
}

// Items returns an iterator over the items within the associated scope. Note that within an iterator of
// ScopedItems, this iterator may only be safe to use during the current outer iteration.
func (s *ScopedItems[T]) Items() iter.Seq[T] {
	return s.items()
}

func maybeYieldScopedItems[T any, K cmp.Ordered](yield func(ScopedItems[T]) bool, segments []string, descender *descender[K], members *sortmap.Map[K, struct{}], items map[K]T, clone func(T) T, filter func(T) bool) bool {

	if !descender.Yield() || members.Len() == 0 {
		// we are still descending to a cursor position or there are no items to yield
		return true /* continue iteration */
	}

	start := descender.StartKey()

	if filter != nil {
		// we don't want to yield a scope unless it contains at least one value that matches the filter,
		// so we seek to the position of the first item prior to yielding.
		var foundMatch bool
		for key, _ := range members.Ascend(start) {
			if !filter(items[key]) {
				continue
			}
			start = key
			foundMatch = true
			break
		}
		if !foundMatch {
			return true /* continue iteration */
		}
	}

	return yield(newScopedItems(segments, start, members, items, clone, filter))
}

// newScopedItems is a helper function for creating a ScopedItems instance within a top-level iterator.
func newScopedItems[T any, K cmp.Ordered](segments []string, start K, members *sortmap.Map[K, struct{}], items map[K]T, clone func(T) T, filter func(T) bool) ScopedItems[T] {
	return ScopedItems[T]{
		segments: segments,
		items: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for key, _ := range members.Ascend(start) {
					val := items[key]
					if filter != nil && !filter(val) {
						continue
					}
					if !yield(clone(val)) {
						return
					}
				}
			}
		},
	}
}

// descender is a helper for descending to the correct target scope component when resuming iteration
// using a cursor value. the descender is intended to make the logic of descent/resumption cleaner, and
// to reduce branching in primary query logic by encapsulating most resumption logic into an iterative
// state-machine. the zero value of the descender is a valid descender with behavior equivalent to a query
// with no cursor.
type descender[K cmp.Ordered] struct {
	startKey   K
	next       func() (string, bool)
	stop       func()
	segment    string
	descending bool
}

// Descend advances the descender by one scope component depth. this should only be called
// after any necessary invocations of yield/next/peek for the current scope level. it must
// be called with the value of the segment that we are descending *into*.
func (d *descender[K]) Descend(segment string) {
	if !d.descending {
		// we are continuing to descend after already having reached the target scope on a previous
		// iteration. ensure the start key is cleared out so that we don't continue to use it on
		// this or any future segments.
		d.startKey = *new(K)
		return
	}

	if segment != d.segment {
		// we were still in descending mode and our path diverged from the expected one. this indicates
		// that the target scope has been deleted/emptied since the time the initial cursor value was
		// created. stop the underlying iterator and back out of descending mode. when the target scope
		// no longer exists, we end up in the next existing scope in iteration order, so the next item
		// encountered will be the proper place to start yielding items from.
		d.Stop()
		return
	}

	d.segment, d.descending = d.next()
}

// Yield indicates wether or not we've descended to the appropriate depth to
// start yielding items.
func (d *descender[K]) Yield() bool {
	return !d.descending
}

// StartKey gets the StartKey that should be used for the current scope level. This will
// be the zero value for all scopes except the resumption target scope.
func (d *descender[K]) StartKey() K {
	if !d.descending {
		// we are at exactly the descent target, use the
		// start key from our cursor.
		return d.startKey
	}

	return *new(K)
}

// NextSegment reports the next scope component to descend to (only relevant in exhaustive descents,
// where we need to know the next value in order to skip previously visited scopes).
func (d *descender[K]) NextSegment() string {
	return d.segment
}

// stop *must* be called to ensure that the descender doesn't leak coroutines. safe for double-call.
func (d *descender[K]) Stop() {
	if d.stop == nil {
		return
	}

	// halt the underlying iterator
	d.stop()

	// clear the descender (only relevant for early halts due to scope path divergence)
	*d = descender[K]{}
}

// newDescender creates a descender instance for use in managing descent to a target scope during
// resumption of a previous iteration.
func newDescender[K cmp.Ordered](cursor Cursor[K]) descender[K] {
	if cursor.IsZero() {
		return descender[K]{}
	}

	next, stop := iter.Pull(scopes.DescendingSegments(cursor.Scope))

	segment, descending := next()

	return descender[K]{
		startKey:   cursor.Key,
		next:       next,
		stop:       stop,
		segment:    segment,
		descending: descending,
	}
}
