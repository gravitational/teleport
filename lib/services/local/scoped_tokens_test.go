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

package local_test

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestScopedTokenService(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(bk)
	require.NoError(t, err)

	ctx := t.Context()

	token := joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "testtoken",
		}.Build(),
		Scope: "/test",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "/test/one",
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
			JoinMethod:    string(types.JoinMethodToken),
		}.Build(),
	}.Build()

	created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: token,
	}.Build())
	require.NoError(t, err)
	cmpOpts := []gocmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}
	assert.Empty(t, gocmp.Diff(token, created.GetToken(), cmpOpts...))
	// ensure the token secret was set if not specified
	assert.NotEmpty(t, created.GetToken().GetStatus().GetSecret())
	token = created.GetToken()

	updatedToken := proto.CloneOf(token)
	updatedToken.GetSpec().SetAssignedScope("/test/test")

	updated, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{
		Token: created.GetToken(),
	}.Build())
	require.NoError(t, err)

	fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name: token.GetMetadata().GetName(),
	}.Build())
	require.NoError(t, err)
	require.Equal(t, fetched.GetToken().GetSpec().GetAssignedScope(), updated.GetToken().GetSpec().GetAssignedScope())
	require.Empty(t, fetched.GetToken().GetStatus().GetSecret())

	list, err := service.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetTokens(), 1)

	fetched, err = service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name:       token.GetMetadata().GetName(),
		WithSecret: true,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, fetched.GetToken().GetStatus().GetSecret())
	assert.Empty(t, gocmp.Diff(updated.GetToken(), fetched.GetToken(), cmpOpts...))

	_, err = service.DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
		Name: fetched.GetToken().GetMetadata().GetName(),
	}.Build())
	require.NoError(t, err)
	_, err = service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name: fetched.GetToken().GetMetadata().GetName(),
	}.Build())
	require.True(t, trace.IsNotFound(err))

	expiredToken := proto.CloneOf(token)
	expiredToken.GetMetadata().SetName("expiredtoken")
	expiredToken.GetMetadata().SetExpires(timestamppb.New(time.Now().UTC().Add(-25 * time.Hour)))

	activeToken := proto.CloneOf(token)
	activeToken.GetMetadata().SetName("activetoken")
	activeToken.GetMetadata().SetExpires(timestamppb.New(time.Now().UTC().Add(25 * time.Hour)))

	expiredRes, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: expiredToken,
	}.Build())
	require.NoError(t, err)

	activeRes, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: activeToken,
	}.Build())
	require.NoError(t, err)

	// expired tokens should error and delete the token
	expiredToken, err = service.UseScopedToken(ctx, expiredRes.GetToken(), nil)
	// If for some reason the expired token is not automatically deleted by the backend, a
	// LimitExceededError will be returned. Otherwise, we should expect the token not to be found.
	require.True(t, trace.IsLimitExceeded(err) || trace.IsNotFound(err))
	require.Nil(t, expiredToken)

	_, err = service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name: expiredRes.GetToken().GetMetadata().GetName(),
	}.Build())
	require.True(t, trace.IsNotFound(err))

	fetchedActive, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
		Name:       activeRes.GetToken().GetMetadata().GetName(),
		WithSecret: true,
	}.Build())

	require.NoError(t, err)
	assert.Empty(t, gocmp.Diff(activeRes.GetToken(), fetchedActive.GetToken(), cmpOpts...))
}

