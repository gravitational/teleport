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
	"golang.org/x/exp/maps"
)

// Set is a map of type string key and struct values.
type Set map[string]struct{}

// NewSet returns Set from given string slice.
func NewSet(values ...string) Set {
	s := make(Set, len(values))
	for _, value := range values {
		s[value] = struct{}{}
	}
	return s
}

func (s Set) add(values ...string) Set {
	out := s.clone()
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func (s Set) contains(str string) bool {
	_, ok := s[str]
	return ok
}

func (s Set) remove(values ...string) any {
	out := s.clone()
	for _, value := range values {
		delete(out, value)
	}
	return out
}

func (s Set) transform(f func(string) string) Set {
	out := make(Set, len(s))
	for str := range s {
		out[f(str)] = struct{}{}
	}
	return out
}

func (s Set) clone() Set {
	return maps.Clone(s)
}

func (s Set) items() []string {
	return maps.Keys(s)
}

func union(sets ...Set) Set {
	result := make(Set)
	for _, s := range sets {
		for v := range s {
			result[v] = struct{}{}
		}
	}
	return result
}
