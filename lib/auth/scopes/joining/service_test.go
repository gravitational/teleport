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
	"slices"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/scopes/joining"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedaccesscache "github.com/gravitational/teleport/lib/scopes/cache/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestScopedJoiningService(t *testing.T) {
	pack := newBackendPack(t)
	service := pack.scopedTokenService

	cases := []struct{
		name string
		accessInfo *services.AccessInfo,
	}{

	}
	newServerForIdentity(t, pack, &services.AccessInfo{
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			Assignments: map[string]*scopesv1.PinnedAssignments{
				"/staging": {
					Roles: []string{"staging-admin"},
				},
			},
		},
	})
	token := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
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

const testClusterName = "test-cluster"

// fakeSplitAuthorizer is a mock implementation of ScopedAuthorizer that provides a hard-coded context.
type fakeSplitAuthorizer struct {
	ctx *authz.ScopedContext
}

func (a *fakeSplitAuthorizer) AuthorizeScoped(ctx context.Context) (*authz.ScopedContext, error) {
	return a.ctx, nil
}

// newFakeScopedAuthorizer builds a fake split authorizer with a hard-coded context based on the provided scoped access info and reader.
// this means that while the identity/assignments can be fake, the underlying reader must contain the expected scoped
// roles in order for the context to be built successfully.
func newFakeScopedAuthorizer(t *testing.T, accessInfo *services.AccessInfo, reader services.ScopedRoleReader) *fakeSplitAuthorizer {
	t.Helper()

	scopedCtx, err := services.NewScopedAccessCheckerContext(t.Context(), accessInfo, testClusterName, reader)
	require.NoError(t, err)

	return &fakeSplitAuthorizer{
		ctx: &authz.ScopedContext{
			User: &types.UserV2{
				Metadata: types.Metadata{
					Name: accessInfo.Username,
				},
			},
			CheckerContext: services.NewScopedSplitAccessCheckerContext(scopedCtx),
		},
	}
}

// newServerForIdentity builds a server with an access checker that is hard-coded to the provided access info. The backend pack
// much be pre-seeded with the relevant scoped/unscoped roles, but assignments are drawn from the access info (as they would be
// if the access info was being taken from a certificate).
func newServerForIdentity(t *testing.T, bk *backendPack, accessInfo *services.AccessInfo) *joining.Server {
	t.Helper()

	var authz authz.ScopedAuthorizer
	if accessInfo.ScopePin != nil {
		authz = newFakeScopedAuthorizer(t, accessInfo, bk.cache)
	}

	srv, err := joining.New(joining.Config{
		ScopedAuthorizer: authz,
		Backend:          bk.scopedTokenService,
		Logger:           logtest.NewLogger(),
	})
	require.NoError(t, err)

	return srv
}

type backendPack struct {
	backend            backend.Backend
	service            *local.ScopedAccessService
	classicService     *local.AccessService
	cache              *scopedaccesscache.Cache
	scopedTokenService *local.ScopedTokenService
}

func (p *backendPack) Close() {
	p.cache.Close()
	p.backend.Close()
}

// newBackendPack creates a scoped access service and populates it with the provided scoped roles.
func newBackendPack(t *testing.T) *backendPack {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { backend.Close() })

	service := local.NewScopedAccessService(backend)
	classicService := local.NewAccessService(backend)
	events := local.NewEventsService(backend)
	scopedTokenService, err := local.NewScopedTokenService(backend)
	require.NoError(t, err)

	cache, err := scopedaccesscache.NewCache(scopedaccesscache.CacheConfig{
		Events: events,
		Reader: service,
	})
	require.NoError(t, err)

	select {
	case <-cache.Init():
	case <-time.After(30 * time.Second):
		require.FailNow(t, "timed out waiting for scoped access cache to initialize")
	}

	roles := []*scopedaccessv1.ScopedRole{
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-admin",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-create",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbCreate},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-read",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbReadNoSecrets},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-list",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbList},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-delete",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbDelete},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "prod-admin",
			},
			Scope: "/prod",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/prod"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
						},
					},
				},
			},
			Version: types.V1,
		},
	}

	for _, role := range roles {
		_, err := service.CreateScopedRole(t.Context(), role)
		require.NoError(t, err)
	}
	return &backendPack{
		backend:            backend,
		service:            service,
		classicService:     classicService,
		cache:              cache,
		scopedTokenService: scopedTokenService,
	}
}
