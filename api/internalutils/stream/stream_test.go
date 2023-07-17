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
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSlice tests the slice stream.
func TestSlice(t *testing.T) {
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
	require.Len(t, s, 0)
}

// TestFilterMap tests the FilterMap combinator.
func TestFilterMap(t *testing.T) {
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
	require.Len(t, s, 0)

	// empty stream
	s, err = Collect(FilterMap(Empty[int](), func(_ int) (string, bool) { panic("unreachable") }))
	require.NoError(t, err)
	require.Len(t, s, 0)

	// failure
	err = Drain(FilterMap(Fail[int](fmt.Errorf("unexpected error")), func(_ int) (string, bool) { panic("unreachable") }))
	require.Error(t, err)
}

// TestMapWhile tests the MapWhile combinator.
func TestMapWhile(t *testing.T) {
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
	require.Len(t, s, 0)

	// empty stream
	s, err = Collect(MapWhile(Empty[int](), func(_ int) (string, bool) { panic("unreachable") }))
	require.NoError(t, err)
	require.Len(t, s, 0)

	// failure
	err = Drain(MapWhile(Fail[int](fmt.Errorf("unexpected error")), func(_ int) (string, bool) { panic("unreachable") }))
	require.Error(t, err)
}

// TestFunc tests the Func stream.
func TestFunc(t *testing.T) {
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
	require.Len(t, s, 0)

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
	require.Len(t, s, 0)

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
	require.Len(t, s, 0)

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
	// empty case
	s, err := Collect(Empty[int]())
	require.NoError(t, err)
	require.Len(t, s, 0)

	// normal error case
	s, err = Collect(Fail[int](fmt.Errorf("unexpected error")))
	require.Error(t, err)
	require.Len(t, s, 0)

	// nil error case
	s, err = Collect(Fail[int](nil))
	require.NoError(t, err)
	require.Len(t, s, 0)
}

func TestCollectPages(t *testing.T) {
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
				require.Len(t, collected, 0)
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
