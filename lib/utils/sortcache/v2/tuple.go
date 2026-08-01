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

import (
	"cmp"
	"iter"
)

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

// Index2 is an index with a two-component compound key. It embeds
// [Index] (so Get/Ascend/Descend and registration work as for any index) and
// adds prefix-scoped iteration over the first key component.
type Index2[T any, A, B cmp.Ordered] struct {
	Index[T, Tuple2[A, B]]
}

// NewTupleIndex2 creates an index with a two-component compound key.
func NewTupleIndex2[T any, A, B cmp.Ordered](name string, key func(T) (A, B)) *Index2[T, A, B] {
	ix := &Index2[T, A, B]{}
	initIndex(&ix.Index, name,
		func(t T) Tuple2[A, B] {
			a, b := key(t)
			return T2(a, b)
		},
		func(x, y Tuple2[A, B]) bool { return x.Compare(y) < 0 },
	)
	return ix
}

// AscendPrefix iterates all values whose first key component equals a, in
// ascending order of the second component.
func (ix *Index2[T, A, B]) AscendPrefix(s *Snapshot[T], a A) iter.Seq[T] {
	start, stop := Prefix2[A, B](a)
	return ix.Ascend(s, start, stop)
}

// AscendPrefixFrom iterates values whose first key component equals a,
// ascending, starting at second component from (inclusive). This is the
// shape of pagination resumption within a prefix: pass the page token as
// from to continue a listing.
func (ix *Index2[T, A, B]) AscendPrefixFrom(s *Snapshot[T], a A, from B) iter.Seq[T] {
	return ix.Ascend(s, Inclusive(T2(a, from)), Inclusive(Tuple2Max[A, B](a)))
}

// DescendPrefix iterates all values whose first key component equals a, in
// descending order of the second component.
func (ix *Index2[T, A, B]) DescendPrefix(s *Snapshot[T], a A) iter.Seq[T] {
	start, stop := Prefix2[A, B](a)
	return ix.Descend(s, stop, start)
}

// DescendPrefixFrom iterates values whose first key component equals a,
// descending, starting at second component from (inclusive).
func (ix *Index2[T, A, B]) DescendPrefixFrom(s *Snapshot[T], a A, from B) iter.Seq[T] {
	return ix.Descend(s, Inclusive(T2(a, from)), Inclusive(Tuple2Min[A, B](a)))
}

// NewSecondaryIndex creates an index sorted by the given key component and
// disambiguated by the primary index's key: the effective compound key is
// (key(t), primary.KeyOf(t)).
//
// This is the constructor for the overwhelmingly common secondary-index
// shape, and it is immune to component-ordering mistakes because the caller
// supplies only the leading component. It also makes the uniqueness
// discipline structural: two distinct values (distinct primary keys) cannot
// collide on a secondary index built this way.
func NewSecondaryIndex[T any, K, PK cmp.Ordered](name string, primary *Index[T, PK], key func(T) K) *Index2[T, K, PK] {
	ix := &Index2[T, K, PK]{}
	initIndex(&ix.Index, name,
		func(t T) Tuple2[K, PK] {
			return T2(key(t), primary.KeyOf(t))
		},
		func(x, y Tuple2[K, PK]) bool { return x.Compare(y) < 0 },
	)
	return ix
}

// Index3 is an index with a three-component compound key. It embeds [Index]
// and adds prefix-scoped iteration over the two leading key components.
type Index3[T any, A, B, C cmp.Ordered] struct {
	Index[T, Tuple3[A, B, C]]
}

// NewTupleIndex3 creates an index with a three-component compound key.
func NewTupleIndex3[T any, A, B, C cmp.Ordered](name string, key func(T) (A, B, C)) *Index3[T, A, B, C] {
	ix := &Index3[T, A, B, C]{}
	initIndex(&ix.Index, name,
		func(t T) Tuple3[A, B, C] {
			a, b, c := key(t)
			return T3(a, b, c)
		},
		func(x, y Tuple3[A, B, C]) bool { return x.Compare(y) < 0 },
	)
	return ix
}

// AscendPrefix iterates all values whose two leading key components equal
// (a, b), in ascending order of the third component.
func (ix *Index3[T, A, B, C]) AscendPrefix(s *Snapshot[T], a A, b B) iter.Seq[T] {
	start, stop := Prefix3[A, B, C](a, b)
	return ix.Ascend(s, start, stop)
}

// AscendPrefixFrom iterates values whose two leading key components equal
// (a, b), ascending, starting at third component from (inclusive).
func (ix *Index3[T, A, B, C]) AscendPrefixFrom(s *Snapshot[T], a A, b B, from C) iter.Seq[T] {
	return ix.Ascend(s, Inclusive(T3(a, b, from)), Inclusive(Tuple3Max[A, B, C](a, b)))
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
