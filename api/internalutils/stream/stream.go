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

import (
	"errors"
	"io"

	"github.com/gravitational/trace"
)

// Stream is a generic interface for streaming APIs. This package was built with the
// intention of making it easier to write streaming resource getters, and may not be
// be suitable for applications outside of that specific usecase. Streams may panic if
// misused. See the Collect function for an example of the correct consumption pattern.
//
// NOTE: streams almost always perform worse than slices in go. unless you're dealing
// with a resource that scales linearly with cluster size, you are probably better off
// just working with slices.
type Stream[T any] interface {
	// Next attempts to advance the stream to the next item. If false is returned,
	// then no more items are available. Next() and Item() must not be called after the
	// first time Next() returns false.
	Next() bool
	// Item gets the current item. Invoking Item() is only safe if Next was previously
	// invoked *and* returned true. Invoking Item() before invoking Next(), or after Next()
	// returned false may cause panics or other unpredictable behavior. Whether or not the
	// item returned is safe for access after the stream is advanced again is dependent
	// on the implementation and should be documented (e.g. an I/O based stream might
	// re-use an underlying buffer).
	Item() T
	// Done checks for any errors that occurred during streaming and informs the stream
	// that we've finished consuming items from it. Invoking Next() or Item() after Done()
	// has been called is not permitted. Done may trigger cleanup operations, but unlike Close()
	// the error reported is specifically related to failures that occurred *during* streaming,
	// meaning that if Done() returns an error, there is a high likelihood that the complete
	// set of values was not observed. For this reason, Done() should always be checked explicitly
	// rather than deferred as Close() might be.
	Done() error
}

// streamFunc is a wrapper that converts a closure into a stream.
type streamFunc[T any] struct {
	fn        func() (T, error)
	doneFuncs []func()
	item      T
	err       error
}

func (stream *streamFunc[T]) Next() bool {
	stream.item, stream.err = stream.fn()
	return stream.err == nil
}

func (stream *streamFunc[T]) Item() T {
	return stream.item
}

func (stream *streamFunc[T]) Done() error {
	for _, fn := range stream.doneFuncs {
		fn()
	}
	if errors.Is(stream.err, io.EOF) {
		return nil
	}
	return stream.err
}

// Func builds a stream from a closure. The supplied closure *must*
// return io.EOF if no more items are available. Failure to return io.EOF
// (or some other error) may cause infinite loops. Cleanup functions may
// be optionally provided which will be run on close. If wrapping a
// paginated API, consider using PageFunc instead.
func Func[T any](fn func() (T, error), doneFuncs ...func()) Stream[T] {
	return &streamFunc[T]{
		fn:        fn,
		doneFuncs: doneFuncs,
	}
}

// Collect aggregates a stream into a slice. If an error is hit, the
// items observed thus far are still returned, but they may not represent
// the complete set.
func Collect[T any](stream Stream[T]) ([]T, error) {
	var c []T
	for stream.Next() {
		c = append(c, stream.Item())
	}
	return c, trace.Wrap(stream.Done())
}

// CollectPages aggregates a paginated stream into a slice. If an error
// is hit, the pages observed thus far are still returned, but they may not
// represent the complete set.
func CollectPages[T any](stream Stream[[]T]) ([]T, error) {
	var c []T
	for stream.Next() {
		c = append(c, stream.Item()...)
	}
	return c, trace.Wrap(stream.Done())
}

// filterMap is a stream that performs a FilterMap operation.
type filterMap[A, B any] struct {
	inner Stream[A]
	fn    func(A) (B, bool)
	item  B
}

func (stream *filterMap[A, B]) Next() bool {
	for {
		if !stream.inner.Next() {
			return false
		}
		var ok bool
		stream.item, ok = stream.fn(stream.inner.Item())
		if !ok {
			continue
		}
		return true
	}
}

func (stream *filterMap[A, B]) Item() B {
	return stream.item
}

func (stream *filterMap[A, B]) Done() error {
	return stream.inner.Done()
}

// FilterMap maps a stream of type A into a stream of type B, filtering out
// items when fn returns false.
func FilterMap[A, B any](stream Stream[A], fn func(A) (B, bool)) Stream[B] {
	return &filterMap[A, B]{
		inner: stream,
		fn:    fn,
	}
}

