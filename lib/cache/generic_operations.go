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

// list retrieves a page of items from the configured cache collection.
// If the cache is not healthy, then the items are retrieved from the upstream backend.
// The items returend are cloned and ownership is retained by the caller.
func (l genericLister[T, I]) list(ctx context.Context, pageSize int, startToken string) ([]T, string, error) {
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

	var out []T
	for sf := range rg.store.resources(l.index, startToken, "") {
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
