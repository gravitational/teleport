/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iter

import "github.com/gravitational/trace"

// joinSorted joins two sorted iterators into a single sorted iterator.
type joinSorted[T any] struct {
	less         func(l, r T) bool
	left, right  Iter[T]
	litem, ritem T
	lskip, rskip bool
	ldone, rdone bool
	lerr, rerr   error
}

func (iter *joinSorted[T]) Next() bool {
	// if left iterator has not terminated and we didn't end the
	// last advance in a left-skip state (i.e. yielding the right item),
	// then try to get the next item from the left iterator.
	if !iter.ldone && !iter.lskip {
		if iter.left.Next() {
			iter.litem = iter.left.Item()
		} else {
			iter.ldone = true
			iter.lerr = iter.left.Done()
		}
	}

	// if right iterator has not terminated and we didn't end the
	// last advance in the right-skip state (i.e. yielding the left item),
	// then try to get the next item from the right iterator.
	if !iter.rdone && !iter.rskip {
		if iter.right.Next() {
			iter.ritem = iter.right.Item()
		} else {
			iter.rdone = true
			iter.rerr = iter.right.Done()
		}
	}

	// both iterators halted, halt outer iteration
	if iter.ldone && iter.rdone {
		return false
	}

	// left iterator has halted, check halt state
	if iter.ldone {
		if iter.lerr != nil {
			return false
		}
		iter.lskip = true
		iter.rskip = false
		return true
	}

	// right iterator has halted, check halt state
	if iter.rdone {
		if iter.rerr != nil {
			return false
		}
		iter.rskip = true
		iter.lskip = false
		return true
	}

	// less checks if right is less than left, if it isn't
	// we yield the left item since we want the equal items
	// from the left iterator to show up to the left of items
	// of the right iterator.
	if iter.less(iter.ritem, iter.litem) {
		iter.lskip = true
		iter.rskip = false
	} else {
		iter.rskip = true
		iter.lskip = false
	}

	return true
}

func (iter *joinSorted[T]) Item() T {
	if iter.lskip {
		return iter.ritem
	}
	return iter.litem
}

func (iter *joinSorted[T]) Done() error {
	if !iter.ldone {
		iter.lerr = iter.left.Done()
		iter.ldone = true
	}
	if !iter.rdone {
		iter.rerr = iter.right.Done()
		iter.rdone = true
	}

	return trace.NewAggregate(iter.lerr, iter.rerr)
}

// JoinSorted joins two sorted iterators into a single sorted iterator consisting of items
// from both iterators. Items from the left iterator appear before items from the right
// iterator if they are equal.
func JoinSorted[T any](left, right Iter[T], less func(i, j T) bool) Iter[T] {
	return &joinSorted[T]{
		left:  left,
		right: right,
		less:  less,
	}
}

type dedupSorted[T any] struct {
	inner  Iter[T]
	equals func(i, j T) bool
	item   T
	first  bool
}

func (iter *dedupSorted[T]) Next() bool {
	for {
		if !iter.inner.Next() {
			return false
		}
		next := iter.inner.Item()
		if !iter.first && iter.equals(iter.item, next) {
			continue
		}
		iter.item = next
		iter.first = false
		return true
	}
}

func (iter *dedupSorted[T]) Item() T {
	return iter.item
}

func (iter *dedupSorted[T]) Done() error {
	return iter.inner.Done()
}

// DedupSorted deduplicates the items within a sorted iterator.
func DedupSorted[T any](iter Iter[T], equals func(i, j T) bool) Iter[T] {
	return &dedupSorted[T]{
		inner:  iter,
		equals: equals,
		first:  true,
	}
}
