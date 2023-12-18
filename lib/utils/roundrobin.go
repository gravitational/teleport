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

package utils

import "sync/atomic"

// RoundRobin is a helper for distributing load across multiple resources in a round-robin
// fashion.
type RoundRobin[T any] struct {
	ct    atomic.Uint64
	items []T
}

// NewRoundRobin creates a new round-robin inst
func NewRoundRobin[T any](items []T) *RoundRobin[T] {
	return &RoundRobin[T]{
		items: items,
	}
}

// Next gets the next item that is up for use.
func (r *RoundRobin[T]) Next() T {
	n := r.ct.Add(1) - 1
	l := uint64(len(r.items))
	return r.items[int(n%l)]
}

// ForEach applies the supplied closure to each item.
func (r *RoundRobin[T]) ForEach(fn func(T)) {
	for _, item := range r.items {
		fn(item)
	}
}
