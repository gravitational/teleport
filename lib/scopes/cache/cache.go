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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/scopes"
)

// Config configures a cache.
type Config[T any, K comparable] struct {
	// Scope is the function used to determine the scope of a value.
	Scope func(T) string
	// Key is the function used to determine the primary key of a value.
	Key func(T) K
}

// node is a tree node in the cache which stores values at a given scope.
type node[T any, K comparable] struct {
	// members is the set of values "at" this scope.
	members map[K]struct{}

	// children is the set of child scopes.
	children map[string]*node[T, K]
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
type Cache[T any, K comparable] struct {
	cfg   Config[T, K]
	items map[K]T
	root  *node[T, K]
}

// New builds a new cache instance based on the supplied config.
func New[T any, K comparable](cfg Config[T, K]) (*Cache[T, K], error) {
	if cfg.Scope == nil {
		return nil, trace.BadParameter("missing required scope function for scope cache")
	}

	if cfg.Key == nil {
		return nil, trace.BadParameter("missing required key function for scope cache")
	}

	return &Cache[T, K]{
		cfg:   cfg,
		items: make(map[K]T),
	}, nil
}

// ScopedItems provides the canonical representation of a scope and an iterator over the items within it. Typically
// used as the item of an outer iterator across multiple scopes.
type ScopedItems[T any] struct {
	scope string
	items iter.Seq[T]
}

// Scope is the canonical representation of the scope to which the items belong. Note that
// it is theoretically possible for this to be different than the scope value of any particular
// item in the iterator.
func (s *ScopedItems[T]) Scope() string {
	// TODO(fspmarshall): should we lazily build the scope string? changes are that a lot of
	// usecases won't care about it.
	return s.scope
}

// Items is an iterator over the items within the above scope. Note that within an iterator of ScopedItems,
// this iterator may only be safe to use during the current outer iteration.
func (s *ScopedItems[T]) Items() iter.Seq[T] {
	// TODO(fspmarshall): lazily build the iterator here so that iteration can happen multiple times
	// if needed.
	return s.items
}

// newScopedItems is a helper function for creating a ScopedItems instance within a top-level iterator.
func newScopedItems[T any, K comparable](segments []string, members map[K]struct{}, items map[K]T) ScopedItems[T] {
	return ScopedItems[T]{
		scope: scopes.Join(segments...),
		items: func(yield func(T) bool) {
			for key := range members {
				if !yield(items[key]) {
					return
				}
			}
		},
	}
}

// PoliciesApplicableToResourceScope iterates over the cached items using policy-application rules (i.e.
// a descending iteration from root, through the leaf of the specified scope).
func (c *Cache[T, K]) PoliciesApplicableToResourceScope(scope string) iter.Seq[ScopedItems[T]] {
	return func(yield func(ScopedItems[T]) bool) {
		if c.root == nil {
			return
		}

		// keep track of the segments visited
		var visited []string

		// start at the root
		current := c.root

		for segment := range scopes.DescendingSegments(scope) {
			// yield the current scope if it is non-empty
			if len(current.members) != 0 {
				if !yield(newScopedItems(visited, current.members, c.items)) {
					return
				}
			}

			// check for the next scope
			if _, ok := current.children[segment]; !ok {
				return
			}

			// advance to the next scope
			visited = append(visited, segment)
			current = current.children[segment]
		}

		// yield the final scope if it is non-empty
		if len(current.members) != 0 {
			if !yield(newScopedItems(visited, current.members, c.items)) {
				return
			}
		}
	}
}

// ResourcesSubjectToPolicyScope iterates over the cached items using resources-subjugation rules (i.e.
// an exhaustive iteration of the specified scope and all of its descendants).
func (c *Cache[T, K]) ResourcesSubjectToPolicyScope(scope string) iter.Seq[ScopedItems[T]] {
	return func(yield func(ScopedItems[T]) bool) {
		if c.root == nil {
			return
		}

		// keep track of the segments visited
		var visited []string

		// search for start position, beginning at the root
		start := c.root

		for segment := range scopes.DescendingSegments(scope) {
			// check for the next scope
			if _, ok := start.children[segment]; !ok {
				// reached the end prior to finding the target scope,
				// nothing to yield.
				return
			}

			// advance to the next scope
			visited = append(visited, segment)
			start = start.children[segment]
		}

		// recursively yield starting position and all of its descendants
		if !recursiveYield(yield, visited, start, c.items) {
			return
		}
	}
}

// recursiveYield is a helper function for recursively yielding all members of a given scope node, and all
// members of all of its children. The returned bool is the return value of the last call to yield.
func recursiveYield[T any, K comparable](yield func(ScopedItems[T]) bool, visited []string, current *node[T, K], items map[K]T) bool {
	// yield the current scope if it is non-empty
	if len(current.members) != 0 {
		if !yield(newScopedItems(visited, current.members, items)) {
			return false
		}
	}

	// recursively yield all child scopes
	for segment, child := range current.children {
		if !recursiveYield(yield, append(visited, segment), child, items) {
			return false
		}
	}

	return true
}

// Put inserts the given value, potentially displacing an existing value with the same
// primary key.
func (c *Cache[T, K]) Put(value T) {
	// get scope and key for this value
	scope, key := c.cfg.Scope(value), c.cfg.Key(value)

	// ensure that any previous value at this primary key has been removed
	c.Del(key)

	if c.root == nil {
		c.root = &node[T, K]{
			members:  make(map[K]struct{}),
			children: make(map[string]*node[T, K]),
		}
	}

	// find the node for this scope
	current := c.root
	for segment := range scopes.DescendingSegments(scope) {
		if _, ok := current.children[segment]; !ok {
			current.children[segment] = &node[T, K]{
				members:  make(map[K]struct{}),
				children: make(map[string]*node[T, K]),
			}
		}
		current = current.children[segment]
	}

	// add the value to the set of members at this scope
	current.members[key] = struct{}{}

	// add the value to the set of members at this primary key
	c.items[key] = value
}

// Del deletes the value associated with the given primary key.
func (c *Cache[T, K]) Del(key K) {
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
		current = current.children[segment]
	}

	// remove the value from the set of members at this scope
	delete(current.members, key)

	// remove the value from the set of members at this primary key
	delete(c.items, key)
}

// Len gets the total number of unique items stored in the cache.
func (c *Cache[T, K]) Len() int {
	return len(c.items)
}
