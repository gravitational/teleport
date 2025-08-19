/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package stream

import (
	"errors"
	"io"
	"iter"

	"github.com/gravitational/trace"
)

// Stream is an alias for a fallible iterator. The Stream type alias and related
// utilities follow a specific model that may not be appropriate for all types
// that match the underlying signature. In particular, streams are modeled as the iterator
// equivalent of a function of the form `func() ([]T, error)`. This means that streams
// are expected to yield at most one error, and to not yield any additional values after
// an error has been produced. The combinators in this package short-circuit on the
// first error encountered, including those that handle multiple substreams.
type Stream[T any] = iter.Seq2[T, error]

// Func builds a stream from a closure. The supplied closure *must*
// return io.EOF if no more items are available. Failure to return io.EOF
// (or some other error) may cause infinite loops. If wrapping a
// paginated API, consider using PageFunc instead.
func Func[T any](fn func() (T, error)) Stream[T] {
	return func(yield func(T, error) bool) {
		for {
			item, err := fn()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					yield(*new(T), trace.Wrap(err))
				}
				return
			}

			if !yield(item, nil) {
				return
			}
		}
	}
}

// Collect aggregates a stream into a slice. If an error is hit, the
// items observed thus far are still returned, but they may not represent
// the complete set.
func Collect[T any](stream Stream[T]) ([]T, error) {
	var c []T
	for item, err := range stream {
		if err != nil {
			return c, trace.Wrap(err)
		}
		c = append(c, item)
	}
	return c, nil
}

// CollectPages aggregates a paginated stream into a slice. If an error
// is hit, the pages observed thus far are still returned, but they may not
// represent the complete set.
func CollectPages[T any, S ~[]T](stream Stream[S]) ([]T, error) {
	var c []T
	for page, err := range stream {
		if err != nil {
			return c, trace.Wrap(err)
		}
		c = append(c, page...)
	}
	return c, nil
}

// FilterMap maps a stream of type A into a stream of type B, filtering out
// items when fn returns false.
func FilterMap[A, B any](stream Stream[A], fn func(A) (B, bool)) Stream[B] {
	return func(yield func(B, error) bool) {
		for item, err := range stream {
			if err != nil {
				yield(*new(B), trace.Wrap(err))
				return
			}

			mapped, ok := fn(item)
			if !ok {
				continue
			}

			if !yield(mapped, nil) {
				return
			}
		}
	}
}

