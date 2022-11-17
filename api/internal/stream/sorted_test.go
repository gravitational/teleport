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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinSortedBasics(t *testing.T) {
	tts := []struct {
		l, r, e []int
		d       string
	}{
		{
			l: []int{1, 2, 3},
			r: []int{1, 2, 3},
			e: []int{1, 1, 2, 2, 3, 3},
			d: "duplicate items",
		},
		{
			l: []int{1, 3, 5},
			r: []int{0, 2, 4},
			e: []int{0, 1, 2, 3, 4, 5},
			d: "alternating items",
		},
		{
			l: []int{1, 2, 3},
			r: []int{4, 5, 6},
			e: []int{1, 2, 3, 4, 5, 6},
			d: "non-overlapping",
		},
		{
			l: []int{1, 2},
			r: []int{0, 3, 4, 5, 6},
			e: []int{0, 1, 2, 3, 4, 5, 6},
			d: "different lengths",
		},
		{
			l: []int{},
			r: []int{1, 2, 3},
			e: []int{1, 2, 3},
			d: "left empty",
		},
		{
			l: []int{1, 2, 3},
			r: []int{},
			e: []int{1, 2, 3},
			d: "right empty",
		},
		{
			d: "all empty",
		},
	}

	for _, tt := range tts {
		s, err := Collect(JoinSorted(
			Slice(tt.l),
			Slice(tt.r),
			func(i, j int) bool {
				return i < j
			},
		))
		require.NoError(t, err, "desc=%q", tt.d)
		require.Equal(t, tt.e, s, "desc=%q", tt.d)
	}
}

func TestJoinSortedFailure(t *testing.T) {
	// left fails immediately
	err := Drain(JoinSorted(
		Fail[int](fmt.Errorf("bad stuff")),
		Slice([]int{1, 2, 3}),
		func(i, j int) bool {
			return i < j
		},
	))
	require.Error(t, err)

	// right fails immediately
	err = Drain(JoinSorted(
		Slice([]int{1, 2, 3}),
		Fail[int](fmt.Errorf("bad stuff")),
		func(i, j int) bool {
			return i < j
		},
	))
	require.Error(t, err)

	// both fail immediately
	err = Drain(JoinSorted(
		Fail[int](fmt.Errorf("bad stuff")),
		Fail[int](fmt.Errorf("other bad stuff")),
		func(i, j int) bool {
			return i < j
		},
	))
	require.Error(t, err)

	// left fails
	s, err := Collect(JoinSorted(
		failAfter(1, 3),
		Slice([]int{0, 2, 4, 6}),
		func(i, j int) bool {
			return i < j
		},
	))
	require.Error(t, err)
	require.Equal(t, []int{0, 1, 2, 3}, s)

	// right fails
	s, err = Collect(JoinSorted(
		Slice([]int{1, 3, 5, 7}),
		failAfter(0, 2),
		func(i, j int) bool {
			return i < j
		},
	))
	require.Error(t, err)
	require.Equal(t, []int{0, 1, 2}, s)
}

func TestDedupSorted(t *testing.T) {
	// basic case
	s, err := Collect(DedupSorted(
		Slice([]int{1, 1, 2, 3, 3, 3, 4}),
		func(i, j int) bool {
			return i == j
		},
	))
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// single-element case
	s, err = Collect(DedupSorted(
		Once(1),
		func(i, j int) bool {
			return i == j
		},
	))
	require.NoError(t, err)
	require.Equal(t, []int{1}, s)

	// zero element case
	s, err = Collect(DedupSorted(
		Empty[int](),
		func(i, j int) bool {
			return i == j
		},
	))
	require.NoError(t, err)
	require.Len(t, s, 0)

	// eventual error ending on new number
	s, err = Collect(DedupSorted(
		failAfter(1, 1, 2, 3, 4),
		func(i, j int) bool {
			return i == j
		},
	))
	require.Error(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// eventual error ending on duplicate
	s, err = Collect(DedupSorted(
		failAfter(1, 2, 2, 3, 3),
		func(i, j int) bool {
			return i == j
		},
	))
	require.Error(t, err)
	require.Equal(t, []int{1, 2, 3}, s)

	// immediate error case
	err = Drain(DedupSorted(
		Fail[int](fmt.Errorf("when it rains")),
		func(i, j int) bool { panic("unreachable") },
	))
	require.Error(t, err)
}

func failAfter[T any](items ...T) Stream[T] {
	inner := Slice(items)
	return Func(func() (T, error) {
		if !inner.Next() {
			var zero T
			return zero, fmt.Errorf("failed after %d", len(items))
		}
		return inner.Item(), nil
	})
}
