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
	"github.com/gravitational/trace"
)

// Dict is a map of type string key and Set values.
type Dict map[string]Set

// NewDict returns a dict initialized with the key-value pairs as specified in
// [pairs].
func NewDict(pairs ...pair) (Dict, error) {
	d := make(Dict, len(pairs))
	for _, p := range pairs {
		k, ok := p.first.(string)
		if !ok {
			return nil, trace.BadParameter("dict keys must have type string, got %T", p.first)
		}
		v, ok := p.second.(Set)
		if !ok {
			return nil, trace.BadParameter("dict values must have type set, got %T", p.second)
		}
		d[k] = v
	}
	return d, nil
}

func (d Dict) addValues(key string, values ...string) Dict {
	out := d.clone()
	s, present := out[key]
	if !present {
		out[key] = NewSet(values...)
		return out
	}
	// Calling s.add would do an unnecessary extra copy since we already
	// cloned the whole Dict. s.s.Add adds to the existing cloned set.
	s.s.Add(values...)
	return out
}

func (d Dict) put(key string, value Set) Dict {
	out := d.clone()
	out[key] = value
	return out
}

func (d Dict) remove(keys ...string) any {
	out := d.clone()
	for _, key := range keys {
		delete(out, key)
	}
	return out
}

func (d Dict) clone() Dict {
	out := make(Dict, len(d))
	for key, set := range d {
		out[key] = set.clone()
	}
	return out
}

// Get implements typical.Getter[set]
func (d Dict) Get(key string) (Set, error) {
	return d[key], nil
}

type pair struct {
	first, second any
}