// Chain joins multiple streams in order, fully consuming one before moving to the next.
func Chain[T any](streams ...Stream[T]) Stream[T] {
	return func(yield func(T, error) bool) {
		for _, stream := range streams {
			for item, err := range stream {
				if err != nil {
					yield(*new(T), trace.Wrap(err))
					return
				}

				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// Chunks breaks a stream into chunks of a fixed size. The last chunk may be smaller
// than the specified size. Zero/negative values of size result in an empty stream.
func Chunks[T any](stream Stream[T], size int) Stream[[]T] {
	if size < 1 {
		return Empty[[]T]()
	}
	return func(yield func([]T, error) bool) {
		var chunk []T
		for item, err := range stream {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			if chunk == nil {
				chunk = make([]T, 0, size)
			}

			chunk = append(chunk, item)
			if len(chunk) == size {
				if !yield(chunk, nil) {
					return
				}
				chunk = nil
			}
		}

		if len(chunk) > 0 {
			if !yield(chunk, nil) {
				return
			}
		}
	}
}

// Fail creates an empty stream that fails immediately with the supplied error.
func Fail[T any](err error) Stream[T] {
	if err != nil {
		return func(yield func(T, error) bool) {
			yield(*new(T), trace.Wrap(err))
		}
	}

	return Empty[T]()
}

// Empty creates an empty stream.
func Empty[T any]() Stream[T] {
	return func(yield func(T, error) bool) {}
}

// Once creates a stream that yields a single item.
func Once[T any](item T) Stream[T] {
	return func(yield func(T, error) bool) {
		yield(item, nil)
	}
}

// OnceFunc builds a stream from a closure that will yield exactly zero or one items. This stream
// is the lazy equivalent of the Once/Fail/Empty combinators. A nil error value results
// in a single-element stream. An error value of io.EOF results in an empty stream. All other error
// values result in a failing stream.
func OnceFunc[T any](fn func() (T, error)) Stream[T] {
	return func(yield func(T, error) bool) {
		item, err := fn()
		if errors.Is(err, io.EOF) {
			return
		}
		yield(item, err)
	}
}

// Drain consumes a stream to completion, reporting its error if any.
func Drain[T any](stream Stream[T]) error {
	for _, err := range stream {
		if err != nil {
			return err
		}
	}

	return nil
}

// Slice constructs a stream from a slice.
func Slice[T any, S ~[]T](items S) Stream[T] {
	return func(yield func(T, error) bool) {
		for _, item := range items {
			if !yield(item, nil) {
				return
			}
		}
	}
}

// PageFunc is equivalent to Func except that it performs internal depagination. As with
// Func, the supplied closure *must* return io.EOF if no more items are available. Failure
// to return io.EOF (or some other error) may result in infinite loops.
func PageFunc[T any](fn func() ([]T, error)) Stream[T] {
	return func(yield func(T, error) bool) {
		for {
			items, err := fn()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					yield(*new(T), trace.Wrap(err))
				}
				return
			}

			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// Skip skips the first n items from a stream. Zero/negative values of n
// have no effect.
func Skip[T any](stream Stream[T], n int) Stream[T] {
	return func(yield func(T, error) bool) {
		n := n // copy
		for item, err := range stream {
			if err != nil {
				yield(*new(T), trace.Wrap(err))
				return
			}

			if n > 0 {
				n--
				continue
			}

			if !yield(item, nil) {
				return
			}
		}
	}
}

// Flatten flattens a stream of streams into a single stream of items.
func Flatten[T any](stream Stream[Stream[T]]) Stream[T] {
	return func(yield func(T, error) bool) {
		for inner, err := range stream {
			if err != nil {
				yield(*new(T), trace.Wrap(err))
				return
			}

			for item, err := range inner {
				if err != nil {
					yield(*new(T), trace.Wrap(err))
					return
				}

				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// MapErr maps over the "terminating" error value of a stream. This is a distinctly different
// concept than mapping over the second value of an iter.Seq2. The mapping function is called with
// a nil value when the stream terminates successfully, and nil values returned by the function are
// not yielded. The effect of this behavior is that MapErr can be used to suppress, inject, *or* modify
// the terminating error of a stream.
func MapErr[T any](stream Stream[T], fn func(error) error) Stream[T] {
	return func(yield func(T, error) bool) {
		for item, err := range stream {
			if err != nil {
				mappedErr := fn(err)
				if mappedErr == nil {
					// terminating error was suppressed
					return
				}
				yield(*new(T), mappedErr)
				return
			}

			if !yield(item, nil) {
				return
			}
		}

		if err := fn(nil); err != nil {
			yield(*new(T), trace.Wrap(err))
		}
	}
}

// RateLimit applies a rate-limiting function to a stream before each attempt to get the
// next item. If the wait function returns a non-nil error, the stream is halted. The wait
// function may return io.EOF to indicate a graceful/expected halting condition.
func RateLimit[T any](stream Stream[T], wait func() error) Stream[T] {
	return func(yield func(T, error) bool) {
		for item, err := range stream {
			if err != nil {
				yield(*new(T), trace.Wrap(err))
				return
			}

			if !yield(item, nil) {
				return
			}

			if err := wait(); err != nil {
				if !errors.Is(err, io.EOF) {
					yield(*new(T), trace.Wrap(err))
				}
				return
			}
		}
	}
}

// MergeStreams merges two sorted streams and returns a single stream which uses the provided less function to determine which item to yield first in order to preserve the sort order.
func MergeStreams[T any](
	streamA Stream[T],
	streamB Stream[T],
	less func(a, b T) bool,
) Stream[T] {
	return func(yield func(T, error) bool) {
		var itemA, itemB T
		var okA, okB bool
		var err error

		nextA, stopA := iter.Pull2(streamA)
		nextB, stopB := iter.Pull2(streamB)
		defer stopA()
		defer stopB()

		itemA, err, okA = nextA()
		if err != nil {
			yield(*new(T), trace.Wrap(err))
			return
		}

		itemB, err, okB = nextB()

		for {
			if err != nil {
				yield(*new(T), trace.Wrap(err))
				return
			}
			switch {
			case !okA && !okB:
				return
			case !okA:
				if !yield(itemB, nil) {
					return
				}

				itemB, err, okB = nextB()
			case !okB:
				if !yield(itemA, nil) {
					return
				}

				itemA, err, okA = nextA()
			default:
				if less(itemA, itemB) {
					if !yield(itemA, nil) {
						return
					}

					itemA, err, okA = nextA()
				} else {
					if !yield(itemB, nil) {
						return
					}

					itemB, err, okB = nextB()
				}
			}
		}
	}
}

// TakeWhile iterates the stream taking items while predicate returns true
func TakeWhile[T any](stream Stream[T], predicate func(T) bool) Stream[T] {
	return func(yield func(T, error) bool) {
		for item, err := range stream {
			if err != nil {
				yield(*new(T), trace.Wrap(err))
				return
			}

			if !predicate(item) {
				return
			}

			if !yield(item, nil) {
				return
			}

		}
	}
}
