package scimsdk

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/utils"
)

func TestStream(t *testing.T) {
	t.Run("Users", func(t *testing.T) {
		setup := func(total int) (*ClientMock, []*User) {
			allUsers := make([]*User, total)
			for i := range allUsers {
				allUsers[i] = &User{
					ID:          fmt.Sprintf("user-%03d", i),
					UserName:    fmt.Sprintf("user-%d", i),
					DisplayName: fmt.Sprintf("User %d", i),
				}
			}

			client := NewSCIMClientMock()
			client.Users = utils.FromSlice(allUsers, (*User).GetID)
			return client, allUsers
		}
		testStreamResources(t, setup, StreamUsers)
	})

	t.Run("Groups", func(t *testing.T) {
		setup := func(total int) (*ClientMock, []*Group) {
			allGroups := make([]*Group, total)
			for i := range allGroups {
				allGroups[i] = &Group{
					ID:          fmt.Sprintf("group-%03d", i),
					DisplayName: fmt.Sprintf("Group %d", i),
				}
			}

			client := NewSCIMClientMock()
			client.Groups = utils.FromSlice(allGroups, (*Group).GetID)
			return client, allGroups
		}
		testStreamResources(t, setup, StreamGroups)
	})
}

// streamFn is a function that returns a stream of resources.
type streamFn[T any] func(context.Context, Client, ...QueryOption) stream.Stream[T]

// testSetupFn is a function that sets up a test client and populates it with a
// set of resources.
type testSetupFn[T any] func(int) (*ClientMock, []T)

// testStreamResources tests that a resource streaming function handles basic
// pagination and corner-case
func testStreamResources[T any](t *testing.T, setup testSetupFn[T], functionUnderTest streamFn[T]) {
	// Asserts that the streamer handles an empty result set correctly.
	t.Run("Empty", func(t *testing.T) {
		const totalResources = 0
		const pageSize = 3

		client, _ := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithCount(pageSize)))
		require.NoError(t, err)
		require.Empty(t, actualResources)
	})

	// Asserts that the streamer works when all results fit in a single page.
	t.Run("SinglePage", func(t *testing.T) {
		const totalResources = 3
		const pageSize = 50

		client, expectedResources := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithCount(pageSize)))
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources, actualResources)
	})

	// Asserts that the streamer works starting from a nonzero offset.
	t.Run("OffsetStart", func(t *testing.T) {
		const totalResources = 20
		const pageSize = 3

		client, expectedResources := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithStartIndex(12), WithCount(pageSize)))
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources[11:], actualResources)
	})

	// Asserts that the streamer works (i.e. returns an empty list and doesn't
	// crash) when the supplied start index exceeds the total number of resources.
	t.Run("OutOfBoundsStart", func(t *testing.T) {
		const totalResources = 10
		const pageSize = 3

		client, _ := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithStartIndex(163), WithCount(pageSize)))
		require.NoError(t, err)
		require.Empty(t, actualResources)
	})

	// Asserts that the streamer correctly advances startIndex across pages,
	// returning every resource exactly once
	t.Run("ExactPagination", func(t *testing.T) {
		// Expected pages: [0,1,2], [3,4,5], [6,7,8] = 3 pages
		const totalResources = 9
		const pageSize = 3

		client, expectedResources := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithCount(pageSize)))
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources, actualResources)
	})

	// Asserts that the streamer correctly advances startIndex across pages,
	// returning every resource exactly once and handling trailing resources
	// on an incomplete page
	t.Run("TrailingPagination", func(t *testing.T) {
		// Expected pages: [0,1,2], [3,4,5], [6] = 3 pages
		const totalResources = 7
		const pageSize = 3

		client, expectedResources := setup(totalResources)

		actualResources, err := stream.Collect(functionUnderTest(t.Context(), client, WithCount(pageSize)))
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources, actualResources)
	})
}
