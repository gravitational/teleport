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
	"reflect"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// collection is responsible for managing a cached resource.
type collection[T any, I comparable] struct {
	// fetcher is called by fetch to retrieve and seed the
	// store with all known resources from upstream.
	fetcher func(ctx context.Context, loadSecrets bool) ([]T, error)
	// store persists all resources in memory.
	store *store[T, I]
	// watch contains the kind of resource being monitored.
	watch types.WatchKind
	// headerTransform is used when handling delete events in [onDelete]. Since
	// [types.OpDelete] events only contain information about the resource key,
	// most event handlers only emit a [types.ResourceHeader] which has enough
	// information to identify a resource. Some resources do emit a half
	// populated [T], or have enough information from the key to emit a full [T].
	//
	// If this optional transformation is supplied it will be called when
	// processing delete events before attempting to delete the resource
	// from the store.
	headerTransform func(hdr *types.ResourceHeader) T
	// filter is an optional function used to prevent some resources
	// from being persisted in the store.
	filter func(T) bool
	// singleton indicates if the resource should only ever have a single item.
	// TODO(tross|fspmarshall|espadolini) investigate if special singleton
	// behavior can be removed.
	singleton bool
}

func (c collection[_, _]) watchKind() types.WatchKind {
	return c.watch
}

// onDelete attempts to remove the provided resource from the store.
// An error is returned if the resource is of an unexpected type, or
// the resource is a [types.ResourceHeader] and no headerTransform was
// specified.
//
// This is a no-op if the configured filter does not return true.
func (c *collection[T, _]) onDelete(r types.Resource) error {
	switch t := r.(type) {
	case interface{ UnwrapT() T }:
		tt := t.UnwrapT()
		if c.filter != nil && !c.filter(tt) {
			return nil
		}

		return trace.Wrap(c.store.delete(tt))
	case *types.ResourceHeader:
		if c.headerTransform == nil {
			return trace.BadParameter("unable to convert types.ResourceHeader to %v (no transform specified, this is a bug)", reflect.TypeFor[T]())
		}

		tt := c.headerTransform(t)
		if c.filter != nil && !c.filter(tt) {
			return nil
		}

		return trace.Wrap(c.store.delete(tt))
	case T:
		if c.filter != nil && !c.filter(t) {
			return nil
		}

		return trace.Wrap(c.store.delete(t))
	default:
		return trace.BadParameter("unexpected type %T (expected %v)", r, reflect.TypeFor[T]())
	}
}

// onUpdate attempts to place the resource into the local store.
// An error is returned if the resource is of an unexpected type
//
// This is a no-op if the configured filter does not return true.
func (c *collection[T, _]) onPut(r types.Resource) error {
	switch t := r.(type) {
	case interface{ UnwrapT() T }:
		tt := t.UnwrapT()
		if c.filter != nil && !c.filter(tt) {
			return nil
		}

		c.store.put(tt)
		return nil
	case T:
		if c.filter != nil && !c.filter(t) {
			return nil
		}

		c.store.put(t)
		return nil
	default:
		return trace.BadParameter("unexpected type %T (expected %v)", r, reflect.TypeFor[T]())
	}
}

// fetch populates the store with items received by the configured fetcher.
func (c collection[T, _]) fetch(ctx context.Context, cacheOK bool) (apply func(context.Context) error, err error) {
	// Singleton objects will only get deleted or updated, not both
	// TODO(tross|fspmarshall|espadolini) investigate if special singleton
	// behavior can be removed.
	deleteSingleton := false

	var resources []T
	if cacheOK {
		resources, err = c.fetcher(ctx, c.watch.LoadSecrets)
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
		if !c.singleton || deleteSingleton || !cacheOK {
			if err := c.store.clear(); err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		}
		// If this is a singleton and we performed a deletion, return here
		// because we only want to update or delete a singleton, not both.
		// Also don't continue if the resource kind isn't cached in the current generation.
		if c.singleton && deleteSingleton || !cacheOK {
			return nil
		}
		for _, resource := range resources {
			if err := c.store.put(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}
