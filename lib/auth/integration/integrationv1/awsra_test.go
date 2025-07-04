/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package integrationv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGenerateAWSRACredentials(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"
	integrationName := "my-integration"
	proxyPublicAddr := "example.com:443"

	ca := newCertAuthority(t, types.AWSRACA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, proxyPublicAddr)

	ig, err := types.NewIntegrationAWSRA(
		types.Metadata{Name: integrationName},
		&types.AWSRAIntegrationSpecV1{
			TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		},
	)
	require.NoError(t, err)
	_, err = localClient.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{
			{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbUse}},
		}},
	}, localClient)

	t.Run("requesting with an user should return access denied", func(t *testing.T) {
		ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{
				{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbUse}},
			}},
		}, localClient)

		_, err := resourceSvc.GenerateAWSRACredentials(ctx, &integrationv1.GenerateAWSRACredentialsRequest{
			Integration: integrationName,
			RoleArn:     "arn:aws:iam::123456789012:role/OpsTeam",
			ProfileArn:  "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
			SubjectName: "test",
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %T", err)
	})

	t.Run("auth and proxy can request credentials", func(t *testing.T) {
		for _, allowedRole := range []types.SystemRole{types.RoleAuth, types.RoleProxy} {
			ctx = authz.ContextWithUser(ctx, authz.BuiltinRole{
				Role:                  types.RoleInstance,
				AdditionalSystemRoles: []types.SystemRole{allowedRole},
				Username:              string(allowedRole),
				Identity: tlsca.Identity{
					Username: string(allowedRole),
				},
			})

			_, err := resourceSvc.GenerateAWSRACredentials(ctx, &integrationv1.GenerateAWSRACredentialsRequest{
				Integration:                   integrationName,
				RoleArn:                       "arn:aws:iam::123456789012:role/OpsTeam",
				ProfileArn:                    "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
				ProfileAcceptsRoleSessionName: true,
				SubjectName:                   "test",
			})
			require.NoError(t, err)
		}
	})
}
