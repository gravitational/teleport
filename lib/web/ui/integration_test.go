/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMakeIntegration(t *testing.T) {
	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{
			Name: "aws-oidc",
		},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OidcRole",
		},
	)
	require.NoError(t, err)

	githubIntegration, err := types.NewIntegrationGitHub(
		types.Metadata{
			Name: "github-my-org",
		},
		&types.GitHubIntegrationSpecV1{
			Organization: "my-org",
		},
	)
	require.NoError(t, err)

	testCases := []struct {
		integration types.Integration
		want        Integration
	}{
		{
			integration: oidcIntegration,
			want: Integration{
				Name:    "aws-oidc",
				SubKind: types.IntegrationSubKindAWSOIDC,
				AWSOIDC: &IntegrationAWSOIDCSpec{
					RoleARN: "arn:aws:iam::123456789012:role/OidcRole",
				},
			},
		},
		{
			integration: githubIntegration,
			want: Integration{
				Name:    "github-my-org",
				SubKind: types.IntegrationSubKindGitHub,
				GitHub: &IntegrationGitHub{
					Organization: "my-org",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.integration.GetName(), func(t *testing.T) {
			actual, err := MakeIntegration(tc.integration)
			require.NoError(t, err)
			require.NotNil(t, actual)
			require.Equal(t, tc.want, *actual)
		})
	}
}
