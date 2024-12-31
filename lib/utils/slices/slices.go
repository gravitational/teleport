/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package slices

import (
	"cmp"
	"slices"
)

// FilterMapUnique applies a function to all elements of a slice and collects them.
// The function returns the value to collect and whether the current element should be included.
// Returned values are sorted and deduplicated.
func FilterMapUnique[T any, S cmp.Ordered](ts []T, fn func(T) (s S, include bool)) []S {
	ss := make([]S, 0, len(ts))
	for _, t := range ts {
		if s, include := fn(t); include {
			ss = append(ss, s)
		}
	}
	slices.Sort(ss)
	return slices.Compact(ss)
}

// SliceToPointerSlice converts a slice of values to a slice of pointers to those values
func SliceToPointerSlice[T any](in []T) []*T {
	out := make([]*T, len(in))
	for i := range in {
		out[i] = &in[i]
	}
	return out
}

// PointerSliceToSlice converts a slice of pointers to values to a slice of values
func PointerSliceToSlice[T any](in []*T) []T {
	out := make([]T, len(in))
	for i := range in {
		if in[i] == nil {
			continue
		}
		out[i] = *in[i]
	}
	return out
}
