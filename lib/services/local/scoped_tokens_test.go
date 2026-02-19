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

	token := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "testtoken",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test/one",
			JoinMethod:    "token",
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret",
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
	assert.Empty(t, gocmp.Diff(token, created.Token, cmpOpts...))

	updatedToken := proto.CloneOf(token)
	updatedToken.Spec.AssignedScope = "/test/test"

	updated, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{
		Token: created.GetToken(),
	})
	require.NoError(t, err)

	fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name: token.Metadata.Name,
	})
	require.NoError(t, err)
	require.Equal(t, fetched.Token.Spec.AssignedScope, updated.Token.Spec.AssignedScope)
	require.Empty(t, fetched.Token.Status.Secret)

	list, err := service.ListScopedTokens(ctx, &joiningv1.ListScopedTokensRequest{})
	require.NoError(t, err)
	require.Len(t, list.Tokens, 1)

	fetched, err = service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name:       token.Metadata.Name,
		WithSecret: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, fetched.Token.Status.Secret)
	assert.Empty(t, gocmp.Diff(updated.Token, fetched.Token, cmpOpts...))

	_, err = service.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
		Name: fetched.Token.Metadata.Name,
	})
	require.NoError(t, err)
	_, err = service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name: fetched.Token.Metadata.Name,
	})
	require.True(t, trace.IsNotFound(err))

	expiredToken := proto.CloneOf(token)
	expiredToken.Metadata.Name = "expiredtoken"
	expiredToken.Metadata.Expires = timestamppb.New(time.Now().UTC().Add(-25 * time.Hour))

	activeToken := proto.CloneOf(token)
	activeToken.Metadata.Name = "activetoken"
	activeToken.Metadata.Expires = timestamppb.New(time.Now().UTC().Add(25 * time.Hour))

	expiredRes, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: expiredToken,
	})
	require.NoError(t, err)

	activeRes, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
		Token: activeToken,
	})
	require.NoError(t, err)

	// expired tokens should error and delete the token
	expiredToken, err = service.UseScopedToken(ctx, expiredRes.Token, nil)
	// If for some reason the expired token is not automatically deleted by the backend, a
	// LimitExceededError will be returned. Otherwise, we should expect the token not to be found.
	require.True(t, trace.IsLimitExceeded(err) || trace.IsNotFound(err))
	require.Nil(t, expiredToken)

	_, err = service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name: expiredRes.Token.Metadata.Name,
	})
	require.True(t, trace.IsNotFound(err))

	fetchedActive, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
		Name:       activeRes.Token.Metadata.Name,
		WithSecret: true,
	})

	require.NoError(t, err)
	assert.Empty(t, gocmp.Diff(activeRes.Token, fetchedActive.Token, cmpOpts...))
}

