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
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestProvisioningService_ListProvisionTokens_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name     string
		count    int
		pageSize int
		nextPage string
	}{
		{
			name:     "empty",
			count:    10,
			pageSize: defaults.MaxIterationLimit,
			nextPage: "",
		},
		{
			name:     "count smaller than page size",
			count:    1,
			pageSize: 2,
			nextPage: "",
		},
		{
			name:     "count bigger than page size",
			count:    3,
			pageSize: 2,
			nextPage: "test-token-3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := newProvisioningService(t, clockwork.NewFakeClock())

			for j := range tc.count {
				err := service.CreateToken(ctx, &types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: "test-token-" + strconv.Itoa(j+1),
					},
					Spec: types.ProvisionTokenSpecV2{
						Roles: []types.SystemRole{
							types.RoleAdmin,
						},
					},
				})
				require.NoError(t, err)
			}

			result, nextPage, err := service.ListProvisionTokens(ctx, tc.pageSize, "", nil, "")
			require.NoError(t, err)
			assert.Equal(t, tc.nextPage, nextPage)
			if tc.count > tc.pageSize {
				assert.Len(t, result, tc.pageSize)

				result, nextPage, err = service.ListProvisionTokens(ctx, tc.pageSize, nextPage, nil, "")
				require.NoError(t, err)

				assert.Empty(t, nextPage)
				assert.Len(t, result, tc.count-tc.pageSize)
			} else {
				assert.Len(t, result, tc.count)
			}
		})
	}
}

func TestProvisioningService_ListProvisionTokens_Filters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service := newProvisioningService(t, clockwork.NewFakeClock())

	tokens := []struct {
		roles   types.SystemRoles
		botName string
	}{
		{
			roles:   types.SystemRoles{types.RoleAdmin},
			botName: "",
		},
		{
			roles:   types.SystemRoles{types.RoleAdmin, types.RoleBot},
			botName: "bot-1",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-2",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-1",
		},
	}

	for i, token := range tokens {
		err := service.CreateToken(ctx, &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: "test-token-" + strconv.Itoa(i+1),
			},
			Spec: types.ProvisionTokenSpecV2{
				Roles:   token.roles,
				BotName: token.botName,
			},
		})
		require.NoError(t, err)
	}

	result, _, err := service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, result, 4)

	t.Run("filter roles", func(t *testing.T) {
		result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode, types.RoleBot}, "")
		require.NoError(t, err)
		assert.Len(t, result, 4)

		result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode}, "")
		require.NoError(t, err)
		assert.Len(t, result, 2)

		result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleBot}, "")
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("filter bot name", func(t *testing.T) {
		result, _, err := service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-1")
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "test-token-2", result[0].GetName())

		result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-2")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test-token-3", result[0].GetName())
	})

	t.Run("filter roles and bot name", func(t *testing.T) {
		result, _, err := service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin}, "bot-1")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test-token-2", result[0].GetName())
	})
}

