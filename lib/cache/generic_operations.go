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
		return out, next, trace.Wrap(err)
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

// genericRanger is a helper to retrieve a stream from a cache collection.
type genericRanger[T any, I comparable] struct {
	// cache to performe the primary read from.
	cache *Cache
	// collection that contains the item.
	collection *collection[T, I]
	// index of the collection to read with.
	index I
	// upstreamRange is the upstream range implementation.
	upstreamRange func(context.Context, string, string) iter.Seq2[T, error]
	// fallbackGetter is an optional fallback if upstream does not implment ranging getters.
	fallbackGetter func(context.Context) ([]T, error)
}

// Range retrieves a stream of items from the configured cache collection within the range [start, end).
// If the cache is not healthy, then the items are retrieved from the upstream backend.
func (g genericRanger[T, I]) Range(ctx context.Context, start, end string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		rg, err := acquireReadGuard(g.cache, g.collection)
		if err != nil {
			yield(*new(T), err)
			return
		}
		defer rg.Release()

		if rg.ReadCache() {
			for item := range rg.store.resources(g.index, start, end) {
				if !yield(g.collection.store.clone(item), nil) {
					return
				}
			}
			return
		}

		// Release the read guard early since all future reads will be
		// performed against the upstream.
		rg.Release()

		for item, err := range g.upstreamRange(ctx, start, end) {
			if err != nil {
				if g.fallbackGetter != nil && trace.IsNotImplemented(err) {
					items, err := g.fallbackGetter(ctx)
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
			// if we get a NotImplemented halfway through the iteration we
			// cannot retroactively not yield the items we have already successfully
			// received, so we have to forward the NotImplemented error even if we
			// have a fallback configured
			g.fallbackGetter = nil
		}
	}
}
