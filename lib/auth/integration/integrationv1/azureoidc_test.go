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
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGenerateAzureOIDCToken(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"
	integrationName := "my-integration"

	publicURL := "https://example.com"

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, publicURL)

	// Create integration
	ig, err := types.NewIntegrationAzureOIDC(
		types.Metadata{Name: integrationName},
		&types.AzureOIDCIntegrationSpecV1{
			TenantID: "foo",
			ClientID: "bar",
		},
	)
	require.NoError(t, err)
	_, err = localClient.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	t.Run("only Auth, Discovery, and Proxy roles should be able to generate Azure tokens", func(t *testing.T) {
		// A dummy user should not be able to generate Azure OIDC tokens
		ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{
				{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbUse}},
			}},
		}, localClient)
		_, err = resourceSvc.GenerateAzureOIDCToken(ctx, &integrationv1.GenerateAzureOIDCTokenRequest{Integration: integrationName})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %T", err)

		// Auth, Discovery, and Proxy roles should be able to generate Azure OIDC tokens
		for _, allowedRole := range []types.SystemRole{types.RoleAuth, types.RoleDiscovery, types.RoleProxy} {
			ctx = authz.ContextWithUser(ctx, authz.BuiltinRole{
				Role:                  types.RoleInstance,
				AdditionalSystemRoles: []types.SystemRole{allowedRole},
				Username:              string(allowedRole),
				Identity: tlsca.Identity{
					Username: string(allowedRole),
				},
			})

			_, err := resourceSvc.GenerateAzureOIDCToken(ctx, &integrationv1.GenerateAzureOIDCTokenRequest{Integration: integrationName})
			require.NoError(t, err)
		}
	})

	t.Run("validate the Azure token", func(t *testing.T) {
		ctx = authz.ContextWithUser(ctx, authz.BuiltinRole{
			Role:                  types.RoleInstance,
			AdditionalSystemRoles: []types.SystemRole{types.RoleDiscovery},
			Username:              string(types.RoleDiscovery),
			Identity: tlsca.Identity{
				Username: string(types.RoleDiscovery),
			},
		})
		resp, err := resourceSvc.GenerateAzureOIDCToken(ctx, &integrationv1.GenerateAzureOIDCTokenRequest{
			Integration: integrationName,
		})
		require.NoError(t, err)

		// Validate JWT against public key
		require.NotEmpty(t, ca.GetActiveKeys().JWT)
		jwtPubKey := ca.GetActiveKeys().JWT[0].PublicKey
		publicKey, err := keys.ParsePublicKey(jwtPubKey)
		require.NoError(t, err)
		key, err := jwt.New(&jwt.Config{
			ClusterName: clusterName,
			Clock:       resourceSvc.clock,
			PublicKey:   publicKey,
		})
		require.NoError(t, err)

		// Verify the Azure token using the JWT
		_, err = key.VerifyAzureToken(resp.Token)
		require.NoError(t, err)
	})
}