func TestMatchToken(t *testing.T) {
	t.Parallel()

	tokenEmpty := types.ProvisionTokenSpecV2{}

	tokenWithNodeRole := types.ProvisionTokenSpecV2{
		Roles: types.SystemRoles{types.RoleNode},
	}

	tokenBot1WithNoBotRole := types.ProvisionTokenSpecV2{
		Roles:   nil,
		BotName: "bot-1",
	}

	tokenBot1WithBotRole := types.ProvisionTokenSpecV2{
		Roles:   types.SystemRoles{types.RoleBot},
		BotName: "bot-1",
	}

	tokenBot1WithNodeAndBotRole := types.ProvisionTokenSpecV2{
		Roles:   types.SystemRoles{types.RoleNode, types.RoleBot},
		BotName: "bot-1",
	}

	tc := []struct {
		name          string
		filterRoles   types.SystemRoles
		filterBotName string
		tokens        []struct {
			spec  types.ProvisionTokenSpecV2
			match bool
		}
	}{
		{
			name:          "empty filters",
			filterRoles:   nil,
			filterBotName: "",
			tokens: []struct {
				spec  types.ProvisionTokenSpecV2
				match bool
			}{
				{spec: tokenEmpty, match: true},
				{spec: tokenWithNodeRole, match: true},
				{spec: tokenBot1WithNoBotRole, match: true},
				{spec: tokenBot1WithBotRole, match: true},
				{spec: tokenBot1WithNodeAndBotRole, match: true},
			},
		},
		{
			name:          "role filter (single)",
			filterRoles:   types.SystemRoles{types.RoleNode},
			filterBotName: "",
			tokens: []struct {
				spec  types.ProvisionTokenSpecV2
				match bool
			}{
				{spec: tokenEmpty, match: false},
				{spec: tokenWithNodeRole, match: true},
				{spec: tokenBot1WithNoBotRole, match: false},
				{spec: tokenBot1WithBotRole, match: false},
				{spec: tokenBot1WithNodeAndBotRole, match: true},
			},
		},
		{
			name:          "role filter (multiple)",
			filterRoles:   types.SystemRoles{types.RoleNode, types.RoleBot},
			filterBotName: "",
			tokens: []struct {
				spec  types.ProvisionTokenSpecV2
				match bool
			}{
				{spec: tokenEmpty, match: false},
				{spec: tokenWithNodeRole, match: true},
				{spec: tokenBot1WithNoBotRole, match: false},
				{spec: tokenBot1WithBotRole, match: true},
				{spec: tokenBot1WithNodeAndBotRole, match: true},
			},
		},
		{
			name:          "bot name filter",
			filterRoles:   nil,
			filterBotName: "bot-1",
			tokens: []struct {
				spec  types.ProvisionTokenSpecV2
				match bool
			}{
				{spec: tokenEmpty, match: false},
				{spec: tokenWithNodeRole, match: false},
				{spec: tokenBot1WithNoBotRole, match: false},
				{spec: tokenBot1WithBotRole, match: true},
				{spec: tokenBot1WithNodeAndBotRole, match: true},
			},
		},
		{
			name:          "role and bot name filters",
			filterRoles:   types.SystemRoles{types.RoleNode},
			filterBotName: "bot-1",
			tokens: []struct {
				spec  types.ProvisionTokenSpecV2
				match bool
			}{
				{spec: tokenEmpty, match: false},
				{spec: tokenWithNodeRole, match: false},
				{spec: tokenBot1WithNoBotRole, match: false},
				{spec: tokenBot1WithBotRole, match: false},
				{spec: tokenBot1WithNodeAndBotRole, match: true},
			},
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			for _, token := range c.tokens {
				match := local.MatchToken(&types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: "test-token",
					},
					Spec: token.spec,
				}, c.filterRoles, c.filterBotName)
				assert.Equal(t, token.match, match, "not match token: %+v", token.spec)
			}
		})
	}
}

func TestProvisioningServiceTokenNameConflict(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service := local.NewProvisioningService(bk)
	scopedTokenService, err := local.NewScopedTokenService(bk)
	require.NoError(t, err)

	ctx := t.Context()

	token := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name: "testtoken",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{
				types.RoleAdmin,
			},
		},
	}
	// create initial token
	err = service.CreateToken(ctx, token)
	require.NoError(t, err)

	// assert that creating another token with the same name fails
	err = service.CreateToken(ctx, token)
	require.True(t, trace.IsAlreadyExists(err))

	// assert that upserting a token with the same name still succeeds
	err = service.UpsertToken(ctx, token)
	require.NoError(t, err)

	// create a scoped token
	scopedToken := joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "testtoken2",
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
	_, err = scopedTokenService.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: scopedToken,
	}.Build())
	require.NoError(t, err)

	token.SetName(scopedToken.GetMetadata().GetName())
	// assert that creating or upserting an unscoped token with a name that conflicts
	// with a scoped token fails
	err = service.CreateToken(ctx, token)
	require.True(t, trace.IsAlreadyExists(err))
	err = service.UpsertToken(ctx, token)
	require.True(t, trace.IsAlreadyExists(err))
}

func newProvisioningService(t *testing.T, clock clockwork.Clock) *local.ProvisioningService {
	t.Helper()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clock,
	})
	require.NoError(t, err)
	service := local.NewProvisioningService(backend)
	return service
}

