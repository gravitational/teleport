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
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	sortcache "github.com/gravitational/teleport/lib/utils/sortcache/v2"
)

// collectionStore is the store interface a collection needs to maintain its
// contents from fetch and event-stream updates, and to hand consistent
// snapshots to read guards.
type collectionStore[T any] interface {
	clear() error
	replace(items []T) error
	put(items ...T) error
	delete(items ...T) error
	snapshot() *sortcache.Snapshot[T]
}

// storeCollection is responsible for managing a cached resource. It is
// generic over its store shape so that legacy string-keyed collections and
// typed-index collections share one implementation; use the [collection] and
// [typedCollection] aliases rather than naming this type directly.
type storeCollection[T any, S collectionStore[T]] struct {
	// fetcher is called by fetch to retrieve and seed the
	// store with all known resources from upstream.
	fetcher func(ctx context.Context, loadSecrets bool) ([]T, error)
	// store persists all resources in memory.
	store S
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
	// changed holds the currently armed change-pulse channel, nil when no
	// listener is armed. Listeners allocate via changedSignal; commits fire
	// via firePulse. The unlistened write-path cost is one atomic load.
	changed atomic.Pointer[chan struct{}]
}

// changedSignal returns a channel that is closed after the next committed
// change to the collection: event puts and deletes, and the wholesale
// replaces performed on cache init and reset. Arm the channel BEFORE reading
// a snapshot so that a change landing after the read fires it. The channel
// fires at most once per arm; call changedSignal again to re-arm. Multiple
// concurrent listeners share one channel and are all woken by a single
// change.
func (c *storeCollection[T, _]) changedSignal() <-chan struct{} {
	for {
		if p := c.changed.Load(); p != nil {
			return *p
		}
		fresh := make(chan struct{})
		if c.changed.CompareAndSwap(nil, &fresh) {
			return fresh
		}
	}
}

// firePulse wakes all listeners armed via changedSignal, if any. When no
// listener is armed this is a single atomic load and no allocation.
func (c *storeCollection[T, _]) firePulse() {
	if c.changed.Load() == nil {
		return
	}
	if p := c.changed.Swap(nil); p != nil {
		close(*p)
	}
}

// unwrap converts an event resource into the collection's resource type. ok
// is false if the resource is excluded by the collection's filter. Delete
// events may carry a bare [types.ResourceHeader], which is converted via the
// collection's headerTransform when allowHeader is set.
func (c *storeCollection[T, _]) unwrap(r types.Resource, allowHeader bool) (out T, ok bool, err error) {
	switch t := r.(type) {
	case interface{ UnwrapT() T }:
		out = t.UnwrapT()
	case *types.ResourceHeader:
		if !allowHeader || c.headerTransform == nil {
			return out, false, trace.BadParameter("unable to convert types.ResourceHeader to %v (no transform specified, this is a bug)", reflect.TypeFor[T]())
		}
		out = c.headerTransform(t)
	case T:
		out = t
	default:
		return out, false, trace.BadParameter("unexpected type %T (expected %v)", r, reflect.TypeFor[T]())
	}

	if c.filter != nil && !c.filter(out) {
		return out, false, nil
	}
	return out, true, nil
}

// OnDeletes attempts to remove the provided resources from the store as a
// single commit. An error is returned if a resource is of an unexpected
// type, or a resource is a [types.ResourceHeader] and no headerTransform was
// specified.
//
// Resources excluded by the configured filter are skipped.
func (c *storeCollection[T, _]) OnDeletes(rs []types.Resource) error {
	items := make([]T, 0, len(rs))
	for _, r := range rs {
		t, ok, err := c.unwrap(r, true /* allow header */)
		if err != nil {
			return trace.Wrap(err)
		}
		if ok {
			items = append(items, t)
		}
	}
	if len(items) == 0 {
		return nil
	}

	if err := c.store.delete(items...); err != nil {
		return trace.Wrap(err)
	}
	c.firePulse()
	return nil
}

// OnPuts attempts to place the provided resources into the local store as a
// single commit. An error is returned if a resource is of an unexpected type.
//
// Resources excluded by the configured filter are skipped.
func (c *storeCollection[T, _]) OnPuts(rs []types.Resource) error {
	items := make([]T, 0, len(rs))
	for _, r := range rs {
		t, ok, err := c.unwrap(r, false /* puts always carry full resources */)
		if err != nil {
			return trace.Wrap(err)
		}
		if ok {
			items = append(items, t)
		}
	}
	if len(items) == 0 {
		return nil
	}

	if err := c.store.put(items...); err != nil {
		return trace.Wrap(err)
	}
	c.firePulse()
	return nil
}

// Fetch populates the store with items received by the configured fetcher.
func (c *storeCollection[T, _]) Fetch(ctx context.Context, cacheOK bool) (apply func(context.Context) error, err error) {
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
		// If this is a singleton and the fetch found nothing, or the resource
		// kind isn't cached in the current generation, wipe the store and
		// don't continue. (A singleton that was found is only updated, not
		// cleared first.)
		if c.singleton && deleteSingleton || !cacheOK {
			if err := c.store.clear(); err != nil {
				return trace.Wrap(err)
			}
			c.firePulse()
			return nil
		}

		if c.singleton && len(resources) == 0 {
			// singleton fetch succeeded but returned nothing; leave any
			// existing value in place, matching the update-or-delete-only
			// singleton contract.
			return nil
		}

		// atomically swap in the fetched generation: concurrent readers
		// observe either the complete old state or the complete new state,
		// never a partially populated store.
		if err := c.store.replace(resources); err != nil {
			return trace.Wrap(err)
		}
		c.firePulse()
		return nil
	}, nil
}

// collection is the legacy string-keyed collection shape: the second type
// parameter names the index-constant type of a [store]. It is deleted once
// the last collection migrates to [typedCollection].
type collection[T any, I comparable] = storeCollection[T, *store[T, I]]

// typedCollection is a collection backed by a [typedStore] with a
// collection-defined index-handle set IX.
type typedCollection[T any, IX any] = storeCollection[T, *typedStore[T, IX]]