func TestScopedTokenList(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(bk)
	require.NoError(t, err)

	ctx := t.Context()

	test := joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "test",
		}.Build(),
		Scope: "/test",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "/test",
			JoinMethod:    "token",
			Roles: []string{
				types.RoleNode.String(),
			},
			UsageMode: string(joining.TokenUsageModeUnlimited),
		}.Build(),
		Status: joiningv1.ScopedTokenStatus_builder{
			Secret: "secret",
		}.Build(),
	}.Build()

	test1 := proto.CloneOf(test)
	test1.GetMetadata().SetName("test1")
	test1.SetScope("/test/aa")
	test1.GetSpec().SetAssignedScope(test1.GetScope())

	test2 := proto.CloneOf(test)
	test2.GetMetadata().SetName("test2")
	test2.SetScope("/test/bb")
	test2.GetSpec().SetAssignedScope(test2.GetScope())
	test2.GetMetadata().SetLabels(map[string]string{
		"hello": "world",
	})

	test3 := proto.CloneOf(test)
	test3.GetMetadata().SetName("test3")
	test3.SetScope("/test/aa/bb")
	test3.GetSpec().SetAssignedScope(test3.GetScope())

	test4 := proto.CloneOf(test)
	test4.GetMetadata().SetName("test4")
	test4.GetSpec().SetAssignedScope("/test/aa")
	test4.SetScope("/test/aa")
	test4.GetSpec().SetAssignedScope(test4.GetScope())

	stage := proto.CloneOf(test)
	stage.GetMetadata().SetName("stage")
	stage.SetScope("/stage")
	stage.GetSpec().SetAssignedScope(stage.GetScope())

	stage1 := proto.CloneOf(stage)
	stage1.GetMetadata().SetName("stage1")
	stage1.GetSpec().SetAssignedScope("/stage/aa")

	stage2 := proto.CloneOf(stage)
	stage2.GetMetadata().SetName("stage2")
	stage2.SetScope("/stage/aa")
	stage2.GetSpec().SetAssignedScope("/stage/aa")

	allTokens := []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2}
	for _, token := range allTokens {
		_, err = service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)
	}

	sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
		return cmp.Compare(left.GetMetadata().GetName(), right.GetMetadata().GetName())
	}
	cases := []struct {
		name     string
		req      *joiningv1.ListScopedTokensRequest
		expected []*joiningv1.ScopedToken
	}{
		{
			name: "all tokens (no filters)",
			req: joiningv1.ListScopedTokensRequest_builder{
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2},
		},
		{
			name: "tokens assigning scope descendant of /test",
			req: joiningv1.ListScopedTokensRequest_builder{
				AssignedScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens assigning scope descendant of /test/aa",
			req: joiningv1.ListScopedTokensRequest_builder{
				AssignedScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens assigning scope ancestor to /test/bb",
			req: joiningv1.ListScopedTokensRequest_builder{
				AssignedScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendants of /test",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens descendants of /test/aa",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens ancestor to /test/bb",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendant of /stage assigning /stage/aa",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage",
				}.Build(),
				AssignedScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{stage1, stage2},
		},
		{
			name: "tokens descendant of /stage/aa assigning /stage/aa",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				}.Build(),
				AssignedScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				}.Build(),
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{stage2},
		},
		{
			name: "tokens in /test scope applying auth role",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				}.Build(),
				Roles:       []string{types.RoleAuth.String()},
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{},
		},
		{
			name: "tokens in /test scope filtered by label",
			req: joiningv1.ListScopedTokensRequest_builder{
				ResourceScope: scopesv1.Filter_builder{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				}.Build(),
				Roles: []string{types.RoleNode.String()},
				Labels: map[string]string{
					"hello": "world",
				},
				WithSecrets: true,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test2},
		},
		{
			name: "tokens have no secrets when WithSecrets is false",
			req: joiningv1.ListScopedTokensRequest_builder{
				WithSecrets: false,
			}.Build(),
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := proto.CloneOf(c.req)
			req.SetLimit(10)
			res, err := service.ListScopedTokens(ctx, req)
			require.NoError(t, err)

			slices.SortStableFunc(c.expected, sortFn)
			slices.SortStableFunc(res.GetTokens(), sortFn)
			require.Len(t, res.GetTokens(), len(c.expected))
			cmpOpts := []gocmp.Option{
				protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
				protocmp.Transform(),
			}
			if !req.GetWithSecrets() {
				cmpOpts = append(cmpOpts, protocmp.IgnoreFields(&joiningv1.ScopedTokenStatus{}, "secret"))
			}
			for i, token := range res.GetTokens() {
				if !req.GetWithSecrets() {
					assert.Empty(t, token.GetStatus().GetSecret())
				}
				assert.Empty(t, gocmp.Diff(c.expected[i], token, cmpOpts...))
			}
		})
	}
}

