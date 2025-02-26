/*
Copyright 2025 Gravitational, Inc.

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

	legacy "github.com/gravitational/teleport/api/internalutils/stream"
)

func TestLegacyCompat(t *testing.T) {
	t.Parallel()

	var s []int
	var err error

	// basic from legacy compat & use as a standard stream
	for n, err := range FromLegacy(legacy.Slice([]int{1, 2, 3, 4})) {
		require.NoError(t, err)
		s = append(s, n)
	}
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// basic into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		for i := 0; i < 4; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}))
	require.NoError(t, err)
	require.Equal(t, []int{0, 1, 2, 3}, s)

	// nested compat
	s, err = legacy.Collect(IntoLegacy(FromLegacy(legacy.Slice([]int{7, 8, 9}))))
	require.NoError(t, err)
	require.Equal(t, []int{7, 8, 9}, s)

	// single-element from legacy compat
	s = nil
	for n, err := range FromLegacy(legacy.Slice([]int{11})) {
		require.NoError(t, err)
		s = append(s, n)
	}
	require.Equal(t, []int{11}, s)

	// single-element into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		if !yield(111, nil) {
			return
		}
	}))
	require.NoError(t, err)
	require.Equal(t, []int{111}, s)

	// empty from legacy compat
	s = nil
	for n, err := range FromLegacy(legacy.Empty[int]()) {
		require.NoError(t, err)
		s = append(s, n)
	}

	// empty into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {}))
	require.NoError(t, err)
	require.Empty(t, s)

	// error from legacy compat
	err = nil
	for _, ierr := range FromLegacy(legacy.Fail[int](fmt.Errorf("unexpected error"))) {
		err = ierr
		break
	}
	require.Error(t, err)

	// error into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		yield(0, fmt.Errorf("unexpected error"))
	}))
	require.Error(t, err)
	require.Empty(t, s)

}
