/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGenerateAWSOIDCToken(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"

	publicURL := "https://example.com"

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, publicURL)

	ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{
			{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbUse}},
		}},
	}, localClient)

	resp, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{})
	require.NoError(t, err)

	// Get Public Key
	require.NotEmpty(t, ca.GetActiveKeys().JWT)
	jwtPubKey := ca.GetActiveKeys().JWT[0].PublicKey

	publicKey, err := utils.ParsePublicKey(jwtPubKey)
	require.NoError(t, err)

	// Validate JWT against public key
	key, err := jwt.New(&jwt.Config{
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: clusterName,
		Clock:       resourceSvc.clock,
		PublicKey:   publicKey,
	})
	require.NoError(t, err)

	_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
		RawToken: resp.GetToken(),
		Issuer:   publicURL,
	})
	require.NoError(t, err)

	// Fails if the issuer is different
	_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
		RawToken: resp.GetToken(),
		Issuer:   publicURL + "3",
	})
	require.Error(t, err)
}

func TestConvertSecurityGroupRulesToProto(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       []awsoidc.SecurityGroupRule
		expected []*integrationv1.SecurityGroupRule
	}{
		{
			name: "valid",
			in: []awsoidc.SecurityGroupRule{{
				IPProtocol: "tcp",
				FromPort:   8080,
				ToPort:     8081,
				CIDRs: []awsoidc.CIDR{{
					CIDR:        "10.10.10.0/24",
					Description: "cidr x",
				}},
			}},
			expected: []*integrationv1.SecurityGroupRule{{
				IpProtocol: "tcp",
				FromPort:   8080,
				ToPort:     8081,
				Cidrs: []*integrationv1.SecurityGroupRuleCIDR{{
					Cidr:        "10.10.10.0/24",
					Description: "cidr x",
				}},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := convertSecurityGroupRulesToProto(tt.in)
			require.Equal(t, tt.expected, out)
		})
	}
}
