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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
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

		_, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{Integration: integrationNameWithoutIssuer})
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

			_, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{Integration: integrationNameWithoutIssuer})
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

	publicKey, err := keys.ParsePublicKey(jwtPubKey)
	require.NoError(t, err)

	// Validate JWT against public key
	key, err := jwt.New(&jwt.Config{
		ClusterName: clusterName,
		Clock:       resourceSvc.clock,
		PublicKey:   publicKey,
	})
	require.NoError(t, err)

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

func TestRBAC(t *testing.T) {
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
		IntegrationService:    resourceSvc,
		Authorizer:            resourceSvc.authorizer,
		ProxyPublicAddrGetter: func() string { return "128.0.0.1" },
		Cache:                 &mockCache{},
	})
	require.NoError(t, err)

	type endpointSubtest struct {
		name string
		fn   func() error
	}
	t.Run("fails when user doesn't have access to integration.use", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		for _, tt := range []endpointSubtest{
			{
				name: "ListDatabases",
				fn: func() error {
					_, err := awsoidService.ListDatabases(userCtx, &integrationv1.ListDatabasesRequest{
						Integration: integrationName,
						Region:      "",
						RdsType:     "",
						Engines:     []string{},
						NextToken:   "",
						VpcId:       "vpc-123",
					})
					return err
				},
			},
			{
				name: "EnrollEKSClusters",
				fn: func() error {
					_, err := awsoidService.EnrollEKSClusters(userCtx, &integrationv1.EnrollEKSClustersRequest{
						Integration:     integrationName,
						Region:          "my-region",
						EksClusterNames: []string{"EKS1"},
						AgentVersion:    "10.0.0",
					})
					return err
				},
			},
			{
				name: "DeployService",
				fn: func() error {
					_, err = awsoidService.DeployService(userCtx, &integrationv1.DeployServiceRequest{
						Integration: integrationName,
						Region:      "my-region",
					})
					return err
				},
			},
			{
				name: "ListSubnets",
				fn: func() error {
					_, err := awsoidService.ListSubnets(userCtx, &integrationv1.ListSubnetsRequest{
						Integration: integrationName,
						Region:      "my-region",
						VpcId:       "vpc-1",
					})
					return err
				},
			},
			{
				name: "ListVPCs",
				fn: func() error {
					_, err := awsoidService.ListVPCs(userCtx, &integrationv1.ListVPCsRequest{
						Integration: integrationName,
						Region:      "my-region",
					})
					return err
				},
			},
			{
				name: "Ping",
				fn: func() error {
					_, err := awsoidService.Ping(userCtx, &integrationv1.PingRequest{
						Integration: integrationName,
					})
					return err
				},
			},
			{
				name: "Ping with arn",
				fn: func() error {
					_, err := awsoidService.Ping(userCtx, &integrationv1.PingRequest{
						Integration: integrationName,
						RoleArn:     "some-arn",
					})
					return err
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.fn()
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, but got %T", err)
			})
		}
	})

	t.Run("calls awsoidc package when user has access to integration.use/read", func(t *testing.T) {
		role := types.RoleSpecV6{
			Allow: types.RoleConditions{Rules: []types.Rule{{
				Resources: []string{types.KindIntegration},
				Verbs:     []string{types.VerbRead, types.VerbUse},
			}}},
		}

		userCtx := authorizerForDummyUser(t, ctx, role, localClient)

		for _, tt := range []endpointSubtest{
			{
				name: "ListDatabases",
				fn: func() error {
					_, err := awsoidService.ListDatabases(userCtx, &integrationv1.ListDatabasesRequest{
						Integration: integrationName,
						Region:      "",
						RdsType:     "",
						Engines:     []string{},
						NextToken:   "",
						VpcId:       "vpc-123",
					})
					return err
				},
			},
			{
				name: "EnrollEKSClusters",
				fn: func() error {
					_, err := awsoidService.EnrollEKSClusters(userCtx, &integrationv1.EnrollEKSClustersRequest{
						Integration:     integrationName,
						Region:          "my-region",
						EksClusterNames: []string{"EKS1"},
						AgentVersion:    "10.0.0",
					})
					return err
				},
			},
			{
				name: "DeployService",
				fn: func() error {
					_, err = awsoidService.DeployService(userCtx, &integrationv1.DeployServiceRequest{
						Integration: integrationName,
						Region:      "my-region",
					})
					return err
				},
			},
			{
				name: "ListSubnets",
				fn: func() error {
					_, err := awsoidService.ListSubnets(userCtx, &integrationv1.ListSubnetsRequest{
						Integration: integrationName,
						Region:      "my-region",
						VpcId:       "vpc-1",
					})
					return err
				},
			},
			{
				name: "ListVPCs",
				fn: func() error {
					_, err := awsoidService.ListVPCs(userCtx, &integrationv1.ListVPCsRequest{
						Integration: integrationName,
						Region:      "my-region",
					})
					return err
				},
			},
			{
				name: "Ping",
				fn: func() error {
					_, err := awsoidService.Ping(userCtx, &integrationv1.PingRequest{})
					return err
				},
			},
			{
				name: "ListDeployedDatabaseServices",
				fn: func() error {
					_, err := awsoidService.ListDeployedDatabaseServices(userCtx, &integrationv1.ListDeployedDatabaseServicesRequest{
						Integration: integrationName,
						Region:      "my-region",
					})
					return err
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.fn()
				require.True(t, trace.IsBadParameter(err), "expected BadParameter error, but got %T", err)
			})
		}
	})
}

func TestConvertEKSCluster(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    awsoidc.EKSCluster
		expected *integrationv1.EKSCluster
	}{
		{
			name: "valid",
			input: awsoidc.EKSCluster{
				Name:                 "my-cluster",
				Region:               "us-east-1",
				Arn:                  "my-arn",
				Labels:               map[string]string{},
				JoinLabels:           map[string]string{},
				Status:               "ACTIVE",
				AuthenticationMode:   "API",
				EndpointPublicAccess: true,
			},
			expected: &integrationv1.EKSCluster{
				Name:                 "my-cluster",
				Region:               "us-east-1",
				Arn:                  "my-arn",
				Labels:               map[string]string{},
				JoinLabels:           map[string]string{},
				Status:               "ACTIVE",
				AuthenticationMode:   "API",
				EndpointPublicAccess: true,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, convertEKSCluster(tt.input))
		})
	}
}
