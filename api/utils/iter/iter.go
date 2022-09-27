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

import (
	"github.com/gravitational/trace"
)

// Iter is a generic interface for iterators. Built with the intention of
// abstracting over streaming apis. Iterators may panic if misused. See the
// Collect function for an example of the correct consumption pattern.
//
// NOTE: iterators are almost always perform worse than slices in go. unless you're dealing
// with a resource that scales linearly with cluster size, you probably don't need
// this.
type Iter[T any] interface {
	// Next attempts to advance the iterator to the next item. If false is returned,
	// then no more items are available. Next() and Item() must not be called after the
	// first time Next() returns false.
	Next() bool
	// Item gets the current item. Invoking Item() is only safe if Next was previously
	// invoked *and* returned true. Invoking Item() before invoking Next(), or after Next()
	// returned false may cause panics or other unpredictable behavior. Whether or not the
	// item returned is safe for access after the iterator is advanced again is dependent
	// on the implementation and should be documented (e.g. an I/O based iterator might
	// re-use an underlying buffer).
	Item() T
	// Done() checks for any errors that occurred during iteration and informs the iterator
	// that we've finished consuming items from it. Invoking Next() or Item() after Done()
	// has been called is not permitted. Done may trigger cleanup operations, but unlike Close()
	// the error reported is specifically related to failures that occurred *during* iteration,
	// meaning that if Done() returns an error, there is a high liklihood that the complete
	// set of values was not observed. For this reason, Done() should always be checked explicitly
	// rather than deferred as Close() might be.
	Done() error
}

// iterFn is a wrapper that converts a closure into an iterator.
type iterFn[T any] struct {
	fn      func() (T, error)
	doneFns []func()
	item    T
	err     error
}

func (iter *iterFn[T]) Next() bool {
	iter.item, iter.err = iter.fn()
	return iter.err == nil
}

func (iter *iterFn[T]) Item() T {
	return iter.item
}

func (iter *iterFn[T]) Done() error {
	for _, fn := range iter.doneFns {
		fn()
	}
	if trace.IsEOF(iter.err) {
		return nil
	}
	return iter.err
}

// IterFn builds an iterator from a closure. The supplied closure *must*
// return io.EOF if no more items are available. Failure to return io.EOF
// (or some other error) may cause infinite loops. Cleanup functions may
// be optionally provided which will be run on close. If wrapping a
// paginated API, consider using PageFn instead.
func IterFn[T any](fn func() (T, error), doneFns ...func()) Iter[T] {
	return &iterFn[T]{
		fn:      fn,
		doneFns: doneFns,
	}
}

// Collect aggregates an iterator into a slice. If an error is hit, the
// items observed thus far are still returned, but they may not represent
// the compelte set.
func Collect[T any](iter Iter[T]) ([]T, error) {
	var c []T
	for iter.Next() {
		c = append(c, iter.Item())
	}
	return c, trace.Wrap(iter.Done())
}

// CollectPages aggregates a paginated iterator into a slice. If an error
// is hit, the pages observed thus far are still returned, but they may not
// represent the complete set.
func CollectPages[T any](iter Iter[[]T]) ([]T, error) {
	var c []T
	for iter.Next() {
		c = append(c, iter.Item()...)
	}
	return c, trace.Wrap(iter.Done())
}

// filterMap is an iterator that performs a FilterMap operation.
type filterMap[A, B any] struct {
	inner Iter[A]
	fn    func(A) (B, bool)
	item  B
	err   error
}

func (iter *filterMap[A, B]) Next() bool {
	for {
		if !iter.inner.Next() {
			return false
		}
		var ok bool
		iter.item, ok = iter.fn(iter.inner.Item())
		if !ok {
			continue
		}
		return true
	}
}

func (iter *filterMap[A, B]) Item() B {
	return iter.item
}

func (iter *filterMap[A, B]) Done() error {
	return iter.inner.Done()
}

