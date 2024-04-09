package devicetrustv1_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
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
	testenv.NoopChecker

	knownUsers      []string
	authorizedUsers []string
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
	found := false
	for _, known := range a.knownUsers {
		if username == known {
			found = true
			break
		}
	}
	if !found {
		return nil, trace.AccessDenied("unknown user")
	}

	// Proceed.
	user, err := types.NewUser(username)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return &authz.Context{
		User:                 user,
		Checker:              a,
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

func (a *userAwareAuthorizer) CheckAccessToRule(ruleCtx services.RuleContext, namespace, rule, verb string) error {
	user, err := ruleCtx.GetIdentifier([]string{"user", "metadata", "name"})
	if err != nil {
		return err
	}

	for _, authz := range a.authorizedUsers {
		if user == authz {
			return nil
		}
	}
	return trace.AccessDenied("access denied")
}
