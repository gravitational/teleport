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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

type lockIndex string

const lockNameIndex lockIndex = "name"

func newLockCollection(upstream services.Access, w types.WatchKind) (*collection[types.Lock, lockIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Access")
	}

	return &collection[types.Lock, lockIndex]{
		store: newStore(
			types.KindLock,
			types.Lock.Clone,
			map[lockIndex]func(types.Lock) string{
				lockNameIndex: types.Lock.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Lock, error) {
			locks, err := clientutils.CollectWithFallback(
				ctx,
				func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
					var noFilter *types.LockFilter
					return upstream.ListLocks(ctx, limit, start, noFilter)
				},
				func(ctx context.Context) ([]types.Lock, error) {
					const inForceOnlyFalse = false
					return upstream.GetLocks(ctx, inForceOnlyFalse)
				},
			)

			return locks, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Lock {
			return &types.LockV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetLock gets a lock by name.
func (c *Cache) GetLock(ctx context.Context, name string) (types.Lock, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetLock")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.Lock, lockIndex]{
		cache:      c,
		collection: c.collections.locks,
		index:      lockNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.Lock, error) {
			upstreamRead = true
			lock, err := c.Config.Access.GetLock(ctx, name)
			return lock, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateConfig)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if lock, err := c.Config.Access.GetLock(ctx, name); err == nil {
			return lock, nil
		}
	}
	return out, trace.Wrap(err)
}

// GetLocks gets all/in-force locks that match at least one of the targets
// when specified.
func (c *Cache) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetLocks")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.locks)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := c.Config.Access.GetLocks(ctx, inForceOnly, targets...)
		return users, trace.Wrap(err)
	}

	var locks []types.Lock
	for lock := range rg.store.resources(lockNameIndex, "", "") {
		if inForceOnly && !lock.IsInForce(time.Now()) {
			continue
		}
		// If no targets specified, return all of the found/in-force locks.
		if len(targets) == 0 {
			locks = append(locks, lock.Clone())
			continue
		}
		// Otherwise, use the targets as filters.
		for _, target := range targets {
			if target.Match(lock) {
				locks = append(locks, lock.Clone())
				break
			}
		}
	}

	return locks, nil
}

func matchLock(lock types.Lock, filter *types.LockFilter, now time.Time) bool {
	if filter == nil {
		return true
	}

	if filter.InForceOnly && !lock.IsInForce(now) {
		return false
	}

	// If no targets specified, return all of the found/in-force locks.
	if len(filter.Targets) == 0 {
		return true
	}

	// Otherwise, use the targets as filters.
	for _, target := range filter.Targets {
		if target.Match(lock) {
			return true
		}
	}

	return false
}

// RangeLocks returns locks within the range [start, end) matching a given filter.
func (c *Cache) RangeLocks(ctx context.Context, start, end string, filter *types.LockFilter) iter.Seq2[types.Lock, error] {
	lister := genericLister[types.Lock, lockIndex]{
		cache:      c,
		collection: c.collections.locks,
		index:      lockNameIndex,
		upstreamList: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
			return c.Config.Access.ListLocks(ctx, limit, start, filter)
		},
		filter: func(l types.Lock) bool {
			return matchLock(l, filter, c.Clock.Now())
		},
		nextToken: types.Lock.GetName,
		// TODO(lokraszewski): DELETE IN v21.0.0
		fallbackGetter: func(ctx context.Context) ([]types.Lock, error) {
			if filter != nil {
				targets := make([]types.LockTarget, 0, len(filter.Targets))
				for _, tgt := range filter.Targets {
					targets = append(targets, *tgt)
				}
				return c.Config.Access.GetLocks(ctx, filter.InForceOnly, targets...)
			} else {
				const inForceOnlyFalse = false
				return c.Config.Access.GetLocks(ctx, inForceOnlyFalse)
			}
		},
	}

	return func(yield func(types.Lock, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeLocks")
		defer span.End()

		for db, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(db, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}

// ListLocks returns a page of locks matching a filter
func (c *Cache) ListLocks(ctx context.Context, limit int, startKey string, filter *types.LockFilter) ([]types.Lock, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListLocks")
	defer span.End()

	lister := genericLister[types.Lock, lockIndex]{
		cache:      c,
		collection: c.collections.locks,
		index:      lockNameIndex,
		upstreamList: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
			return c.Config.Access.ListLocks(ctx, limit, start, filter)
		},
		nextToken: types.Lock.GetName,
		filter: func(l types.Lock) bool {
			return matchLock(l, filter, c.Clock.Now())
		},
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}