// mapWhile is a stream that performs a MapWhile operation.
type mapWhile[A, B any] struct {
	inner Stream[A]
	fn    func(A) (B, bool)
	item  B
}

func (stream *mapWhile[A, B]) Next() bool {
	if !stream.inner.Next() {
		return false
	}

	var ok bool
	stream.item, ok = stream.fn(stream.inner.Item())
	return ok
}

func (stream *mapWhile[A, B]) Item() B {
	return stream.item
}

func (stream *mapWhile[A, B]) Done() error {
	return stream.inner.Done()
}

// MapWhile maps a stream of type A into a stream of type B, halting early
// if fn returns false.
func MapWhile[A, B any](stream Stream[A], fn func(A) (B, bool)) Stream[B] {
	return &mapWhile[A, B]{
		inner: stream,
		fn:    fn,
	}
}

// chain is a stream that performs a Chain operation.
type chain[T any] struct {
	streams []Stream[T]
	err     error
}

func (stream *chain[T]) Next() bool {
	for len(stream.streams) != 0 && stream.err == nil {
		if stream.streams[0].Next() {
			return true
		}

		stream.err = stream.streams[0].Done()
		stream.streams[0] = nil
		stream.streams = stream.streams[1:]
	}

	return false
}

func (stream *chain[T]) Item() T {
	return stream.streams[0].Item()
}

func (stream *chain[T]) Done() error {
	for _, s := range stream.streams {
		s.Done()
	}
	stream.streams = nil

	return stream.err
}

// Chain joins multiple streams in order, fully consuming one before moving to the next.
func Chain[T any](streams ...Stream[T]) Stream[T] {
	return &chain[T]{
		streams: streams,
	}
}

// empty is a stream that halts immediately
type empty[T any] struct {
	err error
}

func (stream empty[T]) Next() bool {
	return false
}

func (stream empty[T]) Item() T {
	panic("Item() called on empty/failed stream")
}

func (stream empty[T]) Done() error {
	return stream.err
}

// Fail creates an empty stream that fails immediately with the supplied error.
func Fail[T any](err error) Stream[T] {
	return empty[T]{err}
}

// Empty creates an empty stream (equivalent to Fail(nil)).
func Empty[T any]() Stream[T] {
	return empty[T]{}
}

// once is a stream that yields a single item
type once[T any] struct {
	yielded bool
	item    T
}

func (stream *once[T]) Next() bool {
	if stream.yielded {
		return false
	}
	stream.yielded = true
	return true
}

func (stream *once[T]) Item() T {
	return stream.item
}

func (stream *once[T]) Done() error {
	return nil
}

// Once creates a stream that yields a single item.
func Once[T any](item T) Stream[T] {
	return &once[T]{
		item: item,
	}
}

// onceFunc is a stream that produces zero or one items based on
// a lazily evaluated closure.
type onceFunc[T any] struct {
	fn   func() (T, error)
	item T
	err  error
}

func (stream *onceFunc[T]) Next() bool {
	if stream.fn == nil {
		return false
	}

	stream.item, stream.err = stream.fn()
	stream.fn = nil
	return stream.err == nil
}

func (stream *onceFunc[T]) Item() T {
	return stream.item
}

func (stream *onceFunc[T]) Done() error {
	if errors.Is(stream.err, io.EOF) {
		return nil
	}
	return stream.err
}

// OnceFunc builds a stream from a closure that will yield exactly zero or one items. This stream
// is the lazy equivalent of the Once/Fail/Empty combinators. A nil error value results
// in a single-element stream. An error value of io.EOF results in an empty stream. All other error
// values result in a failing stream.
func OnceFunc[T any](fn func() (T, error)) Stream[T] {
	return &onceFunc[T]{
		fn: fn,
	}
}

// Drain consumes a stream to completion.
func Drain[T any](stream Stream[T]) error {
	for stream.Next() {
	}
	return trace.Wrap(stream.Done())
}

// slice streams the elements of a slice
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

// Slice constructs a stream from a slice.
func Slice[T any](items []T) Stream[T] {
	return &slice[T]{
		items: items,
		idx:   -1,
	}
}

type pageFunc[T any] struct {
	inner streamFunc[[]T]
	page  slice[T]
}

