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
	jointoken "github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func createToken(ctx context.Context, server *joining.Server, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	res, err := server.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: proto.CloneOf(token),
	})
	if err != nil {
		return nil, err
	}

	return res.GetToken(), nil
}

func TestScopedJoiningService(t *testing.T) {
	ctx := t.Context()
	pack := newBackendPack(t)

	t.Run("basic", func(t *testing.T) {
		service := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging": {
						Roles: []string{"staging-admin"},
					},
				},
			},
		})

		baseToken := &joiningv1.ScopedToken{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Scope:    "/staging",
			Metadata: &headerv1.Metadata{},
			Spec: &joiningv1.ScopedTokenSpec{
				AssignedScope: "/staging/aa",
				JoinMethod:    "token",
				Roles:         []string{"Node"},
				UsageMode:     string(jointoken.TokenUsageModeUnlimited),
			},
		}

		// create a token
		token, err := createToken(ctx, service, baseToken)
		require.NoError(t, err)

		createOpts := []gocmp.Option{
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "name"),
			protocmp.IgnoreFields(&joiningv1.ScopedToken{}, "status"),
			protocmp.Transform(),
		}
		cmpOpts := []gocmp.Option{
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
			protocmp.Transform(),
		}
		assert.Empty(t, gocmp.Diff(baseToken, token, createOpts...))

		// fail to create a token with an assigned scope that is orthogonal to its own
		tokenWithMismatchedScope := proto.CloneOf(baseToken)
		tokenWithMismatchedScope.Metadata.Name = "invalid-token"
		tokenWithMismatchedScope.Spec.AssignedScope = "/prod/aa"
		_, err = createToken(ctx, service, tokenWithMismatchedScope)
		assert.True(t, trace.IsBadParameter(err))

		// create a token with an explicit name
		namedToken := proto.CloneOf(baseToken)
		namedToken.Metadata.Name = "named-token"
		namedToken, err = createToken(ctx, service, namedToken)
		require.NoError(t, err)
		require.NotEmpty(t, namedToken.Metadata.Name)

		// fetch a token
		fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       token.Metadata.Name,
			WithSecret: true,
		})
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(token, fetched.GetToken(), cmpOpts...))

		// delete a token
		_, err = service.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
			Name: namedToken.Metadata.Name,
		})
		require.NoError(t, err)

		// create some tokens to list
		tokenStagingBB := proto.CloneOf(baseToken)
		tokenStagingBB.Scope = "/staging/bb"
		tokenStagingBB.Spec.AssignedScope = "/staging/bb"
		_, err = createToken(ctx, service, tokenStagingBB)
		require.NoError(t, err)

		tokenStagingCC1 := proto.CloneOf(baseToken)
		tokenStagingCC1.Scope = "/staging/cc"
		tokenStagingCC1.Spec.AssignedScope = "/staging/cc"
		tokenStagingCC1, err = createToken(ctx, service, tokenStagingCC1)
		require.NoError(t, err)

		tokenStagingCC2 := proto.CloneOf(baseToken)
		tokenStagingCC2.Scope = "/staging/cc"
		tokenStagingCC2.Spec.AssignedScope = "/staging/cc"
		tokenStagingCC2, err = createToken(ctx, service, tokenStagingCC2)
		require.NoError(t, err)

		// list tokens while filtering their resource scope
		res, err := service.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{
			WithSecrets: true,
			ResourceScope: &scopesv1.Filter{
				Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
				Scope: "/staging/cc",
			},
		})
		require.NoError(t, err)
		assert.Len(t, res.Tokens, 2)
		sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
			return cmp.Compare(left.Metadata.Name, right.Metadata.Name)
		}

		expected := []*joiningv1.ScopedToken{tokenStagingCC1, tokenStagingCC2}
		slices.SortStableFunc(res.Tokens, sortFn)
		slices.SortStableFunc(expected, sortFn)
		for idx, token := range res.Tokens {
			assert.Empty(t, gocmp.Diff(expected[idx], token, cmpOpts...))
		}
	})

	t.Run("auth", func(t *testing.T) {
		admin := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging": {
						Roles: []string{"staging-admin"},
					},
				},
			},
		})

		writer := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging/aa",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging/aa": {
						Roles: []string{"staging-create"},
					},
				},
			},
		})

		reader := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging/aa",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging/aa": {
						Roles: []string{"staging-read"},
					},
				},
			},
		})

		readerNoSecrets := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging/aa",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging/aa": {
						Roles: []string{"staging-readnosecrets"},
					},
				},
			},
		})

		deleter := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging/aa",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging/aa": {
						Roles: []string{"staging-delete"},
					},
				},
			},
		})

		updater := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: &scopesv1.Pin{
				Scope: "/staging/aa",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/staging/aa": {
						Roles: []string{"staging-upserter"},
					},
				},
			},
		})

		baseToken := &joiningv1.ScopedToken{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Scope:    "/staging/aa",
			Metadata: &headerv1.Metadata{},
			Spec: &joiningv1.ScopedTokenSpec{
				AssignedScope: "/staging/aa",
				JoinMethod:    "token",
				Roles:         []string{"Node"},
				UsageMode:     string(jointoken.TokenUsageModeUnlimited),
			},
		}

		var stageTokenAA *joiningv1.ScopedToken
		// ensure writer can create a token at an accessible scope and create base token for use in subtests
		stageTokenAA, err := createToken(ctx, writer, baseToken)
		require.NoError(t, err)

		cmpOpts := []gocmp.Option{
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "name"),
			protocmp.IgnoreFields(&joiningv1.ScopedToken{}, "status"),
			protocmp.Transform(),
		}
		expectedToken := proto.CloneOf(baseToken)
		expectedToken.Status = &joiningv1.ScopedTokenStatus{}
		assert.Empty(t, gocmp.Diff(expectedToken, stageTokenAA, cmpOpts...))

		stageTokenBB := proto.CloneOf(baseToken)
		stageTokenBB.Scope = "/staging/bb"
		stageTokenBB.Spec.AssignedScope = "/staging/bb"
		// create an orthogonal token for negative testing read ops
		stageTokenBB, err = createToken(ctx, admin, stageTokenBB)
		require.NoError(t, err)

		t.Run("ensure writer can't create a token at an orthogonal scope", func(t *testing.T) {
			t.Parallel()
			_, err := createToken(ctx, writer, stageTokenBB)
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure other identities can't create a token", func(t *testing.T) {
			t.Parallel()
			nonWriterIdents := []*joining.Server{reader, deleter}
			for _, ident := range nonWriterIdents {
				_, err := createToken(ctx, ident, baseToken)
				require.True(t, trace.IsAccessDenied(err))
			}
		})

		t.Run("user with readnosecret role cannot read secret at accessible scope", func(t *testing.T) {
			t.Parallel()
			getRes, err := readerNoSecrets.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
				Name: stageTokenAA.Metadata.Name,
			})
			require.NoError(t, err)
			assert.Empty(t, getRes.GetToken().GetStatus().GetSecret())

			_, err = readerNoSecrets.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
				Name:       stageTokenAA.Metadata.Name,
				WithSecret: true,
			})
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure reader can get token at accessible scope with secrets", func(t *testing.T) {
			t.Parallel()
			getRes, err := reader.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
				Name:       stageTokenAA.Metadata.Name,
				WithSecret: true,
			})
			require.NoError(t, err)
			require.NotEmpty(t, getRes.GetToken().GetStatus().GetSecret())
			require.Equal(t, stageTokenAA.GetStatus().GetSecret(), getRes.GetToken().GetStatus().GetSecret())
			require.Empty(t, gocmp.Diff(expectedToken, getRes.GetToken(), cmpOpts...))
		})

		t.Run("reader cannot get token at orthogonal scope", func(t *testing.T) {
			t.Parallel()
			_, err := reader.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
				Name: stageTokenBB.Metadata.Name,
			})
			require.True(t, trace.IsAccessDenied(err))
		})

		nonReaderIdents := []*joining.Server{writer, deleter}
		t.Run("ensure other identities can't read a token", func(t *testing.T) {
			t.Parallel()
			for _, ident := range nonReaderIdents {
				_, err := ident.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
					Name: stageTokenAA.Metadata.Name,
				})
				require.True(t, trace.IsAccessDenied(err))
			}
		})

		t.Run("ensure reader can list only accessible tokens", func(t *testing.T) {
			t.Parallel()
			listRes, err := reader.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{})
			require.NoError(t, err)
			require.NotEmpty(t, listRes.GetTokens())
			for _, token := range listRes.GetTokens() {
				require.Equal(t, "/staging/aa", token.GetScope(), "reader should only see tokens at their accessible scope")
			}
		})

		t.Run("ensure other identities can't list tokens", func(t *testing.T) {
			t.Parallel()
			for _, ident := range nonReaderIdents {
				listRes, err := ident.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{})
				require.NoError(t, err)
				require.Empty(t, listRes.GetTokens())
			}
		})

		nonDeleterIdents := []*joining.Server{reader, writer}
		t.Run("ensure other identities can't delete a token", func(t *testing.T) {
			t.Parallel()
			for _, ident := range nonDeleterIdents {
				_, err := ident.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
					Name: stageTokenAA.Metadata.Name,
				})
				require.True(t, trace.IsAccessDenied(err))
			}
		})

		t.Run("ensure deleter can delete a token", func(t *testing.T) {
			t.Parallel()
			tokenForDelete, err := createToken(ctx, admin, baseToken)
			require.NoError(t, err)
			_, err = deleter.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
				Name: tokenForDelete.Metadata.Name,
			})
			require.NoError(t, err)
		})

		t.Run("ensure deleter can't delete a token at an orthogonal scope", func(t *testing.T) {
			t.Parallel()
			_, err := deleter.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
				Name: stageTokenBB.Metadata.Name,
			})
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure upserter can update a token at accessible scope", func(t *testing.T) {
			t.Parallel()
			tokenForUpsert, err := createToken(ctx, admin, baseToken)
			require.NoError(t, err)

			tokenUpdate := proto.CloneOf(tokenForUpsert)
			tokenUpdate.Metadata = proto.CloneOf(tokenForUpsert.GetMetadata())
			tokenUpdate.Metadata.Labels = map[string]string{"env": "test"}

			_, err = updater.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{
				Token: tokenUpdate,
			})
			require.NoError(t, err)

			t.Run("non upserter role cannot update a token", func(t *testing.T) {
				t.Parallel()
				nonUpdaterIdents := []*joining.Server{reader, readerNoSecrets, writer}
				for _, ident := range nonUpdaterIdents {
					_, err := ident.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{
						Token: tokenUpdate,
					})
					require.True(t, trace.IsAccessDenied(err))
				}
			})
		})

		t.Run("ensure upserter cannot upsert a token at an orthogonal scope", func(t *testing.T) {
			t.Parallel()
			tokenUpdate := proto.CloneOf(stageTokenBB)
			tokenUpdate.Metadata = proto.CloneOf(stageTokenBB.GetMetadata())
			tokenUpdate.Metadata.Labels = map[string]string{"env": "test"}

			_, err := updater.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{
				Token: tokenUpdate,
			})
			require.True(t, trace.IsAccessDenied(err))
		})
	})
}

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

	scopedCtx, err := services.NewScopedAccessCheckerContext(t.Context(), accessInfo, "test-cluster", reader)
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

	authz := newFakeScopedAuthorizer(t, accessInfo, bk.service)
	require.NotNil(t, accessInfo.ScopePin)

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
	scopedTokenService *local.ScopedTokenService
}

func (p *backendPack) Close() {
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
	scopedTokenService, err := local.NewScopedTokenService(backend)
	require.NoError(t, err)

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
							Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbList, types.VerbDelete, types.VerbUpdate},
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
				AssignableScopes: []string{"/staging/aa"},
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
				AssignableScopes: []string{"/staging/aa"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbRead, types.VerbList},
						},
					},
				},
			},
			Version: types.V1,
		}, {
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-readnosecrets",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging/aa"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
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
				AssignableScopes: []string{"/staging/aa"},
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
				Name: "staging-upserter",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging/aa"},
				Allow: &scopedaccessv1.ScopedRoleConditions{
					Rules: []*scopedaccessv1.ScopedRule{
						{
							Resources: []string{types.KindScopedToken},
							Verbs:     []string{types.VerbUpdate, types.VerbCreate},
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
							Verbs:     []string{types.VerbRead, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
						},
					},
				},
			},
			Version: types.V1,
		},
	}

	for _, role := range roles {
		_, err := service.CreateScopedRole(t.Context(), &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	return &backendPack{
		backend:            backend,
		service:            service,
		classicService:     classicService,
		scopedTokenService: scopedTokenService,
	}
}