func TestScopedTokenNameCollisions(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(bk)
	require.NoError(t, err)

	provisioningService := local.NewProvisioningService(bk)

	ctx := t.Context()

	token := joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "testtoken",
		}.Build(),
		Scope: "/test",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "/test/one",
			JoinMethod:    "token",
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		}.Build(),
		Status: joiningv1.ScopedTokenStatus_builder{
			Secret: "secret",
		}.Build(),
	}.Build()

	t.Run("basic", func(t *testing.T) {
		// create initial scoped token
		_, err = service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
			Token: token,
		}.Build())
		require.NoError(t, err)

		// assert that creating another scoped token with the same name fails
		_, err = service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
			Token: token,
		}.Build())
		require.True(t, trace.IsAlreadyExists(err))

		// create a 'classic' token
		classicToken := &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: "testtoken2",
			},
			Spec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{
					types.RoleAdmin,
				},
			},
		}
		err = provisioningService.CreateToken(ctx, classicToken)
		require.NoError(t, err)

		// assert that creating a scoped token with a name that conflicts with
		// a classic token fails
		conflictWithClassic := proto.CloneOf(token)
		conflictWithClassic.GetMetadata().SetName(classicToken.GetName())
		_, err = service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
			Token: conflictWithClassic,
		}.Build())
		require.True(t, trace.IsAlreadyExists(err))
	})

	t.Run("concurrent", func(t *testing.T) {
		for i := range 50 {
			name := fmt.Sprintf("testtoken-%d", i)
			var classicErr error
			var scopedErr error
			wg := sync.WaitGroup{}
			wg.Go(func() {
				classicErr = provisioningService.CreateToken(ctx, &types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: name,
					},
					Spec: types.ProvisionTokenSpecV2{
						Roles: []types.SystemRole{
							types.RoleAdmin,
						},
					},
				})
			})

			wg.Go(func() {
				token := proto.CloneOf(token)
				token.GetMetadata().SetName(name)
				_, scopedErr = service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
					Token: token,
				}.Build())
			})
			wg.Wait()

			// One token type should always succeed and the other should always fail. When one succeeds,
			// the other should always be a [trace.AlreadyExistsError] due to the name conflict.
			if classicErr == nil {
				require.True(t, trace.IsAlreadyExists(scopedErr))
			} else if scopedErr == nil {
				require.True(t, trace.IsAlreadyExists(classicErr))
			} else {
				require.Fail(t, "unexpected failure to create either a scoped or classic token", name)
			}
		}
	})
}

func TestScopedTokenUse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bk, err := memory.New(memory.Config{})
		require.NoError(t, err)
		service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
		require.NoError(t, err)

		ctx := t.Context()

		token := joiningv1.ScopedToken_builder{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name:    "testtoken",
				Expires: timestamppb.New(time.Now().Add(24 * time.Hour)),
			}.Build(),
			Scope: "/test",
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: "/test/one",
				JoinMethod:    "token",
				Roles:         []string{types.RoleNode.String()},
				UsageMode:     string(joining.TokenUsageModeSingle),
			}.Build(),
			Status: joiningv1.ScopedTokenStatus_builder{
				Secret: "secret",
			}.Build(),
		}.Build()

		created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
			Token: token,
		}.Build())
		require.NoError(t, err)

		testKey := []byte("test")
		otherKey := []byte("other")
		fp := sha256.Sum256(testKey)
		reuseDuration := 30 * time.Minute

		now := time.Now().UTC()
		// first usage should always succeed
		tok, err := service.UseScopedToken(ctx, created.GetToken(), testKey)
		require.NoError(t, err)
		usage := tok.GetStatus().GetUsage().GetSingleUse()
		require.NotNil(t, now.Add(reuseDuration), usage.GetReusableUntil().AsTime())
		require.Equal(t, now, usage.GetUsedAt().AsTime())
		require.Equal(t, fp[:], usage.GetUsedByFingerprint())

		// reusing the same token should succeed with the same public key when
		// reusuable_until is still in the future
		tok, err = service.UseScopedToken(ctx, tok, testKey)
		require.NoError(t, err)
		usage = tok.GetStatus().GetUsage().GetSingleUse()
		require.Equal(t, now.Add(reuseDuration), usage.GetReusableUntil().AsTime())
		require.Equal(t, now, usage.GetUsedAt().AsTime())
		require.Equal(t, fp[:], usage.GetUsedByFingerprint())

		// reusing with a different key should fail
		_, err = service.UseScopedToken(ctx, tok, otherKey)
		require.ErrorIs(t, err, joining.ErrTokenExhausted)

		// reusing with the same key after reusuble_until has elapsed should fail
		<-time.After(reuseDuration + time.Minute)
		_, err = service.UseScopedToken(ctx, tok, testKey)
		require.ErrorIs(t, err, joining.ErrTokenExhausted)
	})
}

