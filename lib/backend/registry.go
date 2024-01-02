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
)

// registry contains globally registered functions for intializing new backend
// implementations.
var registry = map[string]newbk{}

// newbk intializes a [Backend].
type newbk interface {
	new(context.Context, Params) (Backend, error)
}

// newbkfn is a function that can initialize a [Backend]. The function can return
// any type that implements [Backend].
type newbkfn[T Backend] func(context.Context, Params) (T, error)

// new converts a generic backend type to the [Backend] interface. This allows
// any backend intialization function to implement the newbk interface.
func (fn newbkfn[T]) new(ctx context.Context, params Params) (Backend, error) {
	return fn(ctx, params)
}

// MustRegister registers a [Backend] implementation. Panicking if it has already
// been registered.
func MustRegister[T Backend](fn newbkfn[T], types ...string) {
	for _, t := range types {
		if _, ok := registry[t]; ok {
			panic(fmt.Sprintf("backend already registered: %s", t))
		}
	}
	for _, bkType := range types {
		registry[bkType] = fn
	}
}
