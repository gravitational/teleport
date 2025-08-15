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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSlice tests the slice stream.
func TestSlice(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(Slice([]int{1, 2, 3}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// single-element slice
	s, err = Collect(Slice([]int{100}))
	require.NoError(t, err)
	require.Equal(t, []int{100}, s)

	// nil slice
	s, err = Collect(Slice([]int(nil)))
	require.NoError(t, err)
	require.Empty(t, s)
}

// TestFilterMap tests the FilterMap combinator.
func TestFilterMap(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(FilterMap(Slice([]int{1, 2, 3, 4}), func(i int) (string, bool) {
		if i%2 == 0 {
			return fmt.Sprintf("%d", i*10), true
		}
		return "", false
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"20", "40"}, s)

	// single-match
	s, err = Collect(FilterMap(Slice([]int{1, 2, 3, 4}), func(i int) (string, bool) {
		if i == 3 {
			return "three", true
		}
		return "", false
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"three"}, s)

	// no matches
	s, err = Collect(FilterMap(Slice([]int{1, 2, 3, 4}), func(i int) (string, bool) {
		return "", false
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// empty stream
	s, err = Collect(FilterMap(Empty[int](), func(_ int) (string, bool) { panic("unreachable") }))
	require.NoError(t, err)
	require.Empty(t, s)

	// failure
	err = Drain(FilterMap(Fail[int](fmt.Errorf("unexpected error")), func(_ int) (string, bool) { panic("unreachable") }))
	require.Error(t, err)
}

// TestChain tests the Chain combinator.
func TestChain(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(Chain(
		Slice([]int{1, 2, 3}),
		Slice([]int{4}),
		Slice([]int{5, 6}),
	))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, s)

	// single substream
	s, err = Collect(Chain(Slice([]int{1, 2, 3})))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// no substreams
	s, err = Collect(Chain[int]())
	require.NoError(t, err)
	require.Empty(t, s)

	// some empty substreams
	s, err = Collect(Chain(
		Empty[int](),
		Slice([]int{4, 5, 6}),
		Empty[int](),
	))
	require.NoError(t, err)
	require.Equal(t, []int{4, 5, 6}, s)

	// all empty substreams
	s, err = Collect(Chain(
		Empty[int](),
		Empty[int](),
	))
	require.NoError(t, err)
	require.Empty(t, s)

	// late failure
	s, err = Collect(Chain(
		Slice([]int{7, 7, 7}),
		Fail[int](fmt.Errorf("some error")),
	))
	require.Error(t, err)
	require.Equal(t, []int{7, 7, 7}, s)

	// early failure
	s, err = Collect(Chain(
		Fail[int](fmt.Errorf("some other error")),
		Func(func() (int, error) { panic("unreachable") }),
	))
	require.Error(t, err)
	require.Empty(t, s)
}

// TestChunks tests the Chunks combinator.
func TestChunks(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 2))
	require.NoError(t, err)
	require.Equal(t, [][]int{{1, 2}, {3, 4}, {5, 6}}, s)

	// single-element chunks
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 1))
	require.NoError(t, err)
	require.Equal(t, [][]int{{1}, {2}, {3}, {4}, {5}, {6}}, s)

	// empty stream
	s, err = Collect(Chunks(Empty[int](), 2))
	require.NoError(t, err)
	require.Empty(t, s)

	// zero chunk size
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 0))
	require.NoError(t, err)
	require.Equal(t, ([][]int)(nil), s)

	// negative chunk size
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), -1))
	require.NoError(t, err)
	require.Equal(t, ([][]int)(nil), s)

	// chunk size larger than stream
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 10))
	require.NoError(t, err)
	require.Equal(t, [][]int{{1, 2, 3, 4, 5, 6}}, s)

	// chunk size equal to stream size
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 6))
	require.NoError(t, err)
	require.Equal(t, [][]int{{1, 2, 3, 4, 5, 6}}, s)

	// chunk size with remainder
	s, err = Collect(Chunks(Slice([]int{1, 2, 3, 4, 5, 6}), 4))
	require.NoError(t, err)
	require.Equal(t, [][]int{{1, 2, 3, 4}, {5, 6}}, s)

	// chunk with immediate failure
	err = Drain(Chunks(Fail[int](fmt.Errorf("unexpected error")), 2))
	require.Error(t, err)

	// chunk with failure after a few iterations
	err = Drain(Chunks(Chain(
		Slice([]int{1, 2, 3}),
		Fail[int](fmt.Errorf("unexpected error")),
		Slice([]int{4, 5, 6}),
	), 2))
	require.Error(t, err)
}

