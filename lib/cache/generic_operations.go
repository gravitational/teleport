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
	"context"
	"iter"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
)

// genericGetter is a helper to retrieve a single item from a cache collection.
type genericGetter[T any, I comparable] struct {
	// cache to performe the primary read from.
	cache *Cache
	// collection that contains the item.
	collection *collection[T, I]
	// index of the collection to read with.
	index I
	// upstreamGet is used to retrieve the item if the
	// cache is not healthy.
	upstreamGet func(context.Context, string) (T, error)
}

// get retrieves a single item by an identifier from
// a cache collection. If the cache is not healthy, then the item is retrieved
// from the upstream backend. The item returend is cloned and ownership
// is retained by the caller.
func (g genericGetter[T, I]) get(ctx context.Context, identifier string) (T, error) {
	var t T
	rg, err := acquireReadGuard(g.cache, g.collection)
	if err != nil {
		return t, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, err := g.upstreamGet(ctx, identifier)
		return out, trace.Wrap(err)
	}

	out, err := rg.store.get(g.index, identifier)
	if err != nil {
		return t, trace.Wrap(err)
	}

	return g.collection.store.clone(out), nil
}

// genericLister is a helper to retrieve a page of items from a cache collection.
type genericLister[T any, I comparable] struct {
	// cache to performe the primary read from.
	cache *Cache
	// collection that contains the item.
	collection *collection[T, I]
	// index of the collection to read with.
	index I
	// isDesc indicates whether the lister should retrieve items in descending order.
	isDesc bool
	// defaultPageSize optionally defines a page size to use if
	// one is not specified by the caller. If not set then
	// [defaults.DefaultChunkSize] is used.
	defaultPageSize int
	// upstreamList is used to retrieve the items if the
	// cache is not healthy.
	upstreamList func(context.Context, int, string) ([]T, string, error)
	// nextToken is used to derive the next token returned from
	// the item at which the next page should start from.
	nextToken func(T) string
	// filter is an optional function used to exclude items from
	// cache reads.
	filter func(T) bool
	// fallbackGetter is an optional fallback if upstream does not implment ranging getters.
	// TODO(okraport): Remove when all deprecated non-paginated endpoints have been removed.
	fallbackGetter func(context.Context) ([]T, error)
}

// clipEnd takes a page of items and checks if it already contains the end token.
// If so it returns a slice up to end and modifes the next token to be empty.
func (g genericLister[T, I]) clipEnd(page []T, next, end string) ([]T, string) {
	if end == "" || len(page) == 0 {
		return page, next
	}

	// Check if the last item is within bounds to shortcircuit.
	if g.nextToken(page[len(page)-1]) < end {
		return page, next
	}

	// Consider a binary search in the future, perhaps `sort.Search`, we do not expect the memory to be
	// contiguous.
	index := slices.IndexFunc(page, func(item T) bool {
		return g.nextToken(item) >= end
	})

	if index >= 0 {
		clear(page[index:])
		return page[:index], ""
	}

	// This case should not happen, if the end is not found (index < 0) then we already should
	// have shortcircuited this logic prior.
	return page, next
}

// listRange retrieves a page of items from the configured cache collection between the start and end tokens.
// If the cache is not healthy, then the items are retrieved from the upstream backend.
// The items returend are cloned and ownership is retained by the caller.
func (l genericLister[T, I]) listRange(ctx context.Context, pageSize int, startToken, endToken string) ([]T, string, error) {
	rg, err := acquireReadGuard(l.cache, l.collection)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := l.upstreamList(ctx, pageSize, startToken)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		out, next = l.clipEnd(out, next, endToken)
		return out, next, nil
	}

	defaultPageSize := defaults.DefaultChunkSize
	if l.defaultPageSize > 0 {
		defaultPageSize = l.defaultPageSize
	}

	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	fetchFn := rg.store.cache.Ascend
	if l.isDesc {
		fetchFn = rg.store.cache.Descend
	}

	var out []T
	for sf := range fetchFn(l.index, startToken, endToken) {
		if len(out) == pageSize {
			return out, l.nextToken(sf), nil
		}

		if l.filter != nil && !l.filter(sf) {
			continue
		}
		out = append(out, l.collection.store.clone(sf))
	}

	return out, "", nil
}

// list retrieves a page of items from the configured cache collection.
// If the cache is not healthy, then the items are retrieved from the upstream backend.
// The items returend are cloned and ownership is retained by the caller.
func (l genericLister[T, I]) list(ctx context.Context, pageSize int, startToken string) ([]T, string, error) {
	out, next, err := l.listRange(ctx, pageSize, startToken, "")
	return out, next, trace.Wrap(err)
}

// Range retrieves a stream of items from the configured cache collection within the range [start, end).
// If the cache is not healthy, then the items are retrieved from the upstream backend.
func (l genericLister[T, I]) Range(ctx context.Context, start, end string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		token := start
		for {
			items, next, err := l.listRange(ctx, l.defaultPageSize, token, end)
			if err != nil {
				yield(*new(T), err)
				return
			}

			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}

			if next == "" {
				return
			}

			token = next
		}
	}
}

// RangeWithFallback retrieves a stream of items from the configured cache collection within the range [start, end).
// If the cache is not healthy, then the items are retrieved from the upstream backend. In addition, a fallback getter
// is supported if the upstream does not implement a list operation, this is only checked on the first page.
func (l genericLister[T, I]) RangeWithFallback(ctx context.Context, start, end string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		// Fallback is only allowed when configured and the entire range is requested.
		fallbackAllowed := l.fallbackGetter != nil && start == "" && end == ""

		for item, err := range l.Range(ctx, start, end) {
			if err != nil {
				if fallbackAllowed && trace.IsNotImplemented(err) {
					items, err := l.fallbackGetter(ctx)
					if err != nil {
						yield(*new(T), err)
						return
					}

					for _, item := range items {
						if !yield(item, nil) {
							return
						}
					}

					return
				}

				yield(*new(T), err)
				return
			}

			if !yield(item, nil) {
				return
			}

			// Disable the fallback once the first item is successfully yielded.
			fallbackAllowed = false
		}

	}
}