func (d *pageFunc[T]) Next() bool {
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

func (d *pageFunc[T]) Item() T {
	return d.page.Item()
}

func (d *pageFunc[T]) Done() error {
	return d.inner.Done()
}

// PageFunc is equivalent to Func except that it performs internal depagination. As with
// Func, the supplied closure *must* return io.EOF if no more items are available. Failure
// to return io.EOF (or some other error) may result in infinite loops.
func PageFunc[T any](fn func() ([]T, error), doneFuncs ...func()) Stream[T] {
	return &pageFunc[T]{
		inner: streamFunc[[]T]{
			fn:        fn,
			doneFuncs: doneFuncs,
		},
	}
}

// Take takes the next n items from a stream. It returns a slice of the items
// and the result of the last call to stream.Next().
func Take[T any](stream Stream[T], n int) ([]T, bool) {
	items := make([]T, 0, n)
	for i := 0; i < n; i++ {
		if !stream.Next() {
			return items, false
		}
		items = append(items, stream.Item())
	}
	return items, true
}

type skip[T any] struct {
	inner Stream[T]
	skip  int
}

func (s *skip[T]) Next() bool {
	for i := 0; i < s.skip; i++ {
		if !s.inner.Next() {
			return false
		}
	}
	s.skip = 0
	return s.inner.Next()
}

func (s *skip[T]) Item() T {
	return s.inner.Item()
}

func (s *skip[T]) Done() error {
	return s.inner.Done()
}

// Skip skips the first n items from a stream. Zero/negative values of n
// have no effect.
func Skip[T any](stream Stream[T], n int) Stream[T] {
	return &skip[T]{
		inner: stream,
		skip:  n,
	}
}

type flatten[T any] struct {
	inner   Stream[Stream[T]]
	current Stream[T]
	err     error
}

func (stream *flatten[T]) Next() bool {
	for {
		if stream.current != nil {
			if stream.current.Next() {
				return true
			}
			stream.err = stream.current.Done()
			stream.current = nil
			if stream.err != nil {
				return false
			}
		}

		if !stream.inner.Next() {
			return false
		}

		stream.current = stream.inner.Item()
	}
}

func (stream *flatten[T]) Item() T {
	return stream.current.Item()
}

func (stream *flatten[T]) Done() error {
	if stream.current != nil {
		stream.err = stream.current.Done()
		stream.current = nil
	}

	ierr := stream.inner.Done()
	if stream.err != nil {
		return stream.err
	}
	return ierr
}

// Flatten flattens a stream of streams into a single stream of items.
func Flatten[T any](stream Stream[Stream[T]]) Stream[T] {
	return &flatten[T]{
		inner: stream,
	}
}

type mapErr[T any] struct {
	inner Stream[T]
	fn    func(error) error
}

func (stream *mapErr[T]) Next() bool {
	return stream.inner.Next()
}

func (stream *mapErr[T]) Item() T {
	return stream.inner.Item()
}

func (stream *mapErr[T]) Done() error {
	return stream.fn(stream.inner.Done())
}

// MapErr maps over the error returned by Done(). The supplied function is called
// for all invocations of Done(), meaning that it can change, suppress, or create
// errors as needed.
func MapErr[T any](stream Stream[T], fn func(error) error) Stream[T] {
	return &mapErr[T]{
		inner: stream,
		fn:    fn,
	}
}

type rateLimit[T any] struct {
	inner   Stream[T]
	wait    func() error
	waitErr error
}

func (stream *rateLimit[T]) Next() bool {
	stream.waitErr = stream.wait()
	if stream.waitErr != nil {
		return false
	}

	return stream.inner.Next()
}

func (stream *rateLimit[T]) Item() T {
	return stream.inner.Item()
}

func (stream *rateLimit[T]) Done() error {
	if err := stream.inner.Done(); err != nil {
		return err
	}

	if errors.Is(stream.waitErr, io.EOF) {
		return nil
	}

	return stream.waitErr
}

// RateLimit applies a rate-limiting function to a stream s.t. calls to Next() block on
// the supplied function before calling the inner stream. If the function returns an
// error, the inner stream is not polled and Next() returns false. The wait function may
// return io.EOF to indicate a graceful/expected halting condition. Any other error value
// is treated as unexpected and will be bubbled up via Done() unless an error from the
// inner stream takes precedence.
func RateLimit[T any](stream Stream[T], wait func() error) Stream[T] {
	return &rateLimit[T]{
		inner: stream,
		wait:  wait,
	}
}

// mergedStream is an adapter for merging two streams based on a less function, it will get the next items from both streams and yield the result of streamA if the comparison function returns true,
// or the result of streamB if false.
// The streams can be of different types, however, the resulting merged stream must be of one type.
type mergedStream[T, U, V any] struct {
	streamA Stream[T]
	streamB Stream[U]
	itemA   T
	itemB   U
	// hasItemA keeps track of whether we have an item available from streamA.
	// We use this flag instead of just checking whether itemA is nil in order to ensure that this adapter also works with non-nullable types.
	hasItemA bool
	// hasItemB keeps track of whether we have an item available from streamB.
	hasItemB bool
	// less is the comparison function used to compare the items from both lists. If true, the merged stream will yield the item from streamA, if false, it will yield the item from streamB.
	less func(a T, b U) bool
	// convert is the function that is used to convert an item from the streamB type into the V type which is the type of the merged stream.
	// If the streams are of the same type, this can simply be a function that returns the item as-is.
	// convertA converts an item from streamA into the V type.
	convertA func(item T) V
	// convertB converts an item from streamB into the V type.
	convertB func(item U) V
}

// Next attempts to advance each stream to the next item. If false is returned, then no more items are available.
func (ms *mergedStream[T, U, V]) Next() bool {
	// Attempt to advance to the next item in streamA, if we don't already have an item from it.
	if !ms.hasItemA && ms.streamA.Next() {
		ms.itemA = ms.streamA.Item()
		ms.hasItemA = true
	}
	// Attempt to advance to the next item in streamB, if we don't already  have an item from it.
	if !ms.hasItemB && ms.streamB.Next() {
		ms.itemB = ms.streamB.Item()
		ms.hasItemB = true
	}

	// Return true if either stream has an item available.
	return ms.hasItemA || ms.hasItemB
}

// Item yields the current item.
// If only an item from one stream is available, then it will yield that item.
// If the items from both streams are available, then the less function will be used to decide which item to yield.
// After yielding an item, it will also reset the availability flag of the yielded item's stream so that subsequent calls know to fetch a new item.
func (ms *mergedStream[T, U, V]) Item() V {
	// If both streams have items available, use the less function to determine which to yield.
	if ms.hasItemA && ms.hasItemB {
		if ms.less(ms.itemA, ms.itemB) {
			// Reset the hasItemA flag since it has been consumed.
			ms.hasItemA = false
			return ms.convertA(ms.itemA)
		}
		// Reset the hasItemB flag since it has been consumed.
		ms.hasItemB = false
		return ms.convertB(ms.itemB)

	}

	// If only streamA has an item available, yield it.
	if ms.hasItemA {
		ms.hasItemA = false
		return ms.convertA(ms.itemA)
	}

	// If only streamB has an item available, yield it.
	if ms.hasItemB {
		ms.hasItemB = false
		return ms.convertB(ms.itemB)
	}

	// If neither stream has an available item, then this function should not have been called at all and there is a logic error.
	panic("Item() was called but neither stream has an item, this is a bug")
}

// Done closes both streams.
func (ms *mergedStream[T, U, V]) Done() error {
	errA := ms.streamA.Done()
	errB := ms.streamB.Done()

	if errA != nil && errB != nil {
		return trace.NewAggregate(errA, errB)
	}
	if errA != nil {
		return trace.Wrap(errA, "failed to close the first stream")
	}
	if errB != nil {
		return trace.Wrap(errB, "failed to close the second stream")
	}

	return nil
}

// MergeStreams merges two streams and returns a single stream which uses the provided less function to determine which item to yield.
func MergeStreams[T any, U, V any](
	streamA Stream[T],
	streamB Stream[U],
	less func(a T, b U) bool,
	convertA func(item T) V,
	convertB func(item U) V,
) Stream[V] {
	return &mergedStream[T, U, V]{
		streamA:  streamA,
		streamB:  streamB,
		less:     less,
		convertA: convertA,
		convertB: convertB,
	}
}