func newToken() *joiningv1.ScopedToken {
	return joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/test",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "/test/one",
			JoinMethod:    "token",
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		}.Build(),
		Status: joiningv1.ScopedTokenStatus_builder{
			Secret: "secret",
		}.Build(),
	}.Build()
}

func newBoundKeypairToken() *joiningv1.ScopedToken {
	return joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/test",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "",
			BotName:       "example",
			BotScope:      "/test",
			JoinMethod:    string(types.JoinMethodBoundKeypair),
			Roles:         []string{types.RoleBot.String()},
			UsageMode:     string(joining.TokenUsageModeBot),
			BoundKeypair:  &joiningv1.BoundKeypairSpec{},
		}.Build(),
	}.Build()
}

func TestScopedTokenUpdate(t *testing.T) {
	t.Parallel()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	cases := []struct {
		name   string
		mutate func(t *testing.T, update *joiningv1.ScopedToken)
		assert func(t *testing.T, created, result *joiningv1.ScopedToken, err error)
	}{
		{
			name: "mutable fields are updated",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.GetMetadata().SetLabels(map[string]string{"env": "test"})
				update.GetSpec().SetAssignedScope("/test/one/two")
			},
			assert: func(t *testing.T, created, result *joiningv1.ScopedToken, err error) {
				require.Equal(t, "/test/one/two", result.GetSpec().GetAssignedScope())
				require.Equal(t, "test", result.GetMetadata().GetLabels()["env"])
			},
		},
		{
			name: "scope changes result in an error",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.SetScope("/other")
			},
			assert: func(t *testing.T, created, result *joiningv1.ScopedToken, err error) {
				require.ErrorContains(t, err, "cannot modify scope of existing scoped token")

			},
		},
		{
			name: "usage mode changes result in an error",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.GetSpec().SetUsageMode(string(joining.TokenUsageModeSingle))
			},
			assert: func(t *testing.T, created, result *joiningv1.ScopedToken, err error) {
				require.ErrorContains(t, err, "cannot modify usage mode of existing scoped token")
			},
		},
		{
			name: "secret change is not allowed",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.SetStatus(joiningv1.ScopedTokenStatus_builder{Secret: "new-secret"}.Build())
			},
			assert: func(t *testing.T, created, result *joiningv1.ScopedToken, err error) {
				require.ErrorContains(t, err, "cannot modify secret of existing scoped token")
			},
		},
		{
			name: "assigned scope changed to a non-descendant scope fails",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.GetSpec().SetAssignedScope("/notadescendant")
			},
			assert: func(t *testing.T, created, result *joiningv1.ScopedToken, err error) {
				require.ErrorContains(t, err, "scoped token assigned scope must be descendant of or equivalent to the token's resource scope")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := newToken()
			created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: token}.Build())
			require.NoError(t, err)

			updated := proto.CloneOf(created.GetToken())
			tc.mutate(t, updated)

			res, err := service.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{Token: updated}.Build())

			tc.assert(t, created.GetToken(), res.GetToken(), err)
		})
	}

	t.Run("fails for nonexistent token", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{Token: token}.Build())
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("editing concurrently fails since revision doesn't match", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		updated := proto.CloneOf(created.GetToken())
		updated.GetMetadata().SetLabels(map[string]string{"env": "test"})
		updated.GetSpec().SetAssignedScope("/test/one/two")

		updated2 := proto.CloneOf(updated)
		updated2.GetMetadata().SetLabels(map[string]string{"env": "production"})

		updateRes1, err := service.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{Token: updated}.Build())
		require.NoError(t, err)
		require.Equal(t, "test", updateRes1.GetToken().GetMetadata().GetLabels()["env"])

		_, err = service.UpdateScopedToken(ctx, joiningv1.UpdateScopedTokenRequest_builder{Token: updated2}.Build())
		require.True(t, trace.IsCompareFailed(err))
	})
}