// FilterMap maps an iterator of type A into an iterator of type B, filtering out
// items when fn returns false.
func FilterMap[A, B any](iter Iter[A], fn func(A) (B, bool)) Iter[B] {
	return &filterMap[A, B]{
		inner: iter,
		fn:    fn,
	}
}

// mapWhile is an iterator that performs a MapWhile operation.
type mapWhile[A, B any] struct {
	inner Iter[A]
	fn    func(A) (B, bool)
	item  B
	err   error
}

func (iter *mapWhile[A, B]) Next() bool {
	if !iter.inner.Next() {
		return false
	}

	var ok bool
	iter.item, ok = iter.fn(iter.inner.Item())
	return ok
}

func (iter *mapWhile[A, B]) Item() B {
	return iter.item
}

func (iter *mapWhile[A, B]) Done() error {
	return iter.inner.Done()
}

// MapWhile maps an iterator of type A into an iterator of type B, halting early
// if fn returns false.
func MapWhile[A, B any](iter Iter[A], fn func(A) (B, bool)) Iter[B] {
	return &mapWhile[A, B]{
		inner: iter,
		fn:    fn,
	}
}

// empty is an iterator that halts immediately
type empty[T any] struct {
	err error
}

func (iter empty[T]) Next() bool {
	return false
}

func (iter empty[T]) Item() T {
	panic("Item() called on empty/failed iterator")
}

func (iter empty[T]) Done() error {
	return iter.err
}

// Fail creates an empty iterator that fails immediately with the supplied error.
func Fail[T any](err error) Iter[T] {
	return empty[T]{err}
}

// Empty creates an empty iterator (equivalent to Fail(nil)).
func Empty[T any]() Iter[T] {
	return empty[T]{}
}

// once is an iterator that yields a single item
type once[T any] struct {
	yielded bool
	item    T
}

func (iter *once[T]) Next() bool {
	if iter.yielded {
		return false
	}
	iter.yielded = true
	return true
}

func (iter *once[T]) Item() T {
	return iter.item
}

func (iter *once[T]) Done() error {
	return nil
}

// Once creates an iterator that yields a single a single item.
func Once[T any](item T) Iter[T] {
	return &once[T]{
		item: item,
	}
}

// Drain consumes an iterator to completion.
func Drain[T any](iter Iter[T]) error {
	for iter.Next() {
	}
	return trace.Wrap(iter.Done())
}

// slice iterates over a slice
type slice[T any] struct {
	items []T
	idx   int
}

func (s *slice[T]) Next() bool {
	s.idx++
	return len(s.items) > s.idx
}

func (s *slice[T]) Item() T {
	return s.items[s.idx]
}

func (s *slice[T]) Done() error {
	return nil
}

// Slice constructs an iterator from a slice.
func Slice[T any](items []T) Iter[T] {
	return &slice[T]{
		items: items,
		idx:   -1,
	}
}

type pageFn[T any] struct {
	inner iterFn[[]T]
	page  slice[T]
	err   error
}

func (d *pageFn[T]) Next() bool {
	for {
		if d.page.Next() {
			return true
		}
		if !d.inner.Next() {
			return false
		}
		d.page = slice[T]{
			items: d.inner.Item(),
			idx:   -1,
		}
	}
}

func (d *pageFn[T]) Item() T {
	return d.page.Item()
}

func (d *pageFn[T]) Done() error {
	return d.inner.Done()
}

// PageFn is equivalent to IterFn except that it performs internal depagination. As with
// IterFn, the supplied closure *must* return io.EOF if no more items are available. Failure
// to return io.EOF (or some other error) may result in infinite loops.
func PageFn[T any](fn func() ([]T, error), doneFns ...func()) Iter[T] {
	return &pageFn[T]{
		inner: iterFn[[]T]{
			fn:      fn,
			doneFns: doneFns,
		},
	}
}
