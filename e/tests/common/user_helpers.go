package common

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func MustCreateUser(t *testing.T, cluster *SUT, name string, roles ...string) types.User {
	user, err := types.NewUser(name)
	require.NoError(t, err)

	for _, r := range roles {
		user.AddRole(r)
	}
	created, err := cluster.Teleport.Process.GetAuthServer().CreateUser(context.Background(), user)
	require.NoError(t, err, "creating user %q", user)

	return created
}

func MustCreateUserWithCleanup(t *testing.T, cluster *SUT, name string, roles ...string) types.User {
	user := MustCreateUser(t, cluster, name, roles...)
	t.Cleanup(func() {
		cluster.Teleport.Process.GetAuthServer().DeleteUser(context.Background(), name)
	})
	return user
}

func UpdateUser(ctx context.Context, usersSvc services.UsersService, username string, mutateFn func(types.User)) error {
	u, err := usersSvc.GetUser(ctx, username, false /* without secrets */)
	if err != nil {
		return trace.Wrap(err)
	}
	mutateFn(u)
	if _, err = usersSvc.UpdateUser(ctx, u); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
