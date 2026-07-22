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
	"fmt"
	"io"
	"strconv"
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
	s, err = Collect(Slice[int](nil))
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

// TestMapWhile tests the MapWhile combinator.
func TestMapWhile(t *testing.T) {
	t.Parallel()

	// normal usage
	s, err := Collect(MapWhile(Slice([]int{1, 2, 3, 4}), func(i int) (string, bool) {
		if i == 3 {
			return "", false
		}
		return fmt.Sprintf("%d", i*10), true
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"10", "20"}, s)

	// halt after 1 element
	s, err = Collect(MapWhile(Slice([]int{1, 2, 3, 4}), func(i int) (string, bool) {
		if i == 1 {
			return "one", true
		}
		return "", false
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"one"}, s)

	// halt immediately
	s, err = Collect(MapWhile(Slice([]int{1, 2, 3, 4}), func(_ int) (string, bool) {
		return "", false
	}))
	require.NoError(t, err)
	require.Empty(t, s)

	// empty stream
	s, err = Collect(MapWhile(Empty[int](), func(_ int) (string, bool) { panic("unreachable") }))
	require.NoError(t, err)
	require.Empty(t, s)

	// failure
	err = Drain(MapWhile(Fail[int](fmt.Errorf("unexpected error")), func(_ int) (string, bool) { panic("unreachable") }))
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

func TestTake(t *testing.T) {
	t.Parallel()

	intSlice := func(n int) []int {
		s := make([]int, 0, n)
		for i := 0; i < n; i++ {
			s = append(s, i)
		}
		return s
	}

	tests := []struct {
		name           string
		input          []int
		n              int
		expectedOutput []int
		expectMore     bool
	}{
		{
			name:           "empty stream",
			input:          []int{},
			n:              10,
			expectedOutput: []int{},
			expectMore:     false,
		},
		{
			name:           "full stream",
			input:          intSlice(20),
			n:              10,
			expectedOutput: intSlice(10),
			expectMore:     true,
		},
		{
			name:           "drain stream of size n",
			input:          intSlice(10),
			n:              10,
			expectedOutput: intSlice(10),
			expectMore:     true,
		},
		{
			name:           "drain stream of size < n",
			input:          intSlice(5),
			n:              10,
			expectedOutput: intSlice(5),
			expectMore:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stream := Slice(tc.input)
			output, more := Take(stream, tc.n)
			require.Equal(t, tc.expectedOutput, output)
			require.Equal(t, tc.expectMore, more)
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
			err := Drain(RateLimit(Fail[int](tt.stream), func() error { return tt.limiter }))
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

	for i := 0; i < workers; i++ {
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

			for stream.Next() {
				items <- struct{}{}
			}

			results <- stream.Done()
		}()
	}

	// yielded tracks total number of tokens yielded on limiter channel
	var yielded int

	// do an initial fill of limiter channel
	for i := 0; i < burst; i++ {
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
	for i := 0; i < burst; i++ {
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
	for i := 0; i < workers; i++ {
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

	// Mock convert function that converts the strings in streamB to integers.
	convertBFunc := func(val string) int {
		bValue, _ := strconv.Atoi(val)
		return bValue
	}

	// Since streamA is already the type we want from the merged stream, the convertA function just returns the item as-is.
	convertAFunc := func(item int) int { return item }

	// Mock compare function that favors the lower value.
	compareFunc := func(a int, b string) bool {
		return a <= convertBFunc(b)
	}

	// Test the case where the streams should have interlaced values.
	t.Run("interlaced streams", func(t *testing.T) {
		streamA := Slice([]int{1, 3, 5})
		streamB := Slice([]string{"2", "4", "6"})

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})

	// Test the case where streamA is empty.
	t.Run("stream A empty", func(t *testing.T) {
		streamA := Empty[int]()
		streamB := Slice([]string{"1", "2", "3"})

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})

	// Test the case where streamB is empty.
	t.Run("stream B empty", func(t *testing.T) {
		streamA := Slice([]int{1, 2, 3})
		streamB := Empty[string]()

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})

	// Test the case where both streams are empty.
	t.Run("both streams empty", func(t *testing.T) {
		streamA := Empty[int]()
		streamB := Empty[string]()

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Empty(t, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})

	// Test the case where every value in streamA is lower than every value in streamB.
	t.Run("compare always favors A", func(t *testing.T) {
		streamA := Slice([]int{1, 2, 3})
		streamB := Slice([]string{"4", "5", "6"})

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})

	// Test the case where every value in streamB is lower than every value in streamA.
	t.Run("compare always favors B", func(t *testing.T) {
		streamA := Slice([]int{4, 5, 6})
		streamB := Slice([]string{"1", "2", "3"})

		resultStream := MergeStreams(streamA, streamB, compareFunc, convertAFunc, convertBFunc)
		out, err := Collect(resultStream)

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)

		err = resultStream.Done()
		require.NoError(t, err)
	})
}
