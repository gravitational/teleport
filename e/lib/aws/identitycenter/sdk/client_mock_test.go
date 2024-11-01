package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientMock(t *testing.T) {
	ctx := context.Background()
	var c Client = NewClientMock()
	t.Run("should list users with account and permission assignments", func(t *testing.T) {
		resp, err := c.ListGroupsWithAccountAndPermAssignment(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})

	t.Run("should list groups with account and permission assignments", func(t *testing.T) {
		resp, err := c.ListUsersWithAccountAndPermAssignment(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})
}