func TestScopedTokenList(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(bk)
	require.NoError(t, err)

	ctx := t.Context()

	test := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test",
			JoinMethod:    "token",
			Roles: []string{
				types.RoleNode.String(),
			},
			UsageMode: string(joining.TokenUsageModeUnlimited),
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret",
		},
	}

	test1 := proto.CloneOf(test)
	test1.Metadata.Name = "test1"
	test1.Scope = "/test/aa"
	test1.Spec.AssignedScope = test1.Scope

	test2 := proto.CloneOf(test)
	test2.Metadata.Name = "test2"
	test2.Scope = "/test/bb"
	test2.Spec.AssignedScope = test2.Scope
	test2.Metadata.Labels = map[string]string{
		"hello": "world",
	}

	test3 := proto.CloneOf(test)
	test3.Metadata.Name = "test3"
	test3.Scope = "/test/aa/bb"
	test3.Spec.AssignedScope = test3.Scope

	test4 := proto.CloneOf(test)
	test4.Metadata.Name = "test4"
	test4.Spec.AssignedScope = "/test/aa"
	test4.Scope = "/test/aa"
	test4.Spec.AssignedScope = test4.Scope

	stage := proto.CloneOf(test)
	stage.Metadata.Name = "stage"
	stage.Scope = "/stage"
	stage.Spec.AssignedScope = stage.Scope

	stage1 := proto.CloneOf(stage)
	stage1.Metadata.Name = "stage1"
	stage1.Spec.AssignedScope = "/stage/aa"

	stage2 := proto.CloneOf(stage)
	stage2.Metadata.Name = "stage2"
	stage2.Scope = "/stage/aa"
	stage2.Spec.AssignedScope = "/stage/aa"

	allTokens := []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2}
	for _, token := range allTokens {
		_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{Token: token})
		require.NoError(t, err)
	}

	sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
		return cmp.Compare(left.Metadata.Name, right.Metadata.Name)
	}
	cases := []struct {
		name     string
		req      *joiningv1.ListScopedTokensRequest
		expected []*joiningv1.ScopedToken
	}{
		{
			name: "all tokens (no filters)",
			req: &joiningv1.ListScopedTokensRequest{
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2},
		},
		{
			name: "tokens assigning scope descendant of /test",
			req: &joiningv1.ListScopedTokensRequest{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens assigning scope descendant of /test/aa",
			req: &joiningv1.ListScopedTokensRequest{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens assigning scope ancestor to /test/bb",
			req: &joiningv1.ListScopedTokensRequest{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendants of /test",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens descendants of /test/aa",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens ancestor to /test/bb",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendant of /stage assigning /stage/aa",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage",
				},
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{stage1, stage2},
		},
		{
			name: "tokens descendant of /stage/aa assigning /stage/aa",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{stage2},
		},
		{
			name: "tokens in /test scope applying auth role",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
				Roles:       []string{types.RoleAuth.String()},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{},
		},
		{
			name: "tokens in /test scope filtered by label",
			req: &joiningv1.ListScopedTokensRequest{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
				Roles: []string{types.RoleNode.String()},
				Labels: map[string]string{
					"hello": "world",
				},
				WithSecrets: true,
			},
			expected: []*joiningv1.ScopedToken{test2},
		},
		{
			name: "tokens have no secrets when WithSecrets is false",
			req: &joiningv1.ListScopedTokensRequest{
				WithSecrets: false,
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := proto.CloneOf(c.req)
			req.Limit = 10
			res, err := service.ListScopedTokens(ctx, req)
			require.NoError(t, err)

			slices.SortStableFunc(c.expected, sortFn)
			slices.SortStableFunc(res.GetTokens(), sortFn)
			require.Len(t, res.GetTokens(), len(c.expected))
			cmpOpts := []gocmp.Option{
				protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
				protocmp.Transform(),
			}
			if !req.WithSecrets {
				cmpOpts = append(cmpOpts, protocmp.IgnoreFields(&joiningv1.ScopedTokenStatus{}, "secret"))
			}
			for i, token := range res.GetTokens() {
				if !req.WithSecrets {
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

	token := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "testtoken",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test/one",
			JoinMethod:    "token",
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret",
		},
	}

	t.Run("basic", func(t *testing.T) {
		// create initial scoped token
		_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
			Token: token,
		})
		require.NoError(t, err)

		// assert that creating another scoped token with the same name fails
		_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
			Token: token,
		})
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
		conflictWithClassic.Metadata.Name = classicToken.GetName()
		_, err = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
			Token: conflictWithClassic,
		})
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
				token.Metadata.Name = name
				_, scopedErr = service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
					Token: token,
				})
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

		token := &joiningv1.ScopedToken{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "testtoken",
				Expires: timestamppb.New(time.Now().Add(24 * time.Hour)),
			},
			Scope: "/test",
			Spec: &joiningv1.ScopedTokenSpec{
				AssignedScope: "/test/one",
				JoinMethod:    "token",
				Roles:         []string{types.RoleNode.String()},
				UsageMode:     string(joining.TokenUsageModeSingle),
			},
			Status: &joiningv1.ScopedTokenStatus{
				Secret: "secret",
			},
		}

		created, err := service.CreateScopedToken(ctx, &joiningv1.CreateScopedTokenRequest{
			Token: token,
		})
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

