/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package backend

import (
	"context"
	"fmt"
	"sync"
)

// registry contains globally registered functions for intializing new backend
// implementations.
var registry = make(map[string]func(context.Context, Params) (Backend, error))
var registryMu sync.RWMutex

// MustRegister registers a [Backend] implementation, panicking if it has already
// been registered. Must only be called before any possible call to [New].
func MustRegister(backend string, fn func(context.Context, Params) (Backend, error)) {
	if fn == nil {
		panic("backend registered with nil function")
	}
	if backend == "" {
		panic("backend registered without a type")
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[backend]; ok {
		panic(fmt.Sprintf("backend already registered: %v", backend))
	}
	registry[backend] = fn
}
