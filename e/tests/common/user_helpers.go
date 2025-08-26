package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

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
