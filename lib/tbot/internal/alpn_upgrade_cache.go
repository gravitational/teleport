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

package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/singleflight"

	apiclient "github.com/gravitational/teleport/api/client"
)

// NewALPNUpgradeCache creates a new ALPNUpgradeCache with the given logger.
func NewALPNUpgradeCache(log *slog.Logger) *ALPNUpgradeCache {
	if log == nil {
		log = slog.Default()
	}
	return &ALPNUpgradeCache{log: log}
}

// ALPNUpgradeCache can be used to determine whether an ALPN upgrade request is
// required.
type ALPNUpgradeCache struct {
	log *slog.Logger

	mu    sync.Mutex
	cache map[string]bool
	group singleflight.Group
}

// IsUpgradeRequired determines whether an ALPN upgrade request is required and
// caches the result.
func (a *ALPNUpgradeCache) IsUpgradeRequired(ctx context.Context, addr string, insecure bool) (bool, error) {
	key := fmt.Sprintf("%s-%t", addr, insecure)

	a.mu.Lock()
	if a.cache == nil {
		a.cache = make(map[string]bool)
	}
	v, ok := a.cache[key]
	if ok {
		a.mu.Unlock()
		return v, nil
	}
	a.mu.Unlock()

	val, err, _ := a.group.Do(key, func() (any, error) {
		// Recheck the cache in case we've just missed a previous group
		// completing
		a.mu.Lock()
		v, ok := a.cache[key]
		if ok {
			a.mu.Unlock()
			return v, nil
		}
		a.mu.Unlock()

		// Ok, now we know for sure that the work hasn't already been done or
		// isn't in flight, we can complete it.
		a.log.DebugContext(ctx, "Testing ALPN upgrade necessary", "addr", addr, "insecure", insecure)
		v = apiclient.IsALPNConnUpgradeRequired(ctx, addr, insecure)
		a.log.DebugContext(ctx, "Tested ALPN upgrade necessary", "addr", addr, "insecure", insecure, "result", v)
		if err := ctx.Err(); err != nil {
			// Check for case where false is returned because client canceled ctx.
			// We don't want to cache this result.
			return v, trace.Wrap(err)
		}

		a.mu.Lock()
		a.cache[key] = v
		a.mu.Unlock()
		return v, nil
	})
	return val.(bool), err
}
