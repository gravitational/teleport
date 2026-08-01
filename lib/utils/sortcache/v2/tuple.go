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

package sortcache

import "cmp"

// Tuple2 is a compound key of two ordered components, compared
// lexicographically by component. It replaces `a + "/" + b` string
// concatenation: components are compared individually, so values containing
// separator characters cannot collide or produce misordered keys.
//
// The unexported sentinel field supports prefix ranges: [Tuple2Min] and
// [Tuple2Max] produce pseudo-keys that sort before/after every real key
// sharing the same First component. Sentinel keys are only meaningful as
// range bounds; they are never stored.
type Tuple2[A, B cmp.Ordered] struct {
	First  A
	Second B
	inf    int8
}

// T2 constructs a Tuple2 key.
func T2[A, B cmp.Ordered](a A, b B) Tuple2[A, B] {
	return Tuple2[A, B]{First: a, Second: b}
}

// Tuple2Min returns a bound key sorting before all keys with the given First.
func Tuple2Min[A, B cmp.Ordered](a A) Tuple2[A, B] {
	return Tuple2[A, B]{First: a, inf: -1}
}

// Tuple2Max returns a bound key sorting after all keys with the given First.
func Tuple2Max[A, B cmp.Ordered](a A) Tuple2[A, B] {
	return Tuple2[A, B]{First: a, inf: 1}
}

// Compare compares two keys lexicographically by component.
func (t Tuple2[A, B]) Compare(o Tuple2[A, B]) int {
	if c := cmp.Compare(t.First, o.First); c != 0 {
		return c
	}
	if t.inf != o.inf {
		return cmp.Compare(t.inf, o.inf)
	}
	if t.inf != 0 {
		return 0
	}
	return cmp.Compare(t.Second, o.Second)
}

// Tuple3 is a compound key of three ordered components, compared
// lexicographically by component. See [Tuple2].
type Tuple3[A, B, C cmp.Ordered] struct {
	First  A
	Second B
	Third  C
	inf    int8
}

// T3 constructs a Tuple3 key.
func T3[A, B, C cmp.Ordered](a A, b B, c C) Tuple3[A, B, C] {
	return Tuple3[A, B, C]{First: a, Second: b, Third: c}
}

// Tuple3Min returns a bound key sorting before all keys with the given
// leading components. Pass only First to bound a one-component prefix.
func Tuple3Min[A, B, C cmp.Ordered](a A, b B) Tuple3[A, B, C] {
	return Tuple3[A, B, C]{First: a, Second: b, inf: -1}
}

// Tuple3Max returns a bound key sorting after all keys with the given
// leading components.
func Tuple3Max[A, B, C cmp.Ordered](a A, b B) Tuple3[A, B, C] {
	return Tuple3[A, B, C]{First: a, Second: b, inf: 1}
}

// Compare compares two keys lexicographically by component.
func (t Tuple3[A, B, C]) Compare(o Tuple3[A, B, C]) int {
	if c := cmp.Compare(t.First, o.First); c != 0 {
		return c
	}
	// The sentinel of a Tuple3 bound applies after (First, Second); a bound
	// over only First is expressed with the zero Second and inf=-1 /
	// max-Second semantics are not needed for the prototype's use cases.
	if c := cmp.Compare(t.Second, o.Second); c != 0 {
		return c
	}
	if t.inf != o.inf {
		return cmp.Compare(t.inf, o.inf)
	}
	if t.inf != 0 {
		return 0
	}
	return cmp.Compare(t.Third, o.Third)
}

// NewTupleIndex2 creates an index with a two-component compound key.
func NewTupleIndex2[T any, A, B cmp.Ordered](name string, key func(T) (A, B)) *Index[T, Tuple2[A, B]] {
	return NewIndexFunc(name,
		func(t T) Tuple2[A, B] {
			a, b := key(t)
			return T2(a, b)
		},
		func(x, y Tuple2[A, B]) bool { return x.Compare(y) < 0 },
	)
}

// NewTupleIndex3 creates an index with a three-component compound key.
func NewTupleIndex3[T any, A, B, C cmp.Ordered](name string, key func(T) (A, B, C)) *Index[T, Tuple3[A, B, C]] {
	return NewIndexFunc(name,
		func(t T) Tuple3[A, B, C] {
			a, b, c := key(t)
			return T3(a, b, c)
		},
		func(x, y Tuple3[A, B, C]) bool { return x.Compare(y) < 0 },
	)
}

// Prefix2 returns bounds covering every key whose First component equals a.
// This is the typed replacement for `Ascend(prefix+"/", NextKey(prefix+"/"))`.
func Prefix2[A, B cmp.Ordered](a A) (start, stop Bound[Tuple2[A, B]]) {
	return Inclusive(Tuple2Min[A, B](a)), Inclusive(Tuple2Max[A, B](a))
}

// Prefix3 returns bounds covering every key whose First and Second
// components equal a and b.
func Prefix3[A, B, C cmp.Ordered](a A, b B) (start, stop Bound[Tuple3[A, B, C]]) {
	return Inclusive(Tuple3Min[A, B, C](a, b)), Inclusive(Tuple3Max[A, B, C](a, b))
}
