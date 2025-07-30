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
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/awsra"
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

func TestAWSRolesAnywhereProfileSyncFilters(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"
	proxyPublicAddr := "example.com:443"

	ca := newCertAuthority(t, types.AWSRACA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, proxyPublicAddr)

	ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{
			{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbCreate, types.VerbUpdate}},
		}},
	}, localClient)

	integrationWithFilters := func(t *testing.T, integrationName string, filters []string) *types.IntegrationV1 {
		ig, err := types.NewIntegrationAWSRA(
			types.Metadata{Name: integrationName},
			&types.AWSRAIntegrationSpecV1{
				TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
				ProfileSyncConfig: &types.AWSRolesAnywhereProfileSyncConfig{
					Enabled:            true,
					ProfileARN:         "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
					RoleARN:            "arn:aws:iam::123456789012:role/SyncRole",
					ProfileNameFilters: filters,
				},
			},
		)
		require.NoError(t, err)
		return ig
	}

	t.Run("valid without filters", func(t *testing.T) {
		ig := integrationWithFilters(t, "integration1", []string{})
		_, err := resourceSvc.CreateIntegration(ctx, &integrationv1.CreateIntegrationRequest{
			Integration: ig,
		})
		require.NoError(t, err)
	})

	t.Run("valid with glob filter", func(t *testing.T) {
		ig := integrationWithFilters(t, "integration2", []string{"MyTeam-*"})
		_, err := resourceSvc.CreateIntegration(ctx, &integrationv1.CreateIntegrationRequest{
			Integration: ig,
		})
		require.NoError(t, err)
	})

	t.Run("valid with regex filter", func(t *testing.T) {
		ig := integrationWithFilters(t, "integration3", []string{"^MyTeam-.*$"})
		_, err := resourceSvc.CreateIntegration(ctx, &integrationv1.CreateIntegrationRequest{
			Integration: ig,
		})
		require.NoError(t, err)
	})

	t.Run("invalid with invalid regex", func(t *testing.T) {
		ig := integrationWithFilters(t, "integration5", []string{`^[invalid-regex{$`})
		_, err := resourceSvc.CreateIntegration(ctx, &integrationv1.CreateIntegrationRequest{
			Integration: ig,
		})
		require.Error(t, err)
	})
}

func TestAWSRolesAnywherePing(t *testing.T) {
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
			ProfileSyncConfig: &types.AWSRolesAnywhereProfileSyncConfig{
				Enabled:    false,
				ProfileARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
				RoleARN:    "arn:aws:iam::123456789012:role/SyncRole",
			},
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

	exampleProfile := ratypes.ProfileDetail{
		Name:                  aws.String("ExampleProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
		RoleArns: []string{
			"arn:aws:iam::123456789012:role/ExampleRole",
			"arn:aws:iam::123456789012:role/SyncRole",
		},
	}

	awsRolesAnywhereService, err := NewAWSRolesAnywhereService(&AWSRolesAnywhereServiceConfig{
		IntegrationService: resourceSvc,
		Authorizer:         resourceSvc.authorizer,
		Clock:              resourceSvc.clock,
		Logger:             resourceSvc.logger,
		newPingClient: func(ctx context.Context, req *awsra.AWSClientConfig) (awsra.PingClient, error) {
			return &mockPingClient{
				accountID: "123456789012",
				profiles: []ratypes.ProfileDetail{
					exampleProfile,
				},
			}, nil
		},
	})
	require.NoError(t, err)

	t.Run("test connection using an integration", func(t *testing.T) {
		pingResp, err := awsRolesAnywhereService.AWSRolesAnywherePing(ctx, &integrationv1.AWSRolesAnywherePingRequest{
			Mode: &integrationv1.AWSRolesAnywherePingRequest_Integration{
				Integration: integrationName,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, pingResp)
		require.Equal(t, "123456789012", pingResp.AccountId)
		require.Equal(t, int32(1), pingResp.ProfileCount)
	})

	t.Run("test connection using provided trust anchor", func(t *testing.T) {
		pingResp, err := awsRolesAnywhereService.AWSRolesAnywherePing(ctx, &integrationv1.AWSRolesAnywherePingRequest{
			Mode: &integrationv1.AWSRolesAnywherePingRequest_Custom{
				Custom: &integrationv1.AWSRolesAnywherePingRequestWithoutIntegration{
					TrustAnchorArn: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
					RoleArn:        "arn:aws:iam::123456789012:role/SyncRole",
					ProfileArn:     "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, pingResp)
		require.Equal(t, "123456789012", pingResp.AccountId)
		require.Equal(t, int32(1), pingResp.ProfileCount)
	})
}

type mockPingClient struct {
	accountID string
	profiles  []ratypes.ProfileDetail
}

func (m *mockPingClient) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	return &rolesanywhere.ListProfilesOutput{
		Profiles:  m.profiles,
		NextToken: nil,
	}, nil
}

func (m *mockPingClient) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: []ratypes.Tag{
			{Key: aws.String("MyTagKey"), Value: aws.String("my-tag-value")},
		},
	}, nil
}

func (m *mockPingClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(m.accountID),
		Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
		UserId:  aws.String("USERID1234567890"),
	}, nil
}
