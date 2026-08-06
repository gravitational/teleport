// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func testToken(i int) string {
	return string(ordered.Encode(uint64(i)))
}

type testListerFixture struct {
	lister genericLister[int, string]
	items  []int
}

func newTestListerFixture(n int, isDesc bool, defaultPageSize int) testListerFixture {
	const testListerKind = "generic_lister_test_kind"
	st := newStore(testListerKind,
		func(i int) int { return i },
		map[string]func(int) string{
			"num": testToken,
		})

	items := make([]int, n)
	for i := range items {
		items[i] = i
		_ = st.put(i)
	}

	coll := &collection[int, string]{
		store: st,
		watch: types.WatchKind{Kind: testListerKind},
	}

	cache := &Cache{}
	cache.ok = true
	cache.confirmedKinds = map[resourceKind]types.WatchKind{
		{kind: testListerKind}: {Kind: testListerKind},
	}

	lister := genericLister[int, string]{
		cache:           cache,
		collection:      coll,
		index:           "num",
		isDesc:          isDesc,
		defaultPageSize: defaultPageSize,
		upstreamList: func(_ context.Context, pageSize int, start string) ([]int, string, error) {
			// We do not test the upstream list behavior since that depends on the actual implementation,
			// testing the mock has no value here.
			panic("upstreamList should not be called in this test; the cache is healthy")
		},
		nextToken: testToken,
	}

	return testListerFixture{lister: lister, items: items}
}

func TestGenericLister_ReturnsEmptyTokenWhenNoMoreItems(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 300).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")
		defaultPageSize := rapid.IntRange(1, 100).Draw(t, "defaultPageSize")

		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		token := ""
		returned := 0
		for returned < n {
			pageSize := rapid.IntRange(-defaults.DefaultChunkSize, defaults.DefaultChunkSize*2).Draw(t, "pageSize")
			items, next, err := fixture.lister.list(t.Context(), pageSize, token)
			require.NoError(t, err)
			returned += len(items)
			token = next
		}
		require.Empty(t, token, "next token should be empty when all items have been returned")
		require.Equal(t, n, returned, "all items should have been returned")
	})
}

func TestGenericLister_BadPageSizeFallsBackToDefault(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 200).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")
		defaultPageSize := rapid.IntRange(1, 50).Draw(t, "defaultPageSize")
		badPageSize := rapid.IntRange(-defaults.DefaultChunkSize, 0).Draw(t, "badPageSize")

		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		gotItems, gotNext, err := fixture.lister.list(t.Context(), badPageSize, "")
		require.NoError(t, err)

		wantItems, wantNext, err := fixture.lister.list(t.Context(), defaultPageSize, "")
		require.NoError(t, err)

		require.Equal(t, wantItems, gotItems, "a bad page size should behave exactly like the resolved default page size")
		require.Equal(t, wantNext, gotNext)
	})
}

func TestGenericLister_MayReturnLessItemsThanRequested(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 2*defaults.DefaultChunkSize).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")
		defaultPageSize := rapid.IntRange(1, 50).Draw(t, "defaultPageSize")
		pageSize := rapid.IntRange(1, 2*defaults.DefaultChunkSize).Draw(t, "pageSize")

		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		items, _, err := fixture.lister.list(t.Context(), pageSize, "")
		require.NoError(t, err)
		require.LessOrEqual(t, len(items), pageSize)
	})
}

func TestGenericLister_ReturnsAllItems(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 300).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")
		defaultPageSize := rapid.IntRange(1, 25).Draw(t, "defaultPageSize")

		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		got := make([]int, 0, n)
		for item, err := range fixture.lister.Range(t.Context(), "", "") {
			require.NoError(t, err)
			got = append(got, item)
		}

		want := append([]int{}, fixture.items...)
		if isDesc {
			slices.Reverse(want)
		}

		require.Equal(t, want, got)
	})
}

func TestGenericLister_PaginationTerminatesRegardlessOfPageSize(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 500).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")

		start := ""
		if rapid.Bool().Draw(t, "hasStart") {
			start = testToken(rapid.IntRange(0, n*2).Draw(t, "start"))
		}
		defaultPageSize := rapid.IntRange(1, 500).Draw(t, "defaultPageSize")
		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		// Allow a little headroom over the exact expected item count
		const guard = 5
		token := start
		for range n + guard {
			pageSize := rapid.IntRange(-defaults.DefaultChunkSize, defaults.DefaultChunkSize*2).Draw(t, "pageSize")
			_, next, err := fixture.lister.list(t.Context(), pageSize, token)
			require.NoError(t, err)
			if next == "" {
				return // pagination terminated as expected
			}
			token = next
		}

		require.FailNow(t, "pagination did not terminate")
	})
}

func TestGenericLister_ItemsAreNeverDuplicated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 300).Draw(t, "n")
		isDesc := rapid.Bool().Draw(t, "isDesc")
		// Small page sizes stress the pagination boundary logic
		defaultPageSize := rapid.IntRange(1, 5).Draw(t, "defaultPageSize")
		fixture := newTestListerFixture(n, isDesc, defaultPageSize)

		seen := make(map[int]int, n)
		for item, err := range fixture.lister.Range(context.Background(), "", "") {
			require.NoError(t, err)
			seen[item]++
			require.Equal(t, 1, seen[item], "item %d was returned more than once", item)
		}

		require.Len(t, seen, n)
	})
}
