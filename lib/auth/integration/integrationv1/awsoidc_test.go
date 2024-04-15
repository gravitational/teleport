/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integrationv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGenerateAWSOIDCToken(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"
	integrationNameWithoutIssuer := "my-integration-without-issuer"
	integrationNameWithIssuer := "my-integration-with-issuer"

	publicURL := "https://example.com"
	s3BucketURI := "s3://mybucket/my-idp"
	s3IssuerURL := "https://mybucket.s3.amazonaws.com/my-idp"

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, publicURL)

	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationNameWithoutIssuer},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)
	_, err = localClient.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	ig, err = types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationNameWithIssuer},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/OpsTeam",
			IssuerS3URI: s3BucketURI,
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

		_, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %T", err)
	})

	t.Run("auth, discovery and proxy can request tokens", func(t *testing.T) {
		for _, allowedRole := range []types.SystemRole{types.RoleAuth, types.RoleDiscovery, types.RoleProxy} {
			ctx = authz.ContextWithUser(ctx, authz.BuiltinRole{
				Role:                  types.RoleInstance,
				AdditionalSystemRoles: []types.SystemRole{allowedRole},
				Username:              string(allowedRole),
				Identity: tlsca.Identity{
					Username: string(allowedRole),
				},
			})

			_, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{})
			require.NoError(t, err)
		}
	})

	ctx = authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:                  types.RoleInstance,
		AdditionalSystemRoles: []types.SystemRole{types.RoleDiscovery},
		Username:              string(types.RoleDiscovery),
		Identity: tlsca.Identity{
			Username: string(types.RoleDiscovery),
		},
	})

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

	t.Run("without integration (old clients)", func(t *testing.T) {
		resp, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{})
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
	})
	t.Run("with integration in rpc call but no issuer defined", func(t *testing.T) {
		resp, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{
			Integration: integrationNameWithoutIssuer,
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
	})
	t.Run("with integration in rpc call and issuer defined", func(t *testing.T) {
		resp, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{
			Integration: integrationNameWithIssuer,
		})
		require.NoError(t, err)

		_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
			RawToken: resp.GetToken(),
			Issuer:   s3IssuerURL,
		})
		require.NoError(t, err)
		// Fails if the issuer is different
		_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
			RawToken: resp.GetToken(),
			Issuer:   publicURL,
		})
		require.Error(t, err)
	})
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

func TestListEICE(t *testing.T) {
	t.Parallel()

	clusterName := "test-cluster"
	proxyPublicAddr := "127.0.0.1.nip.io"
	integrationName := "my-awsoidc-integration"
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, proxyPublicAddr)

	_, err = localClient.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	awsoidService, err := NewAWSOIDCService(&AWSOIDCServiceConfig{
		IntegrationService: resourceSvc,
		Authorizer:         resourceSvc.authorizer,
		Cache:              &mockCache{},
	})
	require.NoError(t, err)

	t.Run("fails when user doesn't have access to integration.use", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		_, err = awsoidService.ListEICE(userCtx, &integrationv1.ListEICERequest{
			Integration: integrationName,
			Region:      "my-region",
			VpcIds:      []string{"vpc-123"},
			NextToken:   "",
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, but got %T", err)
	})
	t.Run("calls awsoidc package when user has access to integration.use/read", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead, types.VerbUse},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		_, err = awsoidService.ListEICE(userCtx, &integrationv1.ListEICERequest{
			Integration: integrationName,
			Region:      "",
			VpcIds:      []string{"vpc-123"},
			NextToken:   "",
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter error, but got %T", err)
	})
}

func TestDeployService(t *testing.T) {
	t.Parallel()

	clusterName := "test-cluster"
	proxyPublicAddr := "127.0.0.1.nip.io"
	integrationName := "my-awsoidc-integration"
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, proxyPublicAddr)

	_, err = localClient.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	awsoidService, err := NewAWSOIDCService(&AWSOIDCServiceConfig{
		IntegrationService: resourceSvc,
		Authorizer:         resourceSvc.authorizer,
		Cache:              &mockCache{},
	})
	require.NoError(t, err)

	t.Run("fails when user doesn't have access to integration.use", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		_, err = awsoidService.DeployService(userCtx, &integrationv1.DeployServiceRequest{
			Integration: integrationName,
			Region:      "my-region",
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, but got %T", err)
	})
	t.Run("calls awsoidc package when user has access to integration.use/read", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead, types.VerbUse},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		_, err = awsoidService.DeployService(userCtx, &integrationv1.DeployServiceRequest{
			Integration: integrationName,
			Region:      "my-region",
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter error, but got %T", err)
	})
}
