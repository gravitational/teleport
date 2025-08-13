package accessgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestService_Query(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env := setup(t)

	env.createUsersAndRoles(t, ctx)

	t.Run("Query is not allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{"nop-test-role"}, map[string][]string{})

		_, err := env.service.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.ErrorContains(t, err, "not allowed to read the access graph")

		require.Equal(t, 0, env.fakeAccessGraphServer.calledFunctions["Query"])
	})

	t.Run("Query is allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user", []string{"test-role"}, map[string][]string{})

		_, err := env.service.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.NoError(t, err)

		require.Equal(t, 1, env.fakeAccessGraphServer.calledFunctions["Query"])
	})

	t.Run("GetFile is always allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{"nop-test-role"}, map[string][]string{})

		_, err := env.service.GetFile(ctx, &accessgraphv1alpha.GetFileRequest{})
		require.NoError(t, err)
	})
}
