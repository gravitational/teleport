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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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
			types.Lock.Clone,
			map[lockIndex]func(types.Lock) string{
				lockNameIndex: types.Lock.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Lock, error) {
			locks, err := upstream.GetLocks(ctx, false)
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
