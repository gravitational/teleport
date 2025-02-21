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
type collection[T any] struct {
	getAll          func(ctx context.Context, loadSecrets bool) ([]T, error)
	store           *store[T]
	watch           types.WatchKind
	headerTransform func(hdr *types.ResourceHeader) T
	filter          func(T) bool
	singleton       bool
}

func (c collection[_]) watchKind() types.WatchKind {
	return c.watch
}

func (c *collection[T]) onDelete(r types.Resource) error {
	switch t := r.(type) {
	case types.Resource153Unwrapper:
		unwrapped := t.Unwrap()
		tt, ok := unwrapped.(T)
		if !ok {
			return trace.BadParameter("unexpected wrapped type %T (expected %v)", unwrapped, reflect.TypeOf((*T)(nil)).Elem())
		}

		return trace.Wrap(c.store.delete(tt))
	case *types.ResourceHeader:
		if c.headerTransform == nil {
			return trace.BadParameter("unable to convert types.ResourceHeader to %v (no transform specified, this is a bug)", reflect.TypeOf((*T)(nil)).Elem())
		}

		return trace.Wrap(c.store.delete(c.headerTransform(t)))
	case T:
		return trace.Wrap(c.store.delete(t))
	default:
		return trace.BadParameter("unexpected type %T (expected %v)", r, reflect.TypeOf((*T)(nil)).Elem())
	}
}

func (c *collection[T]) onUpdate(r types.Resource) error {
	switch t := r.(type) {
	case types.Resource153Unwrapper:
		unwrapped := t.Unwrap()
		tt, ok := unwrapped.(T)
		if !ok {
			return trace.BadParameter("unexpected wrapped type %T (expected %v)", unwrapped, reflect.TypeOf((*T)(nil)).Elem())
		}

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
		return trace.BadParameter("unexpected type %T (expected %v)", r, reflect.TypeOf((*T)(nil)).Elem())
	}
}

func (c collection[T]) fetch(ctx context.Context, cacheOK bool) (apply func(context.Context) error, err error) {
	// Singleton objects will only get deleted or updated, not both
	deleteSingleton := false

	var resources []T
	if cacheOK {
		resources, err = c.getAll(ctx, c.watch.LoadSecrets)
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

type resourceHandler interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// onDelete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to clear.
	onDelete(t types.Resource) error
	// onUpdate will update a single target resource from the cache
	onUpdate(t types.Resource) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

type collections struct {
	byKind map[resourceKind]resourceHandler

	staticTokens    *collection[types.StaticTokens]
	certAuthorities *collection[types.CertAuthority]
	users           *collection[types.User]
}

func setupCollections(c Config, watches []types.WatchKind) (*collections, error) {
	out := &collections{
		byKind: make(map[resourceKind]resourceHandler, 1),
	}

	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)

		switch watch.Kind {
		case types.KindStaticTokens:
			collect, err := newStaticTokensCollection(c.ClusterConfig, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.staticTokens = collect
			out.byKind[resourceKind] = out.staticTokens
		case types.KindCertAuthority:
			collect, err := newCertAuthorityCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.certAuthorities = collect
			out.byKind[resourceKind] = out.certAuthorities
		case types.KindUser:
			collect, err := newUserCollection(c.Users, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.users = collect
			out.byKind[resourceKind] = out.users
		}
	}

	return out, nil
}