func TestScopedTokenUpsert(t *testing.T) {
	t.Parallel()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	newToken := func() *joiningv1.ScopedToken {
		return &joiningv1.ScopedToken{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.New().String(),
			},
			Scope: "/test",
			Spec: &joiningv1.ScopedTokenSpec{
				AssignedScope: "/test/one",
				JoinMethod:    "token",
				Roles:         []string{types.RoleNode.String()},
				UsageMode:     string(joining.TokenUsageModeUnlimited),
			},
			Status: &joiningv1.ScopedTokenStatus{
				Secret: "secret",
			},
		}
	}
	cmpOpts := []gocmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	t.Run("upsert creates a new entry", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		upsertedToken, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       upsertedToken.GetToken().GetMetadata().GetName(),
			WithSecret: true,
		})
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(upsertedToken.GetToken(), fetched.GetToken(), cmpOpts...))
	})

	t.Run("upsert updates existing entry", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		updated := proto.CloneOf(token)
		updated.Metadata.Labels = map[string]string{"env": "test"}
		updated.Spec.AssignedScope = "/test/one/two"

		updatedToken, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: updated})
		require.NoError(t, err)

		fetched, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		})
		require.NoError(t, err)
		assert.Empty(t, gocmp.Diff(updatedToken.GetToken(), fetched.GetToken(), cmpOpts...))
	})

	t.Run("upsert fails because the scope is changed", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		updated := proto.CloneOf(token)
		updated.Scope = "/other"
		updated.Spec.AssignedScope = "/other/one"

		_, err = service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: updated})
		require.ErrorContains(t, err, "cannot modify scope of existing scoped token")
	})

	t.Run("upsert fails because the usage status is changed", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		updated := proto.CloneOf(token)
		updated.Spec = proto.CloneOf(token.Spec)
		updated.Spec.UsageMode = string(joining.TokenUsageModeSingle)

		_, err = service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: updated})
		require.ErrorContains(t, err, "cannot modify usage mode of existing scoped token")
	})

	t.Run("upsert fails because the secret is changed", func(t *testing.T) {
		t.Parallel()
		token := newToken()
		_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		updated := proto.CloneOf(token)
		updated.Status = proto.CloneOf(token.Status)
		updated.Status.Secret = "new-secret"

		_, err = service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: updated})
		require.ErrorContains(t, err, "cannot modify secret of existing scoped token")
	})

	t.Run("upsert succeeds when revisions don't match", func(t *testing.T) {
		t.Parallel()
		token := newToken()

		_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: token})
		require.NoError(t, err)

		fetched1, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		})
		require.NoError(t, err)

		update1 := proto.CloneOf(fetched1.GetToken())
		update1.Metadata.Labels = map[string]string{"env": "production"}
		_, err = service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: update1})
		require.NoError(t, err)

		// This should succeed as we set the revision before comparing
		update2 := proto.CloneOf(fetched1.GetToken())
		update2.GetMetadata().Revision = "somerev"
		update2.Metadata.Labels = map[string]string{"env": "staging"}
		_, err = service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: update2})
		require.NoError(t, err, "upsert should succeed with retry on modification with stale revision")

		// Verify that update2 succeeded and overwrote update1's label
		final, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		})
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
				update.Metadata.Labels = map[string]string{
					"user": fmt.Sprintf("user-%d", i),
				}
				_, err := service.UpsertScopedToken(ctx, &joiningv1.UpsertScopedTokenRequest{Token: update})
				return err
			})
		}
		require.NoError(t, wg.Wait(), "concurrent upserts should all succeed")

		// Verify the token still exists and is consistent.
		final, err := service.GetScopedToken(ctx, &joiningv1.GetScopedTokenRequest{
			Name:       token.GetMetadata().GetName(),
			WithSecret: true,
		})
		require.NoError(t, err)
		require.NotNil(t, final.GetToken())
		// One of the concurrent writers must have won; the label should
		// belong to one of the 4 users.
		label := final.GetToken().GetMetadata().GetLabels()["user"]
		require.Contains(t, []string{"user-0", "user-1", "user-2", "user-3"}, label)
	})
}
