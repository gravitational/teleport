/*
Copyright 2023 Gravitational, Inc.

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

package pagination

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIteratePages(t *testing.T) {
	u1, err := types.NewUser("user1")
	require.NoError(t, err)
	u2, err := types.NewUser("user2")
	require.NoError(t, err)
	u3, err := types.NewUser("user3")
	require.NoError(t, err)

	// Regular pager with a happy map function.
	numCalls := 0
	testPager := func(pageToken string) ([]types.User, string, error) {
		numCalls++
		switch pageToken {
		case "":
			return []types.User{u1}, "1", nil
		case "1":
			return []types.User{u2}, "2", nil
		case "2":
			return []types.User{u3}, "", nil
		}

		return nil, "", trace.BadParameter("unrecognized paging token")
	}

	accumulatedUsers := []types.User{}
	require.NoError(t, Iterate("", testPager, func(u types.User) (bool, error) {
		accumulatedUsers = append(accumulatedUsers, u)
		return true, nil
	}))
	require.Equal(t, 3, numCalls)
	require.Empty(t, cmp.Diff([]types.User{u1, u2, u3}, accumulatedUsers))

	// Regular pager with a non-empty initial page token.
	numCalls = 0
	accumulatedUsers = []types.User{}
	require.NoError(t, Iterate("1", testPager, func(u types.User) (bool, error) {
		accumulatedUsers = append(accumulatedUsers, u)
		return true, nil
	}))
	require.Equal(t, 2, numCalls)
	require.Empty(t, cmp.Diff([]types.User{u2, u3}, accumulatedUsers))

	// Create a pager that errors.
	errorPager := func(pageToken string) ([]types.User, string, error) {
		switch pageToken {
		case "":
			return []types.User{u1}, "1", nil
		case "1":
			return []types.User{u2}, "2", nil
		}

		return nil, "", trace.BadParameter("unrecognized paging token")
	}

	accumulatedUsers = []types.User{}
	// Use IteratePagesWithPageSize to verify the page is passed in properly.
	require.ErrorIs(t, Iterate("", errorPager, func(u types.User) (bool, error) {
		accumulatedUsers = append(accumulatedUsers, u)
		return true, nil
	}), trace.BadParameter("unrecognized paging token"))
	require.Empty(t, cmp.Diff([]types.User{u1, u2}, accumulatedUsers))

	// Create an map function that errors.
	require.ErrorIs(t, Iterate("", testPager, func(u types.User) (bool, error) {
		return true, trace.BadParameter("expected error")
	}), trace.BadParameter("expected error"))

	// Create an map function that short circuits.
	numCalls = 0
	accumulatedUsers = []types.User{}
	require.NoError(t, Iterate("", testPager, func(u types.User) (bool, error) {
		accumulatedUsers = append(accumulatedUsers, u)
		return false, nil
	}))
	require.Equal(t, 1, numCalls)
	require.Empty(t, cmp.Diff([]types.User{u1}, accumulatedUsers))
}
