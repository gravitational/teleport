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

package stream

import "github.com/gravitational/trace"

// joinSorted joins two sorted streams into a single sorted stream.
type joinSorted[T any] struct {
	less         func(l, r T) bool
	left, right  Stream[T]
	litem, ritem T
	lskip, rskip bool
	ldone, rdone bool
	lerr, rerr   error
}

func (stream *joinSorted[T]) Next() bool {
	// if left stream has not terminated and we didn't end the
	// last advance in a left-skip state (i.e. yielding the right item),
	// then try to get the next item from the left stream.
	if !stream.ldone && !stream.lskip {
		if stream.left.Next() {
			stream.litem = stream.left.Item()
		} else {
			stream.ldone = true
			stream.lerr = stream.left.Done()
		}
	}

	// if right stream has not terminated and we didn't end the
	// last advance in the right-skip state (i.e. yielding the left item),
	// then try to get the next item from the right stream.
	if !stream.rdone && !stream.rskip {
		if stream.right.Next() {
			stream.ritem = stream.right.Item()
		} else {
			stream.rdone = true
			stream.rerr = stream.right.Done()
		}
	}

	// both streams halted, halt outer streamation
	if stream.ldone && stream.rdone {
		return false
	}

	// left stream has halted, check halt state
	if stream.ldone {
		if stream.lerr != nil {
			return false
		}
		stream.lskip = true
		stream.rskip = false
		return true
	}

	// right stream has halted, check halt state
	if stream.rdone {
		if stream.rerr != nil {
			return false
		}
		stream.rskip = true
		stream.lskip = false
		return true
	}

	// less checks if right is less than left, if it isn't
	// we yield the left item since we want the equal items
	// from the left stream to show up to the left of items
	// of the right stream.
	if stream.less(stream.ritem, stream.litem) {
		stream.lskip = true
		stream.rskip = false
	} else {
		stream.rskip = true
		stream.lskip = false
	}

	return true
}

func (stream *joinSorted[T]) Item() T {
	if stream.lskip {
		return stream.ritem
	}
	return stream.litem
}

func (stream *joinSorted[T]) Done() error {
	if !stream.ldone {
		stream.lerr = stream.left.Done()
		stream.ldone = true
	}
	if !stream.rdone {
		stream.rerr = stream.right.Done()
		stream.rdone = true
	}

	return trace.NewAggregate(stream.lerr, stream.rerr)
}

// JoinSorted joins two sorted streams into a single sorted stream consisting of items
// from both streams. Items from the left stream appear before items from the right
// stream if they are equal.
func JoinSorted[T any](left, right Stream[T], less func(i, j T) bool) Stream[T] {
	return &joinSorted[T]{
		left:  left,
		right: right,
		less:  less,
	}
}

type dedupSorted[T any] struct {
	inner  Stream[T]
	equals func(i, j T) bool
	item   T
	first  bool
}

func (stream *dedupSorted[T]) Next() bool {
	for {
		if !stream.inner.Next() {
			return false
		}
		next := stream.inner.Item()
		if !stream.first && stream.equals(stream.item, next) {
			continue
		}
		stream.item = next
		stream.first = false
		return true
	}
}

func (stream *dedupSorted[T]) Item() T {
	return stream.item
}

func (stream *dedupSorted[T]) Done() error {
	return stream.inner.Done()
}

// DedupSorted deduplicates the items within a sorted stream.
func DedupSorted[T any](stream Stream[T], equals func(i, j T) bool) Stream[T] {
	return &dedupSorted[T]{
		inner:  stream,
		equals: equals,
		first:  true,
	}
}