func TestValidateProvisionToken(t *testing.T) {
	t.Parallel()

	makeToken := func(t *testing.T, spec types.ProvisionTokenSpecV2) types.ProvisionToken {
		t.Helper()

		if len(spec.Roles) == 0 {
			spec.Roles = []types.SystemRole{types.RoleNode}
		}

		token, err := types.NewProvisionTokenFromSpec("test", time.Now().Add(time.Hour), spec)
		require.NoError(t, err)
		return token
	}

	tests := []struct {
		name        string
		spec        types.ProvisionTokenSpecV2
		wantErr     bool
		errContains string
	}{
		{
			name: "ec2 rejects organizational unit include matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodEC2,
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{"ou-1234"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `the "ec2" join method does not support the "aws_organizational_units" parameter`,
		},
		{
			name: "ec2 rejects organizational unit exclude matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodEC2,
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Exclude: []string{"ou-1234"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `the "ec2" join method does not support the "aws_organizational_units" parameter`,
		},
		{
			name: "iam rejects organizational unit matchers without organization id",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{"ou-1234"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `allow rule with "aws_organizational_units" matchers must also specify "aws_organization_id" when using the "iam" join method`,
		},
		{
			name: "iam rejects exclude-only organizational unit matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Exclude: []string{"ou-1234"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `at least one entry in "aws_organizational_units.include" must be specified`,
		},
		{
			name: "iam rejects wildcard mixed with explicit include matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{types.Wildcard, "ou-1234"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `when using wildcard for "aws_organizational_units.include", no other values are allowed`,
		},
		{
			name: "iam rejects wildcard exclude matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{"ou-1234"},
							Exclude: []string{types.Wildcard},
						},
					},
				},
			},
			wantErr:     true,
			errContains: `using wildcard in "aws_organizational_units.exclude" is not allowed`,
		},
		{
			name: "iam accepts organization id without organizational unit matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
					},
				},
			},
		},
		{
			name: "iam accepts wildcard include matcher",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{types.Wildcard},
						},
					},
				},
			},
		},
		{
			name: "iam accepts explicit include and exclude matchers",
			spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
				Allow: []*types.TokenRule{
					{
						AWSOrganizationID: "o-123abcd",
						AWSOrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{"ou-1234"},
							Exclude: []string{"ou-5678"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := local.ValidateProvisionToken(makeToken(t, tt.spec))
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateOracleJoinToken(t *testing.T) {
	t.Parallel()
	makeToken := func(spec types.ProvisionTokenSpecV2) types.ProvisionToken {
		spec.JoinMethod = types.JoinMethodOracle
		spec.Roles = []types.SystemRole{types.RoleNode}
		token, err := types.NewProvisionTokenFromSpec("foo", time.Now().Add(time.Hour), spec)
		require.NoError(t, err)
		return token
	}

	t.Run("oracle", func(t *testing.T) {
		tests := []struct {
			name   string
			token  types.ProvisionToken
			assert require.ErrorAssertionFunc
		}{
			{
				name: "ok",
				token: makeToken(types.ProvisionTokenSpecV2{
					Oracle: &types.ProvisionTokenSpecV2Oracle{
						Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
							{
								Tenancy:            makeTenancyID("foo"),
								ParentCompartments: []string{makeCompartmentID("foo"), makeCompartmentID("bar")},
								Regions:            []string{"us-phoenix-1", "iad"},
							},
						},
					},
				}),
				assert: require.NoError,
			},
			{
				name: "invalid tenant",
				token: makeToken(types.ProvisionTokenSpecV2{
					Oracle: &types.ProvisionTokenSpecV2Oracle{
						Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
							{
								Tenancy:            "foo",
								ParentCompartments: []string{makeCompartmentID("foo"), makeCompartmentID("bar")},
								Regions:            []string{"us-phoenix-1", "iad"},
							},
						},
					},
				}),
				assert: require.Error,
			},
			{
				name: "invalid compartment",
				token: makeToken(types.ProvisionTokenSpecV2{
					Oracle: &types.ProvisionTokenSpecV2Oracle{
						Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
							{
								Tenancy:            makeTenancyID("foo"),
								ParentCompartments: []string{"foo", makeCompartmentID("bar")},
								Regions:            []string{"us-phoenix-1", "iad"},
							},
						},
					},
				}),
				assert: require.Error,
			},
			{
				name: "invalid region",
				token: makeToken(types.ProvisionTokenSpecV2{
					Oracle: &types.ProvisionTokenSpecV2Oracle{
						Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
							{
								Tenancy:            makeTenancyID("foo"),
								ParentCompartments: []string{makeCompartmentID("foo"), makeCompartmentID("bar")},
								Regions:            []string{"invalid", "iad"},
							},
						},
					},
				}),
				assert: require.Error,
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				tc.assert(t, local.ValidateOracleJoinToken(tc.token))
			})
		}
	})
}

func makeOCID(resourceType, region, id string) string {
	return fmt.Sprintf("ocid1.%s.oc1.%s.%s", resourceType, region, id)
}

func makeTenancyID(id string) string {
	return makeOCID("tenancy", "", id)
}

func makeCompartmentID(id string) string {
	return makeOCID("compartment", "", id)
}
