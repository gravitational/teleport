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

// CollectValues applies a function to all elements of a slice and collects them.
// The function returns the value to collect and whether the current element should be skipped.
// Returned values are sorted and deduplicated.
func CollectValues[T any, S cmp.Ordered](ts []T, fn func(T) (s S, skip bool)) []S {
	ss := make([]S, 0, len(ts))
	for _, t := range ts {
		s, skip := fn(t)
		if skip {
			continue
		}
		ss = append(ss, s)
	}
	slices.Sort(ss)
	return slices.Compact(ss)
}
