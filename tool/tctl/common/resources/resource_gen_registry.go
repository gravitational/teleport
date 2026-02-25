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

package resources

import "sync"

// GeneratedHandlerFactory creates a tctl resource handler.
type GeneratedHandlerFactory func() Handler

var generatedHandlers struct {
	mu sync.RWMutex
	m  map[string]GeneratedHandlerFactory
}

// RegisterGeneratedHandler registers a generated tctl handler for a kind.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedHandler(factory GeneratedHandlerFactory) {
	if factory == nil {
		panic("resources: handler factory is nil")
	}

	h := factory()
	if h.kind == "" {
		panic("resources: handler factory returned handler with empty kind")
	}

	generatedHandlers.mu.Lock()
	defer generatedHandlers.mu.Unlock()

	if generatedHandlers.m == nil {
		generatedHandlers.m = make(map[string]GeneratedHandlerFactory)
	}
	if _, exists := generatedHandlers.m[h.kind]; exists {
		panic("resources: duplicate generated handler for kind " + h.kind)
	}
	generatedHandlers.m[h.kind] = factory
}

func applyGeneratedHandlers(base map[string]Handler) map[string]Handler {
	generatedHandlers.mu.RLock()
	defer generatedHandlers.mu.RUnlock()

	if len(generatedHandlers.m) == 0 {
		return base
	}
	for kind, factory := range generatedHandlers.m {
		if _, exists := base[kind]; exists {
			panic("resources: generated handler conflicts with existing handler for kind " + kind)
		}
		base[kind] = factory()
	}
	return base
}
