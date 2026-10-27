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

package itertools

import (
	"iter"
)

// FilterMap returns an iterator over seq that applies fn to each value v in seq,
// and yields the result if fn(v) returns ok == true.
func FilterMap[In, Out any](seq iter.Seq[In], fn func(In) (Out, bool)) iter.Seq[Out] {
	return func(yield func(Out) bool) {
		for in := range seq {
			if out, ok := fn(in); ok && !yield(out) {
				return
			}
		}
	}
}
