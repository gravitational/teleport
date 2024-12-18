// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/types"
)

// genericCollection is a generic collection implementation for resource type T with collection-specific logic
// encapsulated in executor type E. Type R provides getter methods related to the collection, e.g. GetNodes(),
// GetRoles().
type genericCollection[T any, R any, E executor[T, R]] struct {
	cache *Cache
	watch types.WatchKind
	exec  E
}

// fetch implements collection
func (g *genericCollection[T, R, _]) fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error) {
	// Singleton objects will only get deleted or updated, not both
	deleteSingleton := false

	var resources []T
	if cacheOK {
		resources, err = g.exec.getAll(ctx, g.cache, g.watch.LoadSecrets)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			deleteSingleton = true
		}
	}

	return func(ctx context.Context) error {
		// Always perform the delete if this is not a singleton, otherwise
		// only perform the delete if the singleton wasn't found
		// or the resource kind isn't cached in the current generation.
		if !g.exec.isSingleton() || deleteSingleton || !cacheOK {
			if err := g.exec.deleteAll(ctx, g.cache); err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		}
		// If this is a singleton and we performed a deletion, return here
		// because we only want to update or delete a singleton, not both.
		// Also don't continue if the resource kind isn't cached in the current generation.
		if g.exec.isSingleton() && deleteSingleton || !cacheOK {
			return nil
		}
		for _, resource := range resources {
			if err := g.exec.upsert(ctx, g.cache, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

// processEvent implements collection
func (g *genericCollection[T, R, _]) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		if err := g.exec.delete(ctx, g.cache, event.Resource); err != nil {
			if !trace.IsNotFound(err) {
				g.cache.Logger.WarnContext(ctx, "Failed to delete resource", "error", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		var resource T
		var ok bool
		switch r := event.Resource.(type) {
		case types.Resource153Unwrapper:
			resource, ok = r.Unwrap().(T)
			if !ok {
				return trace.BadParameter("unexpected wrapped type %T (expected %T)", r.Unwrap(), resource)
			}

		default:
			resource, ok = event.Resource.(T)
		}

		if !ok {
			return trace.BadParameter("unexpected type %T (expected %T)", event.Resource, resource)
		}

		if err := g.exec.upsert(ctx, g.cache, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		g.cache.Logger.WarnContext(ctx, "Skipping unsupported event type", "event", event.Type)
	}
	return nil
}

// watchKind implements collection
func (g *genericCollection[T, R, _]) watchKind() types.WatchKind {
	return g.watch
}

var _ collection = (*genericCollection[types.Resource, any, executor[types.Resource, any]])(nil)

// genericCollection obtains the reader object from the executor based on the provided health status of the cache.
// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
func (c *genericCollection[T, R, _]) getReader(cacheOK bool) R {
	return c.exec.getReader(c.cache, cacheOK)
}

var _ collectionReader[any] = (*genericCollection[types.Resource, any, executor[types.Resource, any]])(nil)