func TestScopedTokenUpsert(t *testing.T) {
	t.Parallel()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	cmpOpts := []gocmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	t.Run("upsert creates a new entry", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		upsertedToken, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       upsertedToken.GetToken().GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(upsertedToken.GetToken(), fetched.GetToken(), cmpOpts...))
	})

	t.Run("upsert creates a new secret when creating a new token with no secret set", func(t *testing.T) {
		t.Parallel()
		// Submit a minimal token with no name, join method, or secret — the
		// create path of UpsertScopedToken will fill in missing information
		token := joiningv1.ScopedToken_builder{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Scope:   "/test",
			Metadata: headerv1.Metadata_builder{
				Name: uuid.New().String(),
			}.Build(),
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: "/test/one",
				Roles:         []string{types.RoleNode.String()},
				UsageMode:     string(joining.TokenUsageModeUnlimited),
				JoinMethod:    string(types.JoinMethodToken),
			}.Build(),
		}.Build()

		upserted, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, upserted.GetToken().GetMetadata().GetName(), "name should be generated")
		require.Equal(t, string(types.JoinMethodToken), upserted.GetToken().GetSpec().GetJoinMethod(), "join method should default to token")
		require.NotEmpty(t, upserted.GetToken().GetStatus().GetSecret(), "secret should be generated")
	})

	t.Run("upsert updates existing entry", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		updated := proto.CloneOf(token)
		updated.GetMetadata().SetLabels(map[string]string{"env": "test"})
		updated.GetSpec().SetAssignedScope("/test/one/two")

		updatedToken, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: updated}.Build())
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(updatedToken.GetToken(), fetched.GetToken(), cmpOpts...))
	})

	t.Run("bound keypair tokens require non-nil field", func(t *testing.T) {
		t.Parallel()

		token := newBoundKeypairToken()
		token.GetSpec().ClearBoundKeypair()

		_, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.ErrorContains(t, err, "bound_keypair tokens require a non-nil spec.bound_keypair")
	})

	t.Run("bound keypair initialized from empty", func(t *testing.T) {
		t.Parallel()

		token := newBoundKeypairToken()

		upserted, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		spec := upserted.GetToken().GetSpec().GetBoundKeypair()
		require.NotNil(t, spec)
		status := upserted.GetToken().GetStatus().GetUsage().GetBoundKeypair()
		require.NotNil(t, status)

		require.NotEmpty(t, upserted.GetToken().GetMetadata().GetName(), "name should be set")
		require.Equal(t, string(types.JoinMethodBoundKeypair), upserted.GetToken().GetSpec().GetJoinMethod(), "join method should default be bound_keypair")
		require.Empty(t, spec.GetRecovery().GetMode())
		require.EqualValues(t, 1, spec.GetRecovery().GetLimit())
		require.NotEmpty(t, status.GetRegistrationSecret())
	})

	t.Run("bound keypair initialized with existing registration secret", func(t *testing.T) {
		t.Parallel()

		token := newBoundKeypairToken()
		token.GetSpec().GetBoundKeypair().SetOnboarding(joiningv1.BoundKeypairSpec_OnboardingSpec_builder{
			RegistrationSecret: "abc123",
		}.Build())

		upserted, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		status := upserted.GetToken().GetStatus().GetUsage().GetBoundKeypair()
		require.Equal(t, "abc123", status.GetRegistrationSecret(), "specified registration secret must be copied to status")
	})

	t.Run("bound keypair initialized with preregistered key", func(t *testing.T) {
		t.Parallel()

		token := newBoundKeypairToken()
		token.GetSpec().GetBoundKeypair().SetOnboarding(joiningv1.BoundKeypairSpec_OnboardingSpec_builder{
			InitialPublicKey: "abc123",
		}.Build())

		upserted, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		status := upserted.GetToken().GetStatus().GetUsage().GetBoundKeypair()

		require.Empty(t, status.GetBoundPublicKey(), "bound public key must not be copied at creation time")
		require.Empty(t, status.GetRegistrationSecret(), "no registration secret can be generated")
	})

	rejectCases := []struct {
		name    string
		mutate  func(t *testing.T, update *joiningv1.ScopedToken)
		wantErr string
	}{
		{
			name: "scope is changed",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.SetScope("/other")
				update.GetSpec().SetAssignedScope("/other/one")
			},
			wantErr: "cannot modify scope of existing scoped token",
		},
		{
			name: "usage mode is changed",
			mutate: func(t *testing.T, update *joiningv1.ScopedToken) {
				update.SetSpec(proto.CloneOf(update.GetSpec()))
				update.GetSpec().SetUsageMode(string(joining.TokenUsageModeSingle))
			},
			wantErr: "cannot modify usage mode of existing scoped token",
		},
	}

	for _, tc := range rejectCases {
		t.Run("upsert fails because "+tc.name, func(t *testing.T) {
			t.Parallel()
			token := newToken()
			_, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
			require.NoError(t, err)

			updated := proto.CloneOf(token)
			tc.mutate(t, updated)

			_, err = service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: updated}.Build())
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("secret is preserved when not included", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		created, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)
		originalSecret := created.GetToken().GetStatus().GetSecret()
		require.NotEmpty(t, originalSecret)

		// attempt to overwrite the secret via upsert
		updated := proto.CloneOf(token)
		updated.ClearStatus()

		// upsert should succeed but preserve the original secret
		_, err = service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: updated}.Build())
		require.NoError(t, err)

		// confirm the original secret is still in place
		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, originalSecret, fetched.GetToken().GetStatus().GetSecret())
	})

	t.Run("upsert succeeds when revisions don't match", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		_, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		fetched1, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)

		update1 := proto.CloneOf(fetched1.GetToken())
		update1.GetMetadata().SetLabels(map[string]string{"env": "production"})
		_, err = service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: update1}.Build())
		require.NoError(t, err)

		// This should succeed as we set the revision before comparing
		update2 := proto.CloneOf(fetched1.GetToken())
		update2.GetMetadata().SetRevision("somerev")
		update2.GetMetadata().SetLabels(map[string]string{"env": "staging"})
		_, err = service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: update2}.Build())
		require.NoError(t, err, "upsert should succeed with retry on modification with stale revision")

		// Verify that update2 succeeded and overwrote update1's label
		final, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, "staging", final.GetToken().GetMetadata().GetLabels()["env"])
	})

	t.Run("concurrent upserts succeed", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		// Simulate 4 users concurrently upserting the same token with
		// different labels. Every upsert should succeed thanks to the
		// built-in retry logic.
		const numUsers = 4

		var wg errgroup.Group
		for i := range numUsers {
			wg.Go(func() error {
				update := proto.CloneOf(token)
				update.GetMetadata().SetLabels(map[string]string{
					"user": fmt.Sprintf("user-%d", i),
				})
				_, err := service.UpsertScopedToken(ctx, joiningv1.UpsertScopedTokenRequest_builder{Token: update}.Build())
				return err
			})
		}
		require.NoError(t, wg.Wait(), "concurrent upserts should all succeed")

		// Verify the token still exists and is consistent.
		final, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, final.GetToken())
		// One of the concurrent writers must have won; the label should
		// belong to one of the 4 users.
		label := final.GetToken().GetMetadata().GetLabels()["user"]
		require.Contains(t, []string{"user-0", "user-1", "user-2", "user-3"}, label)
	})
}

func TestScopedTokenCreate(t *testing.T) {
	t.Parallel()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	cmpOpts := []gocmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	t.Run("create a new scoped token", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       created.GetToken().GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(created.GetToken(), fetched.GetToken(), cmpOpts...))
	})

	t.Run("create a token with no secret", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		token.GetStatus().SetSecret("")
		created, err := service.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: token}.Build())
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, joiningv1.GetScopedTokenRequest_builder{
			Name:       created.GetToken().GetMetadata().GetName(),
			WithSecret: true,
		}.Build())
		require.NoError(t, err)

		opts := append(slices.Clone(cmpOpts), protocmp.IgnoreFields(&joiningv1.ScopedTokenStatus{}, "secret"))
		assert.Empty(t, gocmp.Diff(created.GetToken(), fetched.GetToken(), opts...))
		// token should be assigned a random secret
		assert.NotEmpty(t, created.GetToken().GetStatus().GetSecret())
	})
}
