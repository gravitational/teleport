/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package cache

import (
	"sync"

	"github.com/gravitational/teleport/api/types"
)

// GeneratedCollectionBuilder creates a cache collection handler for a watch kind.
type GeneratedCollectionBuilder func(config Config, watch types.WatchKind) (collectionHandler, error)

var generatedCollectionBuilders struct {
	mu sync.RWMutex
	m  map[resourceKind]GeneratedCollectionBuilder
}

// RegisterGeneratedCollectionBuilder registers a generated collection builder.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedCollectionBuilder(watch types.WatchKind, builder GeneratedCollectionBuilder) {
	if watch.Kind == "" {
		panic("cache: watch kind is required")
	}
	if builder == nil {
		panic("cache: collection builder is nil")
	}

	key := resourceKindFromWatchKind(watch)

	generatedCollectionBuilders.mu.Lock()
	defer generatedCollectionBuilders.mu.Unlock()

	if generatedCollectionBuilders.m == nil {
		generatedCollectionBuilders.m = make(map[resourceKind]GeneratedCollectionBuilder)
	}
	if _, exists := generatedCollectionBuilders.m[key]; exists {
		panic("cache: duplicate generated collection builder for kind " + key.String())
	}
	generatedCollectionBuilders.m[key] = builder
}

// generatedCollectionBuilderKinds returns the resource kinds registered by
// generated collection builders. This is used in tests to verify that ForAuth
// includes all generated cache-enabled resource kinds.
func generatedCollectionBuilderKinds() []resourceKind {
	generatedCollectionBuilders.mu.RLock()
	defer generatedCollectionBuilders.mu.RUnlock()

	kinds := make([]resourceKind, 0, len(generatedCollectionBuilders.m))
	for k := range generatedCollectionBuilders.m {
		kinds = append(kinds, k)
	}
	return kinds
}

func generatedCollectionBuilder(watch types.WatchKind) (GeneratedCollectionBuilder, bool) {
	key := resourceKindFromWatchKind(watch)

	generatedCollectionBuilders.mu.RLock()
	defer generatedCollectionBuilders.mu.RUnlock()

	b, ok := generatedCollectionBuilders.m[key]
	return b, ok
}
