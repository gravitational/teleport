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
	"github.com/google/uuid"
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
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func createToken(ctx context.Context, server *joining.Server, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	cloned := proto.CloneOf(token)
	cloned.SetMetadata(headerv1.Metadata_builder{
		Name: uuid.New().String(),
	}.Build())
	res, err := server.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: cloned,
	}.Build())
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
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging": {"/staging": {"staging-admin"}},
				}),
			}.Build(),
		})

		baseToken := joiningv1.ScopedToken_builder{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Scope:    "/staging",
			Metadata: &headerv1.Metadata{},
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: "/staging/aa",
				JoinMethod:    "token",
				Roles:         []string{"Node"},
				UsageMode:     string(jointoken.TokenUsageModeUnlimited),
			}.Build(),
		}.Build()

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
		tokenWithMismatchedScope.GetMetadata().SetName("invalid-token")
		tokenWithMismatchedScope.GetSpec().SetAssignedScope("/prod/aa")
		_, err = createToken(ctx, service, tokenWithMismatchedScope)
		assert.True(t, trace.IsBadParameter(err))

		// create a token with an explicit name
		namedToken := proto.CloneOf(baseToken)
		namedToken.GetMetadata().SetName("named-token")
		namedToken, err = createToken(ctx, service, namedToken)
		require.NoError(t, err)
		require.NotEmpty(t, namedToken.GetMetadata().GetName())

		// fetch a token
		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(token, fetched.GetToken(), cmpOpts...))

		// delete a token
		_, err = service.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
			Name: namedToken.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)

		// create some tokens to list
		tokenStagingBB := proto.CloneOf(baseToken)
		tokenStagingBB.SetScope("/staging/bb")
		tokenStagingBB.GetSpec().SetAssignedScope("/staging/bb")
		_, err = createToken(ctx, service, tokenStagingBB)
		require.NoError(t, err)

		tokenStagingCC1 := proto.CloneOf(baseToken)
		tokenStagingCC1.SetScope("/staging/cc")
		tokenStagingCC1.GetSpec().SetAssignedScope("/staging/cc")
		tokenStagingCC1, err = createToken(ctx, service, tokenStagingCC1)
		require.NoError(t, err)

		tokenStagingCC2 := proto.CloneOf(baseToken)
		tokenStagingCC2.SetScope("/staging/cc")
		tokenStagingCC2.GetSpec().SetAssignedScope("/staging/cc")
		tokenStagingCC2, err = createToken(ctx, service, tokenStagingCC2)
		require.NoError(t, err)

		// list tokens while filtering their resource scope
		res, err := service.ListScopedTokens(ctx, joiningv1.ListScopedTokensRequest_builder{
			WithSecrets: true,
			ResourceScope: scopesv1.Filter_builder{
				Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
				Scope: "/staging/cc",
			}.Build(),
		}.Build())
		require.NoError(t, err)
		assert.Len(t, res.GetTokens(), 2)
		sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
			return cmp.Compare(left.GetMetadata().GetName(), right.GetMetadata().GetName())
		}

		expected := []*joiningv1.ScopedToken{tokenStagingCC1, tokenStagingCC2}
		slices.SortStableFunc(res.GetTokens(), sortFn)
		slices.SortStableFunc(expected, sortFn)
		for idx, token := range res.GetTokens() {
			assert.Empty(t, gocmp.Diff(expected[idx], token, cmpOpts...))
		}
	})

	t.Run("auth", func(t *testing.T) {
		admin := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging": {"/staging": {"staging-admin"}},
				}),
			}.Build(),
		})

		writer := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging/aa",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging/aa": {"/staging/aa": {"staging-create"}},
				}),
			}.Build(),
		})

		reader := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging/aa",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging/aa": {"/staging/aa": {"staging-read"}},
				}),
			}.Build(),
		})

		readerNoSecrets := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging/aa",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging/aa": {"/staging/aa": {"staging-readnosecrets"}},
				}),
			}.Build(),
		})

		deleter := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging/aa",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging/aa": {"/staging/aa": {"staging-delete"}},
				}),
			}.Build(),
		})

		updater := newServerForIdentity(t, pack, &services.AccessInfo{
			ScopePin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/staging/aa",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging/aa": {"/staging/aa": {"staging-upserter"}},
				}),
			}.Build(),
		})

		baseToken := joiningv1.ScopedToken_builder{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Scope:    "/staging/aa",
			Metadata: &headerv1.Metadata{},
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: "/staging/aa",
				JoinMethod:    "token",
				Roles:         []string{"Node"},
				UsageMode:     string(jointoken.TokenUsageModeUnlimited),
			}.Build(),
		}.Build()

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
		expectedToken.SetStatus(&joiningv1.ScopedTokenStatus{})
		assert.Empty(t, gocmp.Diff(expectedToken, stageTokenAA, cmpOpts...))

		stageTokenBB := proto.CloneOf(baseToken)
		stageTokenBB.SetScope("/staging/bb")
		stageTokenBB.GetSpec().SetAssignedScope("/staging/bb")
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
			getRes, err := readerNoSecrets.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
				Name: stageTokenAA.GetMetadata().GetName(),
			}.Build())
			require.NoError(t, err)
			assert.Empty(t, getRes.GetToken().GetStatus().GetSecret())

			_, err = readerNoSecrets.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
				Name:       stageTokenAA.GetMetadata().GetName(),
				WithSecret: true,
			}.Build())
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure reader can get token at accessible scope with secrets", func(t *testing.T) {
			t.Parallel()
			getRes, err := reader.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
				Name:       stageTokenAA.GetMetadata().GetName(),
				WithSecret: true,
			}.Build())
			require.NoError(t, err)
			require.NotEmpty(t, getRes.GetToken().GetStatus().GetSecret())
			require.Equal(t, stageTokenAA.GetStatus().GetSecret(), getRes.GetToken().GetStatus().GetSecret())
			require.Empty(t, gocmp.Diff(expectedToken, getRes.GetToken(), cmpOpts...))
		})

		t.Run("reader cannot get token at orthogonal scope", func(t *testing.T) {
			t.Parallel()
			_, err := reader.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
				Name: stageTokenBB.GetMetadata().GetName(),
			}.Build())
			require.True(t, trace.IsAccessDenied(err))
		})

		nonReaderIdents := []*joining.Server{writer, deleter}
		t.Run("ensure other identities can't read a token", func(t *testing.T) {
			t.Parallel()
			for _, ident := range nonReaderIdents {
				_, err := ident.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
					Name: stageTokenAA.GetMetadata().GetName(),
				}.Build())
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
				_, err := ident.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
					Name: stageTokenAA.GetMetadata().GetName(),
				}.Build())
				require.True(t, trace.IsAccessDenied(err))
			}
		})

		t.Run("ensure deleter can delete a token", func(t *testing.T) {
			t.Parallel()
			tokenForDelete, err := createToken(ctx, admin, baseToken)
			require.NoError(t, err)
			_, err = deleter.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
				Name: tokenForDelete.GetMetadata().GetName(),
			}.Build())
			require.NoError(t, err)
		})

		t.Run("ensure deleter can't delete a token at an orthogonal scope", func(t *testing.T) {
			t.Parallel()
			_, err := deleter.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
				Name: stageTokenBB.GetMetadata().GetName(),
			}.Build())
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure upserter can update a token at accessible scope", func(t *testing.T) {
			t.Parallel()
			tokenForUpsert, err := createToken(ctx, admin, baseToken)
			require.NoError(t, err)

			tokenUpdate := proto.CloneOf(tokenForUpsert)
			tokenUpdate.SetMetadata(proto.CloneOf(tokenForUpsert.GetMetadata()))
			tokenUpdate.GetMetadata().SetLabels(map[string]string{"env": "test"})

			_, err = updater.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{
				Token: tokenUpdate,
			}.Build())
			require.NoError(t, err)

			t.Run("non upserter role cannot update a token", func(t *testing.T) {
				t.Parallel()
				nonUpdaterIdents := []*joining.Server{reader, readerNoSecrets, writer}
				for _, ident := range nonUpdaterIdents {
					_, err := ident.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{
						Token: tokenUpdate,
					}.Build())
					require.True(t, trace.IsAccessDenied(err))
				}
			})
		})

		t.Run("ensure upserter cannot upsert a token at an orthogonal scope", func(t *testing.T) {
			t.Parallel()
			tokenUpdate := proto.CloneOf(stageTokenBB)
			tokenUpdate.SetMetadata(proto.CloneOf(stageTokenBB.GetMetadata()))
			tokenUpdate.GetMetadata().SetLabels(map[string]string{"env": "test"})

			_, err := updater.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{
				Token: tokenUpdate,
			}.Build())
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure updater can update a token at accessible scope", func(t *testing.T) {
			tokenForUpdate, err := createToken(ctx, admin, baseToken)
			require.NoError(t, err)

			tokenUpdate := proto.CloneOf(tokenForUpdate)
			tokenUpdate.GetMetadata().SetLabels(map[string]string{"env": "updated"})

			_, err = updater.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{
				Token: tokenUpdate,
			}.Build())
			require.NoError(t, err)

			// Update should fail after updating if revisions don't match.
			staleUpdate := proto.CloneOf(tokenForUpdate)
			staleUpdate.GetMetadata().SetLabels(map[string]string{"bad": "update"})
			_, err = updater.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{
				Token: staleUpdate,
			}.Build())
			require.True(t, trace.IsCompareFailed(err))

			t.Run("non updater role cannot update a token", func(t *testing.T) {
				nonUpdaterIdents := []*joining.Server{reader, readerNoSecrets, writer}
				for _, ident := range nonUpdaterIdents {
					_, err := ident.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{
						Token: tokenUpdate,
					}.Build())
					require.True(t, trace.IsAccessDenied(err))
				}
			})
		})

		t.Run("ensure updater cannot update a token at an orthogonal scope", func(t *testing.T) {
			tokenUpdate := proto.CloneOf(stageTokenBB)
			tokenUpdate.GetMetadata().SetLabels(map[string]string{"env": "test"})

			_, err := updater.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{
				Token: tokenUpdate,
			}.Build())
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("ensure updater cannot bypass scope auth by spoofing scope in request", func(t *testing.T) {
			tokenUpdate := proto.CloneOf(stageTokenBB)
			tokenUpdate.SetScope("/staging/aa")
			tokenUpdate.GetSpec().SetAssignedScope("/staging/aa")

			_, err := updater.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{
				Token: tokenUpdate,
			}.Build())
			require.True(t, trace.IsBadParameter(err))
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
			CheckerContext: scopedCtx,
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
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-admin",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbList, types.VerbDelete, types.VerbUpdate},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-create",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging/aa"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbCreate},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-read",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging/aa"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbRead, types.VerbList},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-readnosecrets",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging/aa"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-delete",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging/aa"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbDelete},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "staging-upserter",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/staging/aa"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbUpdate, types.VerbCreate},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(), scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "prod-admin",
			}.Build(),
			Scope: "/prod",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/prod"},
				Rules: []*scopedaccessv1.ScopedRule{
					scopedaccessv1.ScopedRule_builder{
						Resources: []string{types.KindScopedToken},
						Verbs:     []string{types.VerbRead, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	for _, role := range roles {
		_, err := service.CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: role,
		}.Build())
		require.NoError(t, err)
	}

	return &backendPack{
		backend:            backend,
		service:            service,
		classicService:     classicService,
		scopedTokenService: scopedTokenService,
	}
}
