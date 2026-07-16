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

import (
	"github.com/gravitational/teleport/lib/utils/set"
	"github.com/gravitational/trace"
)

// Set is a map of type string key and struct values. Set is a thin wrapper over
// the set.Set[T] generic set type, allowing the Set to implement the
// interface(s) required for use with the expression package.
// The default value is an empty set and all methods are safe to call (even if
// the underlying map is nil).
type Set struct {
	s set.Set[string]
}

// NewSet constructs a new set from an arbitrary collection of elements
func NewSet(values ...string) Set {
	return Set{set.New(values...)}
}

// NewFlattenedSet constructs a new string set from `any` elements. It supports
// plain strings, string arrays, []any containing only strings, and other Sets
// containing strings, and returns the flattened union of all strings across all
// provided values. If any unsupported types are provided, an error is returned.
//
// Library users use this implementation to override the default `set()`
// behavior if they need to handle lists, e.g. from JSON.
func NewFlattenedSet(values ...any) (Set, error) {
	var elements []string

	for _, e := range values {
		switch element := e.(type) {
		case string:
			elements = append(elements, element)
		case []string:
			// Unlikely for data parsed via JSON, but other callers might pass
			// a []string and it's easy to support.
			elements = append(elements, element...)
		case []any:
			// Flatten []any to strings if possible. We won't bother recursing
			// further unless an actual use case for doing so presents itself;
			// for now, just expect strings.
			for _, item := range element {
				s, ok := item.(string)
				if !ok {
					return Set{}, trace.BadParameter("unsupported set element type, got %T", item)
				}
				elements = append(elements, s)
			}
		case Set:
			elements = append(elements, element.items()...)
		default:
			return Set{}, trace.BadParameter("unsupported set element type, got: %T", e)
		}
	}

	return Set{set.New(elements...)}, nil
}

// add creates a new Set containing all values in the receiver Set and adds
// [elements].
func (s Set) add(elements ...string) Set {
	if s.s == nil {
		return NewSet(elements...)
	}
	return Set{s.s.Clone().Add(elements...)}
}

// remove creates a new Set containing all values in the receiver Set, minus
// all supplied elements. Implements expression.Remover for Set.
func (s Set) remove(elements ...string) any {
	return Set{s.s.Clone().Remove(elements...)}
}

func (s Set) contains(element string) bool {
	return s.s.Contains(element)
}

func (s Set) clone() Set {
	return Set{s.s.Clone()}
}

func (s Set) items() []string {
	return s.s.ElementsNotNil()
}

// union computes the union of multiple sets
func union(sets ...Set) Set {
	result := set.New[string]()
	for _, set := range sets {
		result.Union(set.s)
	}
	return Set{result}
}