// TestFunc tests the Func stream.
func TestFunc(t *testing.T) {
	t.Parallel()

	// normal usage
	var n int
	s, err := Collect(Func(func() (int, error) {
		n++
		if n > 3 {
			return 0, io.EOF
		}
		return n, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// single-element
	var once bool
	s, err = Collect(Func(func() (int, error) {
		if once {
			return 0, io.EOF
		}
		once = true
		return 100, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{100}, s)

	// no element
	s, err = Collect(Func(func() (int, error) {
		return 0, io.EOF
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// immediate error
	err = Drain(Func(func() (int, error) {
		return 0, fmt.Errorf("unexpected error")
	}))
	require.Error(t, err)

	// error after a few streamations
	n = 0
	err = Drain(Func(func() (int, error) {
		n++
		if n > 10 {
			return 0, fmt.Errorf("unexpected error")
		}
		return n, nil
	}))
	require.Error(t, err)
}

func TestPageFunc(t *testing.T) {
	t.Parallel()

	// basic pages
	var n int
	s, err := Collect(PageFunc(func() ([]int, error) {
		n++
		if n > 3 {
			return nil, io.EOF
		}
		return []int{
			n,
			n * 10,
			n * 100,
		}, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 10, 100, 2, 20, 200, 3, 30, 300}, s)

	// single page
	var once bool
	s, err = Collect(PageFunc(func() ([]int, error) {
		if once {
			return nil, io.EOF
		}
		once = true
		return []int{1, 2, 3}, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// single element
	once = false
	s, err = Collect(PageFunc(func() ([]int, error) {
		if once {
			return nil, io.EOF
		}
		once = true
		return []int{100}, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{100}, s)

	// no pages
	s, err = Collect(PageFunc(func() ([]int, error) {
		return nil, io.EOF
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// lots of empty pages
	n = 0
	s, err = Collect(PageFunc(func() ([]int, error) {
		n++
		switch n {
		case 5:
			return []int{1, 2, 3}, nil
		case 10:
			return []int{4, 5, 6}, nil
		case 15:
			return nil, io.EOF
		default:
			return nil, nil
		}
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, s)

	// only empty and/or nil pages
	n = 0
	s, err = Collect(PageFunc(func() ([]int, error) {
		n++
		if n > 20 {
			return nil, io.EOF
		}
		if n%2 == 0 {
			return []int{}, nil
		}
		return nil, nil
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// eventual failure
	n = 0
	s, err = Collect(PageFunc(func() ([]int, error) {
		n++
		if n > 3 {
			return nil, fmt.Errorf("bad things")
		}
		return []int{1, 2, 3}, nil
	}))
	require.Error(t, err)
	require.Equal(t, []int{1, 2, 3, 1, 2, 3, 1, 2, 3}, s)

	// immediate failure
	err = Drain(PageFunc(func() ([]int, error) {
		return nil, fmt.Errorf("very bad things")
	}))
	require.Error(t, err)
}

// TestEmpty tests the Empty/Fail stream.
func TestEmpty(t *testing.T) {
	t.Parallel()

	// empty case
	s, err := Collect(Empty[int]())
	require.NoError(t, err)
	require.Empty(t, s)

	// normal error case
	s, err = Collect(Fail[int](fmt.Errorf("unexpected error")))
	require.Error(t, err)
	require.Empty(t, s)

	// nil error case
	s, err = Collect(Fail[int](nil))
	require.NoError(t, err)
	require.Empty(t, s)
}

// TestOnceFunc tests the OnceFunc stream combinator.
func TestOnceFunc(t *testing.T) {
	t.Parallel()

	// single-element variant
	s, err := Collect(OnceFunc(func() (int, error) {
		return 1, nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1}, s)

	// empty stream case
	s, err = Collect(OnceFunc(func() (int, error) {
		return 1, io.EOF
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// error case
	s, err = Collect(OnceFunc(func() (int, error) {
		return 1, fmt.Errorf("unexpected error")
	}))
	require.Error(t, err)
	require.Empty(t, s)
}

func TestCollectPages(t *testing.T) {
	t.Parallel()

	tts := []struct {
		pages  [][]string
		expect []string
		err    error
		desc   string
	}{
		{
			pages: [][]string{
				{"foo", "bar"},
				{},
				{"bin", "baz"},
			},
			expect: []string{
				"foo",
				"bar",
				"bin",
				"baz",
			},
			desc: "basic-depagination",
		},
		{
			pages: [][]string{
				{"one"},
			},
			expect: []string{"one"},
			desc:   "single-element-case",
		},
		{
			desc: "empty-case",
		},
		{
			err:  fmt.Errorf("failure"),
			desc: "error-case",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			var stream Stream[[]string]
			if tt.err == nil {
				stream = Slice(tt.pages)
			} else {
				stream = Fail[[]string](tt.err)
			}
			collected, err := CollectPages(stream)
			if tt.err == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if len(tt.expect) == 0 {
				require.Empty(t, collected)
			} else {
				require.Equal(t, tt.expect, collected)
			}
		})
	}
}

// TestSkip tests the Skip combinator.
func TestSkip(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(Skip(Slice([]int{1, 2, 3, 4}), 2))
	require.NoError(t, err)
	require.Equal(t, []int{3, 4}, s)

	// skip all
	s, err = Collect(Skip(Slice([]int{1, 2, 3, 4}), 4))
	require.NoError(t, err)
	require.Empty(t, s)

	// skip none
	s, err = Collect(Skip(Slice([]int{1, 2, 3, 4}), 0))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// negative skip
	s, err = Collect(Skip(Slice([]int{1, 2, 3, 4}), -1))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// skip more than available
	s, err = Collect(Skip(Slice([]int{1, 2, 3, 4}), 5))
	require.NoError(t, err)
	require.Empty(t, s)

	// positive skip on empty stream
	s, err = Collect(Skip(Empty[int](), 2))
	require.NoError(t, err)
	require.Empty(t, s)

	// zero skip on empty stream
	s, err = Collect(Skip(Empty[int](), 0))
	require.NoError(t, err)
	require.Empty(t, s)

	// negative skip on empty stream
	s, err = Collect(Skip(Empty[int](), -1))
	require.NoError(t, err)
	require.Empty(t, s)

	// immediate failure
	err = Drain(Skip(Fail[int](fmt.Errorf("unexpected error")), 1))
	require.Error(t, err)

	// failure during skip
	err = Drain(Skip(Chain(
		Slice([]int{1, 2}),
		Fail[int](fmt.Errorf("unexpected error")),
		Slice([]int{3, 4}),
	), 3))
	require.Error(t, err)
}

// TestFlatten tests the Flatten combinator.
func TestFlatten(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(Flatten(Slice([]Stream[int]{
		Slice([]int{1, 2}),
		Slice([]int{3, 4}),
		Slice([]int{5, 6}),
	})))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, s)

	// empty stream
	s, err = Collect(Flatten(Empty[Stream[int]]()))
	require.NoError(t, err)
	require.Empty(t, s)

	// empty substreams
	s, err = Collect(Flatten(Slice([]Stream[int]{
		Empty[int](),
		Slice([]int{1, 2, 3}),
		Empty[int](),
		Slice([]int{4, 5, 6}),
		Empty[int](),
	})))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, s)

	// immediate failure
	err = Drain(Flatten(Fail[Stream[int]](fmt.Errorf("unexpected error"))))
	require.Error(t, err)

	// failure during streaming
	s, err = Collect(Flatten(Slice([]Stream[int]{
		Slice([]int{1, 2}),
		Fail[int](fmt.Errorf("unexpected error")),
		Slice([]int{3, 4}),
	})))
	require.Error(t, err)
	require.Equal(t, []int{1, 2}, s)
}

// TestMapErr tests the MapErr combinator.
func TestMapErr(t *testing.T) {
	t.Parallel()

	// normal inject error
	err := Drain(MapErr(Slice([]int{1, 2, 3}), func(err error) error {
		require.NoError(t, err)
		return fmt.Errorf("unexpected error")
	}))
	require.Error(t, err)

	// empty inject error
	err = Drain(MapErr(Empty[int](), func(err error) error {
		require.NoError(t, err)
		return fmt.Errorf("unexpected error")
	}))
	require.Error(t, err)

	// normal suppress error
	s, err := Collect(MapErr(Chain(Slice([]int{1, 2, 3}), Fail[int](fmt.Errorf("unexpected error"))), func(err error) error {
		require.Error(t, err)
		return nil
	}))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// empty suppress error
	s, err = Collect(MapErr(Fail[int](fmt.Errorf("unexpected error")), func(err error) error {
		require.Error(t, err)
		return nil
	}))
	require.NoError(t, err)
	require.Empty(t, s)
}

// TestRateLimitFailure verifies the expected failure conditions of the RateLimit helper.
func TestRateLimitFailure(t *testing.T) {
	t.Parallel()

	var limiterError = errors.New("limiter-error")
	var streamError = errors.New("stream-error")

	tts := []struct {
		desc    string
		items   int
		stream  error
		limiter error
		expect  error
	}{
		{
			desc:    "simultaneous",
			stream:  streamError,
			limiter: limiterError,
			expect:  streamError,
		},
		{
			desc:   "stream-only",
			stream: streamError,
			expect: streamError,
		},
		{
			desc:    "limiter-only",
			limiter: limiterError,
			expect:  limiterError,
		},
		{
			desc:    "limiter-graceful",
			limiter: io.EOF,
			expect:  nil,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			// stream must be non-empty for limiter to be triggered
			stream := Once(1)
			if tt.stream != nil {
				stream = Fail[int](tt.stream)
			}
			err := Drain(RateLimit(stream, func() error { return tt.limiter }))
			if tt.expect == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.expect)
		})
	}
}

// TestRateLimit sets up a concurrent channel-based limiter and verifies its effect on a pool of workers consuming
// items from streams.
func TestRateLimit(t *testing.T) {
	t.Parallel()

	const workers = 16
	const maxItemsPerWorker = 16
	const tokens = 100
	const burst = 10

	lim := make(chan struct{}, burst)
	done := make(chan struct{})

	results := make(chan error, workers)

	items := make(chan struct{}, tokens+1)

	for range workers {
		go func() {
			stream := RateLimit(repeat("some-item", maxItemsPerWorker), func() error {
				select {
				case <-lim:
					return nil
				case <-done:
					// make sure we still consume remaining tokens even if 'done' is closed (simplifies
					// test logic by letting us close 'done' immediately after sending last token without
					// worrying about racing).
					select {
					case <-lim:
						return nil
					default:
						return io.EOF
					}
				}
			})

			for _, err := range stream {
				if err != nil {
					results <- err
					return
				}
				items <- struct{}{}
			}

			results <- nil
		}()
	}

	// limiter isn't applied until after the first item is yielded, so pop the first item
	// from each worker immediately to simplify test logic.
	for range workers {
		select {
		case <-items:
		case <-time.After(time.Second * 10):
			require.FailNow(t, "timeout waiting for first item")
		}
	}

	// yielded tracks total number of tokens yielded on limiter channel
	var yielded int

	// do an initial fill of limiter channel
	for range burst {
		select {
		case lim <- struct{}{}:
			yielded++
		default:
			require.FailNow(t, "initial burst should never block")
		}
	}

	var consumed int

	// consume item receipt events
	timeoutC := time.After(time.Second * 30)
	for range burst {
		select {
		case <-items:
			consumed++
		case <-timeoutC:
			require.FailNow(t, "timeout waiting for item")
		}
	}

	// ensure no more items available
	select {
	case <-items:
		require.FailNow(t, "received item without corresponding token yield")
	default:
	}

	// yield the rest of the tokens
	for yielded < tokens {
		select {
		case lim <- struct{}{}:
			yielded++
		case <-timeoutC:
			require.FailNow(t, "timeout waiting to yield token")
		}
	}

	// signal workers that they should exit once remaining tokens
	// are consumed.
	close(done)

	// wait for all workers to finish
	for range workers {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-timeoutC:
			require.FailNow(t, "timeout waiting for worker to exit")
		}
	}

	// consume the rest of the item events
ConsumeItems:
	for {
		select {
		case <-items:
			consumed++
		default:
			break ConsumeItems
		}
	}

	// note that total number of processed items may actually vary since we are rate-limiting
	// how frequently a stream is *polled*, not how frequently it yields an item. A stream being
	// polled may result in us discovering that it is empty, in which case a limiter token is still
	// consumed, but no item is yielded.
	require.LessOrEqual(t, consumed, tokens)
	require.GreaterOrEqual(t, consumed, tokens-workers)
}

// repeat repeats the same item N times
func repeat[T any](item T, count int) Stream[T] {
	var n int
	return Func(func() (T, error) {
		n++
		if n > count {
			var zero T
			return zero, io.EOF
		}
		return item, nil
	})
}

// TestMergeStreams tests the MergeStreams adapter.
func TestMergeStreams(t *testing.T) {
	t.Parallel()

	// Mock compare function that favors the lower value.
	compareFunc := func(a, b int) bool {
		return a <= b
	}

	// Test the case where the streams should have interlaced values.
	t.Run("interlaced streams", func(t *testing.T) {
		streamA := Slice([]int{1, 3, 5})
		streamB := Slice([]int{2, 4, 6})

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)
	})

	// Test the case where streamA is empty.
	t.Run("stream A empty", func(t *testing.T) {
		streamA := Empty[int]()
		streamB := Slice([]int{1, 2, 3})

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, out)
	})

	// Test the case where streamB is empty.
	t.Run("stream B empty", func(t *testing.T) {
		streamA := Slice([]int{1, 2, 3})
		streamB := Empty[int]()

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, out)
	})

	// Test the case where both streams are empty.
	t.Run("both streams empty", func(t *testing.T) {
		streamA := Empty[int]()
		streamB := Empty[int]()

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Empty(t, out)
	})

	// Test the case where every value in streamA is lower than every value in streamB.
	t.Run("compare always favors A", func(t *testing.T) {
		streamA := Slice([]int{1, 2, 3})
		streamB := Slice([]int{4, 5, 6})

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)
	})

	// Test the case where every value in streamB is lower than every value in streamA.
	t.Run("compare always favors B", func(t *testing.T) {
		streamA := Slice([]int{4, 5, 6})
		streamB := Slice([]int{1, 2, 3})

		resultStream := MergeStreams(streamA, streamB, compareFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)
	})
}

func TestTakeWhile(t *testing.T) {
	t.Parallel()

	// Regular operation
	out, err := Collect(TakeWhile(
		Slice([]int{1, 2, 3, 4, 5, 6}),
		func(item int) bool {
			return item < 4
		},
	))

	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, out)

	out, err = Collect(TakeWhile(
		Slice([]int{1, 2, 3, 4, 5, 6}),
		func(_ int) bool {
			return true
		},
	))

	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)

	// Propagate error
	out, err = Collect(TakeWhile(
		Fail[int](fmt.Errorf("unexpected error")),
		func(_ int) bool {
			return true
		},
	))

	require.Error(t, err)
	require.Empty(t, out)

	// Test early exit
	var actual []int
	TakeWhile(Slice([]int{1, 2, 3, 4, 5, 6}),
		func(item int) bool { return true },
	)(func(item int, err error) bool {
		actual = append(actual, item)
		return item < 3
	})
	require.Equal(t, []int{1, 2, 3}, actual)

}
