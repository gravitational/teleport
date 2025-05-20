// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIdentityCenterAccountMatcher(t *testing.T) {
	testCases := []struct {
		name            string
		roleAssignments []types.IdentityCenterAccountAssignment
		condition       types.RoleConditionType
		matcher         RoleMatcher
		expectMatch     require.BoolAssertionFunc
	}{
		{
			name:            "empty nonmatch",
			roleAssignments: nil,
			condition:       types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "11111111",
			},
			expectMatch: require.False,
		},
		{
			name: "simple account match",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "11111111",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "11111111",
			},
			expectMatch: require.True,
		},
		{
			name: "multiple account assignments match",
			roleAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       "00000000",
					PermissionSet: "some:arn",
				},
				{
					Account:       "11111111",
					PermissionSet: "some:arn",
				},
			},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "11111111",
			},
			expectMatch: require.True,
		},
		{
			name: "simple account nonmatch",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "11111111",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "potato",
			},
			expectMatch: require.False,
		},
		{
			name: "multiple account assignments match",
			roleAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       "00000000",
					PermissionSet: "some:arn",
				},
				{
					Account:       "11111111",
					PermissionSet: "some:arn",
				},
			},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "66666666",
			},
			expectMatch: require.False,
		},
		{
			name: "account glob match",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "potato",
			},
			expectMatch: require.True,
		},
		{
			name: "account glob nonmatch",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*!",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountMatcher{
				accountID: "potato",
			},
			expectMatch: require.False,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			roleSpec := types.RoleSpecV6{}
			condition := &roleSpec.Deny
			if testCase.condition == types.Allow {
				condition = &roleSpec.Allow
			}
			condition.AccountAssignments = append(condition.AccountAssignments,
				testCase.roleAssignments...)

			r, err := types.NewRole("test", roleSpec)
			require.NoError(t, err)

			match, err := testCase.matcher.Match(r, testCase.condition)
			require.NoError(t, err)

			testCase.expectMatch(t, match)
		})
	}
}

func TestIdentityCenterAccountAssignmentMatcher(t *testing.T) {
	testCases := []struct {
		name            string
		roleAssignments []types.IdentityCenterAccountAssignment
		condition       types.RoleConditionType
		matcher         RoleMatcher
		expectMatch     require.BoolAssertionFunc
	}{
		{
			name:            "empty nonmatch",
			roleAssignments: nil,
			condition:       types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.False,
		},
		{
			name: "simple match",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "11111111",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.True,
		},
		{
			name: "multiple match",
			roleAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       "00000000",
					PermissionSet: "some:arn",
				},
				{
					Account:       "11111111",
					PermissionSet: "some:arn",
				},
			},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.True,
		},
		{
			name: "multiple nonmatch",
			roleAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       "00000000",
					PermissionSet: "some:arn",
				},
				{
					Account:       "11111111",
					PermissionSet: "some:arn",
				},
			},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "66666666",
				permissionSetARN: "some:other:arn",
			},
			expectMatch: require.False,
		},
		{
			name: "account glob",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*1",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.True,
		},
		{
			name: "account glob nonmatch",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*!!!!",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.False,
		},
		{
			name: "globbed",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*",
				PermissionSet: "*",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.True,
		},
		{
			name: "globbed nonmatch",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "*",
				PermissionSet: ":not:an:arn:*",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.False,
		},
		{
			name: "bad account",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "11111111",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "potato",
				permissionSetARN: "some:arn",
			},
			expectMatch: require.False,
		},
		{
			name: "bad permissionset arn",
			roleAssignments: []types.IdentityCenterAccountAssignment{{
				Account:       "11111111",
				PermissionSet: "some:arn",
			}},
			condition: types.Allow,
			matcher: &IdentityCenterAccountAssignmentMatcher{
				accountID:        "11111111",
				permissionSetARN: "banana",
			},
			expectMatch: require.False,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			roleSpec := types.RoleSpecV6{}
			condition := &roleSpec.Deny
			if testCase.condition == types.Allow {
				condition = &roleSpec.Allow
			}
			condition.AccountAssignments = append(condition.AccountAssignments,
				testCase.roleAssignments...)

			r, err := types.NewRole("test", roleSpec)
			require.NoError(t, err)

			match, err := testCase.matcher.Match(r, testCase.condition)
			require.NoError(t, err)

			testCase.expectMatch(t, match)
		})
	}
}
