package devicetrustv1_test

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// authorizerUserKey is used by [userAwareAuthorizer].
const authorizerUserKey = "user"

// contextWithUser returns an outbound context for the specified user.
// Meant to be used in conjunction with [userAwareAuthorizer].
func contextWithUser(ctx context.Context, user string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authorizerUserKey, user)
}

// userAwareAuthorizer allows access based on the context user. See [metadata]
// and [authorizerUserKey]
// Used by CreateDeviceEnrollToken/auto-enroll tests.
type userAwareAuthorizer struct {
	knownUsers        []string
	authorizedUsers   []string
	userToSystemRoles map[string][]types.SystemRole
}

func (a *userAwareAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	// Fetch the user from the "user" metadata key.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("ctx lacks metadata")
	}
	users := md.Get(authorizerUserKey)
	if len(users) == 0 || len(users[0]) == 0 {
		return nil, errors.New("ctx lacks user")
	}
	username := users[0]

	// Fail Authorize for unknown users.
	found := slices.Contains(a.knownUsers, username)
	if !found {
		return nil, trace.AccessDenied("unknown user")
	}

	user, err := types.NewUser(username)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	var identity authz.IdentityGetter
	if roles, ok := a.userToSystemRoles[username]; ok {
		var firstRole types.SystemRole
		systemRoles := make([]string, len(roles))
		for i, role := range roles {
			if i == 0 {
				firstRole = role
			}
			systemRoles[i] = string(role)
		}

		identity = authz.BuiltinRole{
			Role:                  firstRole,
			AdditionalSystemRoles: roles,
			Username:              username,
			Identity: tlsca.Identity{
				SystemRoles: systemRoles,
			},
		}
	}

	return &authz.Context{
		User: user,
		Checker: &userAwareChecker{
			authorizedUsers: a.authorizedUsers,
			identity:        identity,
		},
		Identity:             identity,
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

type userAwareChecker struct {
	testenv.NoopChecker

	authorizedUsers []string
	identity        authz.IdentityGetter
}

func (c *userAwareChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace, rule, verb string) error {
	user, err := ruleCtx.GetIdentifier([]string{"user", "metadata", "name"})
	if err != nil {
		return err
	}

	for _, authz := range c.authorizedUsers {
		if user == authz {
			return nil
		}
	}
	return trace.AccessDenied("access denied")
}

func (c *userAwareChecker) HasRole(wantRole string) bool {
	if c.identity == nil {
		return false
	}

	identity := c.identity.GetIdentity()
	return slices.Contains(identity.SystemRoles, wantRole)
}
