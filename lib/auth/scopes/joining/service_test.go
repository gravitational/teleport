package joining_test

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/scopes/joining"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func assertEqualScopedTokens(t *testing.T, expected *joiningv1.ScopedToken, val *joiningv1.ScopedToken) bool {
	return assert.Equal(t, expected.GetMetadata().GetName(), val.GetMetadata().GetName()) &&
		assert.Equal(t, expected.GetScope(), val.GetScope()) &&
		assert.Equal(t, expected.GetSpec().GetAssignedScope(), val.GetSpec().GetAssignedScope()) &&
		assert.Equal(t, expected.GetSpec().GetJoinMethod(), val.GetSpec().GetJoinMethod())
}

func TestScopedJoiningService(t *testing.T) {
	ctx := withAuthCtx(t.Context(), newAuthCtx(types.RoleAdmin))

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	svc, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	service, err := joining.New(joining.Config{
		Logger:     logtest.NewLogger(),
		Backend:    svc,
		Authorizer: fakeAuthorizer{},
	})
	require.NoError(t, err)

	token := &joiningv1.ScopedToken{
		Metadata: &headerv1.Metadata{
			Name: "testtoken",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test/aa",
			JoinMethod:    "token",
			Roles:         []string{"Node"},
		},
	}

	created, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: token,
	})
	require.NoError(t, err)
	assertEqualScopedTokens(t, token, created.GetToken())

	tokenWithMismatchedScope := proto.CloneOf(token)
	tokenWithMismatchedScope.Metadata.Name = "invalid-token"
	tokenWithMismatchedScope.Spec.AssignedScope = "/stage/aa"

	_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: tokenWithMismatchedScope,
	})
	assert.True(t, trace.IsBadParameter(err))

	// update token with reachable scope
	token.Spec.AssignedScope = "/test/bb"
	updated, err := service.UpdateScopedToken(ctx, &joiningv1.UpdateScopedTokenRequest{
		Token: token,
	})
	require.NoError(t, err)
	assertEqualScopedTokens(t, token, updated.GetToken())

	// get token
	fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name: token.Metadata.Name,
	})
	require.NoError(t, err)
	assertEqualScopedTokens(t, token, fetched.GetToken())

	// fail to update token with unreachable scope
	token.Spec.AssignedScope = "/stage/bb"
	_, err = service.UpdateScopedToken(ctx, &joiningv1.UpdateScopedTokenRequest{
		Token: token,
	})
	require.True(t, trace.IsBadParameter(err))

	// delete token
	_, err = service.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
		Name: token.Metadata.Name,
	})
	require.NoError(t, err)

	// create some tokens
	token.Spec.AssignedScope = "/test/bb"
	_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: token,
	})
	require.NoError(t, err)

	token2 := proto.CloneOf(token)
	token2.Metadata.Name = "testtoken2"
	token2.Scope = "/test/aa"
	token2.Spec.AssignedScope = "/test/aa"
	_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: token2,
	})
	require.NoError(t, err)

	token3 := proto.CloneOf(token)
	token3.Metadata.Name = "testtoken3"
	token3.Scope = "/test/bb"
	token3.Spec.AssignedScope = "/test/bb"
	_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: token3,
	})
	require.NoError(t, err)

	res, err := service.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{
		ResourceScope: &scopesv1.Filter{
			Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/test/aa",
		},
	})
	require.NoError(t, err)
	assert.Len(t, res.Tokens, 2)
	sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
		return cmp.Compare(left.Metadata.Name, right.Metadata.Name)
	}

	expected := []*joiningv1.ScopedToken{token, token2}
	slices.SortStableFunc(res.Tokens, sortFn)
	slices.SortStableFunc(expected, sortFn)
	for idx, token := range res.Tokens {
		assertEqualScopedTokens(t, expected[idx], token)
	}
}

type fakeChecker struct {
	services.AccessChecker
	role string
}

func (f *fakeChecker) HasRole(role string) bool {
	return role == f.role
}

type authKey struct{}

func withAuthCtx(ctx context.Context, authCtx authz.Context) context.Context {
	return context.WithValue(ctx, authKey{}, authCtx)
}

func newAuthCtx(role types.SystemRole) authz.Context {
	return authz.Context{
		Identity: authz.BuiltinRole{
			Role: role,
		},
		Checker: &fakeChecker{
			role: string(role),
		},
	}
}

func TestScopedJoiningServiceValidation(t *testing.T) {
	ctx := withAuthCtx(t.Context(), newAuthCtx(types.RoleAdmin))

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	svc, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	service, err := joining.New(joining.Config{
		Logger:     logtest.NewLogger(),
		Backend:    svc,
		Authorizer: fakeAuthorizer{},
	})
	require.NoError(t, err)

	cases := []struct {
		name         string
		token        *joiningv1.ScopedToken
		expectErrMsg string
	}{
		{
			name:         "reject nil token",
			expectErrMsg: "scoped token must not be nil",
		},
		{
			name: "reject scoped token without a resource scope",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
			},
			expectErrMsg: "scoped token must not be nil",
		},
		{
			name: "reject scoped token with invalid resource scope",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "test",
			},
			expectErrMsg: "scoped token must not be nil",
		},
		{
			name: "reject scoped token that assigns an invalid scope",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/test",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "test/aa",
				},
			},
			expectErrMsg: "validating scoped token assigned scope",
		},
		{
			name: "reject scoped token that assigns a scope that is not descendant of its own scope",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/test",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/stage/aa",
				},
			},
			expectErrMsg: "scoped token assigned scope must be descendant of its resource scope",
		},
		{
			name: "reject scoped token that fails to assign at least one role",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/test",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/test/aa",
				},
			},
			expectErrMsg: "at least one role must be assigned to a token",
		},
		{
			name: "accept well formed token",
			token: &joiningv1.ScopedToken{
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Scope: "/test",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/test/aa",
					Roles:         []string{"Node"},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
				Token: c.token,
			})

			if c.expectErrMsg != "" {
				require.Contains(t, err.Error(), c.expectErrMsg)
			} else {
				assertEqualScopedTokens(t, c.token, created.GetToken())
			}
		})
	}
}

type fakeAuthorizer struct{}

func (f fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, ok := ctx.Value(authKey{}).(authz.Context)
	if !ok {
		return nil, errors.New("no auth context found")
	}

	return &authCtx, nil
}
