package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

// willHaveAccessListRoles determines whether the user will have roles from access lists after login.
// It's used to determine whether to allow a user without any roles mapped from SAML connector to continue.
func willHaveAccessListRoles(ctx context.Context, authServer *auth.Server, user types.User) (bool, error) {
	// TODO(nixpig): Evaluate whether any performance implications to generating full user login state versus
	// another implementation that only evaluates whether the user has minimum of one role from Access List.
	uls, err := authServer.GeneratePureULS(ctx, user)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return len(uls.GetAccessListRoles()) > 0, nil
}
