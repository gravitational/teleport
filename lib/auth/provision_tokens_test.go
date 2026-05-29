/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

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
			err := validateProvisionToken(makeToken(t, tt.spec))
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}
