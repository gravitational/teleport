/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Config 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

import (
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestIntegration() {
	const (
		resourceType = "teleport_integration"

		createAWSOIDCIntegrationFixture = "integration_0_create_aws_oidc.tf"
		updateAWSOIDCIntegrationFixture = "integration_1_update_aws_oidc.tf"
		awsOIDCResourceName             = resourceType + "." + "aws_oidc"

		createAzureOIDCIntegrationFixture = "integration_2_create_azure_oidc.tf"
		updateAzureOIDCIntegrationFixture = "integration_3_update_azure_oidc.tf"
		azureOIDCResourceName             = resourceType + "." + "azure_oidc"

		createAWSRAIntegrationFixture = "integration_4_create_aws_ra.tf"
		updateAWSRAIntegrationFixture = "integration_5_update_aws_ra.tf"
		awsRAResourceName             = resourceType + "." + "aws_ra"
	)
	t := s.T()
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy: func(state *terraform.State) error {
			integrations, err := s.client.ListAllIntegrations(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					return nil
				}
				return err
			}
			for _, ig := range integrations {
				switch ig.GetName() {
				case "aws-oidc", "azure-oidc", "aws-ra":
					return trace.Errorf("created integrations still exist after destroy: %v", types.ResourceNames(integrations))
				default:
				}
			}
			return nil
		},
		IsUnitTest: true,
		Steps: []resource.TestStep{
			// aws subkind steps
			{
				Config: s.getFixture(createAWSOIDCIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(awsOIDCResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "sub_kind", "aws-oidc"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.name", "aws-oidc"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckNoResourceAttr(awsOIDCResourceName, "spec.aws_oidc.audience"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "spec.aws_oidc.issuer_s3_uri", "s3://test-s3-bucket/some-prefix"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "spec.aws_oidc.role_arn", "arn:aws:iam::123456789012:role/test-role"),
					resource.TestCheckNoResourceAttr(awsOIDCResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(createAWSOIDCIntegrationFixture),
				PlanOnly: true,
			},
			{
				Config: s.getFixture(updateAWSOIDCIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(awsOIDCResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "sub_kind", "aws-oidc"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.name", "aws-oidc"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "spec.aws_oidc.audience", "aws-identity-center"),
					resource.TestCheckNoResourceAttr(awsOIDCResourceName, "spec.aws_oidc.issuer_s3_uri"),
					resource.TestCheckResourceAttr(awsOIDCResourceName, "spec.aws_oidc.role_arn", "arn:aws:iam::123456789012:role/test-role-updated"),
					resource.TestCheckNoResourceAttr(awsOIDCResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(updateAWSOIDCIntegrationFixture),
				PlanOnly: true,
			},
			// azure subkind steps
			{
				Config: s.getFixture(createAzureOIDCIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(azureOIDCResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "sub_kind", "azure-oidc"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.name", "azure-oidc"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "spec.azure_oidc.tenant_id", "some-tenant"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "spec.azure_oidc.client_id", "some-client"),
					resource.TestCheckNoResourceAttr(azureOIDCResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(createAzureOIDCIntegrationFixture),
				PlanOnly: true,
			},
			{
				Config: s.getFixture(updateAzureOIDCIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(azureOIDCResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "sub_kind", "azure-oidc"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.name", "azure-oidc"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "spec.azure_oidc.tenant_id", "updated-tenant"),
					resource.TestCheckResourceAttr(azureOIDCResourceName, "spec.azure_oidc.client_id", "updated-client"),
					resource.TestCheckNoResourceAttr(azureOIDCResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(updateAzureOIDCIntegrationFixture),
				PlanOnly: true,
			},
			// aws-ra subkind steps
			{
				Config: s.getFixture(createAWSRAIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(awsRAResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(awsRAResourceName, "sub_kind", "aws-ra"),
					resource.TestCheckResourceAttr(awsRAResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.name", "aws-ra"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.enabled", "true"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_accepts_role_session_name", "true"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_arn", "arn:aws:rolesanywhere:us-east-1:123456789012:profile/test-profile"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_name_filters.0", "*llama*"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_name_filters.1", "^.*$"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.role_arn", "arn:aws:iam::123456789012:role/test-role"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.trust_anchor_arn", "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/test-anchor"),
					resource.TestCheckNoResourceAttr(awsRAResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(createAWSRAIntegrationFixture),
				PlanOnly: true,
			},
			{
				Config: s.getFixture(updateAWSRAIntegrationFixture),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(awsRAResourceName, "kind", "integration"),
					resource.TestCheckResourceAttr(awsRAResourceName, "sub_kind", "aws-ra"),
					resource.TestCheckResourceAttr(awsRAResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.name", "aws-ra"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.description", "Test integration"),
					resource.TestCheckResourceAttr(awsRAResourceName, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.enabled", "false"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_accepts_role_session_name", "false"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_arn", "arn:aws:rolesanywhere:us-east-1:123456789012:profile/updated"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_name_filters.0", "*updated*"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.profile_name_filters.1", "^.*$"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.profile_sync_config.role_arn", "arn:aws:iam::123456789012:role/updated"),
					resource.TestCheckResourceAttr(awsRAResourceName, "spec.aws_ra.trust_anchor_arn", "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/updated"),
					resource.TestCheckNoResourceAttr(awsRAResourceName, "spec.credentials"),
				),
			},
			{
				Config:   s.getFixture(updateAWSRAIntegrationFixture),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportIntegration() {
	const resourceType = "teleport_integration"
	t := s.T()
	ctx := t.Context()

	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(types.Metadata{
		Name:   "aws-oidc-import-me",
		Labels: map[string]string{"foo": "bar"},
	}, &types.AWSOIDCIntegrationSpecV1{
		Audience:    "aws-identity-center",
		IssuerS3URI: "s3://test-s3-bucket/some-prefix",
		RoleARN:     "arn:aws:iam::123456789012:role/test-role",
	})
	require.NoError(t, err)

	azureOIDCIntegration, err := types.NewIntegrationAzureOIDC(types.Metadata{
		Name:   "azure-oidc-import-me",
		Labels: map[string]string{"foo": "bar"},
	}, &types.AzureOIDCIntegrationSpecV1{
		ClientID: "a-client-id",
		TenantID: "a-tenant-id",
	})
	require.NoError(t, err)

	awsRAIntegration, err := types.NewIntegrationAWSRA(types.Metadata{
		Name:   "aws-ra-import-me",
		Labels: map[string]string{"foo": "bar"},
	}, &types.AWSRAIntegrationSpecV1{
		ProfileSyncConfig: &types.AWSRolesAnywhereProfileSyncConfig{
			Enabled:                       true,
			ProfileARN:                    "arn:aws:rolesanywhere:us-east-1:123456789012:profile/test",
			ProfileAcceptsRoleSessionName: true,
			RoleARN:                       "arn:aws:iam::123456789012:role/test",
			ProfileNameFilters:            []string{"foo", "bar"},
		},
		TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/test",
	})
	require.NoError(t, err)

	var created []types.Integration
	for _, ig := range []types.Integration{awsOIDCIntegration, azureOIDCIntegration, awsRAIntegration} {
		ig, err := s.client.CreateIntegration(ctx, ig)
		require.NoError(t, err)
		created = append(created, ig)
	}
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		existing, err := s.client.ListAllIntegrations(ctx)
		require.NoError(t, err)
		require.Subset(t, existing, created)
	}, 5*time.Second, 200*time.Millisecond, "waiting for all created integrations to be available")

	tests := []struct {
		integration types.Integration
		stateCheck  func([]*terraform.InstanceState) error
	}{
		{
			integration: awsOIDCIntegration,
			stateCheck: func(state []*terraform.InstanceState) error {
				require.Equal(t, "aws-oidc", state[0].Attributes["sub_kind"])
				require.Equal(t, "aws-oidc-import-me", state[0].Attributes["metadata.name"])
				require.Equal(t, "aws-identity-center", state[0].Attributes["spec.aws_oidc.audience"])
				require.Equal(t, "s3://test-s3-bucket/some-prefix", state[0].Attributes["spec.aws_oidc.issuer_s3_uri"])
				require.Equal(t, "arn:aws:iam::123456789012:role/test-role", state[0].Attributes["spec.aws_oidc.role_arn"])
				return nil
			},
		},
		{
			integration: azureOIDCIntegration,
			stateCheck: func(state []*terraform.InstanceState) error {
				require.Equal(t, "azure-oidc", state[0].Attributes["sub_kind"])
				require.Equal(t, "a-tenant-id", state[0].Attributes["spec.azure_oidc.tenant_id"])
				require.Equal(t, "a-client-id", state[0].Attributes["spec.azure_oidc.client_id"])
				return nil
			},
		},
		{
			integration: awsRAIntegration,
			stateCheck: func(state []*terraform.InstanceState) error {
				require.Equal(t, "aws-ra", state[0].Attributes["sub_kind"])
				require.Equal(t, "true", state[0].Attributes["spec.aws_ra.profile_sync_config.enabled"])
				require.Equal(t, "true", state[0].Attributes["spec.aws_ra.profile_sync_config.profile_accepts_role_session_name"])
				require.Equal(t, "arn:aws:rolesanywhere:us-east-1:123456789012:profile/test", state[0].Attributes["spec.aws_ra.profile_sync_config.profile_arn"])
				require.Equal(t, "foo", state[0].Attributes["spec.aws_ra.profile_sync_config.profile_name_filters.0"])
				require.Equal(t, "bar", state[0].Attributes["spec.aws_ra.profile_sync_config.profile_name_filters.1"])
				require.Equal(t, "arn:aws:iam::123456789012:role/test", state[0].Attributes["spec.aws_ra.profile_sync_config.role_arn"])
				require.Equal(t, "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/test", state[0].Attributes["spec.aws_ra.trust_anchor_arn"])
				return nil
			},
		},
	}

	var steps []resource.TestStep
	for _, test := range tests {
		steps = append(steps, resource.TestStep{
			Config:        s.terraformConfig + "\n" + fmt.Sprintf(`resource %q %q {}`, resourceType, test.integration.GetName()),
			ResourceName:  resourceType + "." + test.integration.GetName(),
			ImportState:   true,
			ImportStateId: test.integration.GetName(),
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				// common checks
				require.Equal(t, "integration", state[0].Attributes["kind"])
				require.Equal(t, "v1", state[0].Attributes["version"])
				require.Equal(t, test.integration.GetName(), state[0].Attributes["metadata.name"])
				require.Equal(t, "bar", state[0].Attributes["metadata.labels.foo"])
				// checks that are specific to each integration
				return test.stateCheck(state)
			},
		})
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps:                    steps,
	})
}
