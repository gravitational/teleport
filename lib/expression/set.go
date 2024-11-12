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

package expression

import "github.com/gravitational/teleport/lib/utils"

// Set is a map of type string key and struct values. Set is a thin wrapper over
// the utils.Set[T] generic set type, allowing the Set to implement the
// interface(s) required for use with the expression package.
type Set struct {
	utils.Set[string]
}

// NewSet constructs a new set from an arbitrary collection of elements
func NewSet(values ...string) Set {
	return Set{utils.NewSet(values...)}
}

// remove creates a new Set containing all values in the receiver Set, minus
// all supplied elements. Implements expression.Remover for Set.
func (s Set) remove(elements ...string) any {
	return Set{s.Set.Clone().Remove(elements...)}
}

// union computes the union of multiple sets
func union(sets ...Set) Set {
	result := utils.NewSet[string]()
	for _, set := range sets {
		result.Union(set.Set)
	}
	return Set{result}
}
