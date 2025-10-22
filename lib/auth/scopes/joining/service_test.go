// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package joining_test

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/defaults"
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
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      "testtoken",
			Namespace: defaults.Namespace,
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
	cmpOpts := []gocmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}
	assert.Empty(t, gocmp.Diff(token, created.GetToken(), cmpOpts...))

	tokenWithMismatchedScope := proto.CloneOf(token)
	tokenWithMismatchedScope.Metadata.Name = "invalid-token"
	tokenWithMismatchedScope.Spec.AssignedScope = "/stage/aa"

	// make sure update is no-op
	_, err = service.UpdateScopedToken(ctx, &joiningv1.UpdateScopedTokenRequest{})
	require.True(t, trace.IsNotImplemented(err))

	_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: tokenWithMismatchedScope,
	})
	assert.True(t, trace.IsBadParameter(err))

	tokenWithoutName := proto.CloneOf(token)
	tokenWithoutName.Metadata.Name = ""
	createdWithoutName, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: tokenWithoutName,
	})
	require.NoError(t, err)
	require.NotEmpty(t, createdWithoutName.Token.GetMetadata().GetName())

	// get token
	fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name: token.Metadata.Name,
	})
	require.NoError(t, err)
	assert.Empty(t, gocmp.Diff(token, fetched.GetToken(), cmpOpts...))

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
	assert.Len(t, res.Tokens, 3)
	sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
		return cmp.Compare(left.Metadata.Name, right.Metadata.Name)
	}

	expected := []*joiningv1.ScopedToken{token, tokenWithoutName, token2}
	slices.SortStableFunc(res.Tokens, sortFn)
	slices.SortStableFunc(expected, sortFn)
	for idx, token := range res.Tokens {
		assert.Empty(t, gocmp.Diff(expected[idx], token, cmpOpts...))
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

type fakeAuthorizer struct{}

func (f fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, ok := ctx.Value(authKey{}).(authz.Context)
	if !ok {
		return nil, errors.New("no auth context found")
	}

	return &authCtx, nil
}
