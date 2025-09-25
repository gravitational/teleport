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
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
)

func assertEqualScopedTokens(t *testing.T, expected *joiningv1.ScopedToken, val *joiningv1.ScopedToken) bool {
	return assert.Equal(t, expected.GetMetadata().GetName(), val.GetMetadata().GetName()) &&
		assert.Equal(t, expected.GetScope(), val.GetScope()) &&
		assert.Equal(t, expected.GetSpec().GetAssignedScope(), val.GetSpec().GetAssignedScope()) &&
		assert.Equal(t, expected.GetSpec().GetJoinMethod(), val.GetSpec().GetJoinMethod())
}

func TestScopedTokenService(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	token := &joiningv1.ScopedToken{
		Metadata: &headerv1.Metadata{
			Name: "testtoken",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test/one",
			JoinMethod:    "token",
		},
	}

	created, err := service.CreateScopedToken(ctx, token)
	require.NoError(t, err)
	assertEqualScopedTokens(t, token, created)

	created.Spec.AssignedScope = "/test/two"

	updated, err := service.UpdateScopedToken(ctx, created)
	require.NoError(t, err)
	assertEqualScopedTokens(t, created, updated)

	fetched, err := service.GetScopedToken(ctx, token.Metadata.Name)
	require.NoError(t, err)
	assertEqualScopedTokens(t, updated, fetched)

	token2 := proto.CloneOf(token)
	token2.Metadata.Name = "testtoken2"
	token2.Scope = "/test2"
	token2.Spec.AssignedScope = "/test2/one"

	upserted, err := service.UpsertScopedToken(ctx, token2)
	require.NoError(t, err)
	assertEqualScopedTokens(t, token2, upserted)

	upserted.Spec.AssignedScope = "/test2/two"
	upserted2, err := service.UpsertScopedToken(ctx, token2)
	require.NoError(t, err)
	assertEqualScopedTokens(t, upserted, upserted2)
}

func TestScopedTokenList(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := local.NewScopedTokenService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := t.Context()

	test := &joiningv1.ScopedToken{
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

	test3 := proto.CloneOf(test)
	test3.Metadata.Name = "test3"
	test3.Scope = "/test/aa/bb"
	test3.Spec.AssignedScope = test3.Scope

	test4 := proto.CloneOf(test)
	test4.Metadata.Name = "test4"
	test4.Spec.AssignedScope = "/test/aa"
	test4.Scope = "/test/aa"
	test4.Spec.AssignedScope = test4.Scope
	test4.Spec.Roles = []string{types.RoleAuth.String()}

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
		_, err = service.CreateScopedToken(ctx, token)
		require.NoError(t, err)
	}

	sortFn := func(left *joiningv1.ScopedToken, right *joiningv1.ScopedToken) int {
		return cmp.Compare(left.Metadata.Name, right.Metadata.Name)
	}
	cases := []struct {
		name     string
		filters  *services.ScopedTokenFilters
		expected []*joiningv1.ScopedToken
	}{
		{
			name:     "all tokens (no filters)",
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4, stage, stage1, stage2},
		},
		{
			name: "tokens assigning scope descendant of /test",
			filters: &services.ScopedTokenFilters{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens assigning scope descendant of /test/aa",
			filters: &services.ScopedTokenFilters{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				},
			},
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens assigning scope ancestor to /test/bb",
			filters: &services.ScopedTokenFilters{
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				},
			},
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendants of /test",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3, test4},
		},
		{
			name: "tokens descendants of /test/aa",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test/aa",
				},
			},
			expected: []*joiningv1.ScopedToken{test1, test3, test4},
		},
		{
			name: "tokens ancestor to /test/bb",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/test/bb",
				},
			},
			expected: []*joiningv1.ScopedToken{test, test2},
		},
		{
			name: "tokens descendant of /stage assigning /stage/aa",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage",
				},
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
			},
			expected: []*joiningv1.ScopedToken{stage1, stage2},
		},
		{
			name: "tokens descendant of /stage/aa assigning /stage/aa",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
				AssignedScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/stage/aa",
				},
			},
			expected: []*joiningv1.ScopedToken{stage2},
		},
		{
			name: "tokens in /test scope applying node role",
			filters: &services.ScopedTokenFilters{
				ResourceScope: &scopesv1.Filter{
					Mode:  scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/test",
				},
				Roles: types.SystemRoles{types.RoleNode},
			},
			expected: []*joiningv1.ScopedToken{test, test1, test2, test3},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tokens, _, err := service.ListScopedTokens(ctx, 10, "", c.filters)
			require.NoError(t, err)

			slices.SortStableFunc(c.expected, sortFn)
			slices.SortStableFunc(tokens, sortFn)
			require.Len(t, tokens, len(c.expected))
			for i, token := range tokens {
				assertEqualScopedTokens(t, c.expected[i], token)
			}
		})
	}
}
