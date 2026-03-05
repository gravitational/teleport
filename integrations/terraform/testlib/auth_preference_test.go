/*
Copyright 2015-2021 Gravitational, Inc.

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

package testlib

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestAuthPreferenceDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	authPreference := &types.AuthPreferenceV2{
		Metadata: types.Metadata{
			Name: "test",
		},
		Spec: types.AuthPreferenceSpecV2{
			Type:                       constants.Local,
			ConnectorName:              constants.LocalConnector,
			SecondFactor:               constants.SecondFactorOTP,
			AllowLocalAuth:             types.NewBoolOption(true),
			AllowBrowserAuthentication: types.NewBoolOption(false),
			MessageOfTheDay:            "test message",
			LockingMode:                constants.LockingModeBestEffort,
			IDP: &types.IdPOptions{
				SAML: &types.IdPSAMLOptions{
					Enabled: types.NewBoolOption(false),
				},
			},
			Webauthn: &types.Webauthn{
				RPID: "example.com",
			},
		},
	}

	err := authPreference.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	authPreferencesBefore, err := s.client.GetAuthPreference(ctx)
	require.NoError(s.T(), err)

	_, err = s.client.ClusterConfigClient().UpsertAuthPreference(ctx, &clusterconfigv1.UpsertAuthPreferenceRequest{AuthPreference: authPreference})
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		authPreferencesCurrent, err := s.client.GetAuthPreference(ctx)
		require.NoError(s.T(), err)

		return authPreferencesBefore.GetMetadata().Revision != authPreferencesCurrent.GetMetadata().Revision
	}, 5*time.Second, time.Second)

	name := "data.teleport_auth_preference.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("auth_preference_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_auth_preference"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.allow_local_auth", "true"),
					resource.TestCheckResourceAttr(name, "spec.allow_browser_authentication", "false"),
					resource.TestCheckResourceAttr(name, "spec.allow_headless", "false"),
					resource.TestCheckResourceAttr(name, "spec.idp.saml.enabled", "false"),
					resource.TestCheckResourceAttr(name, "spec.locking_mode", "best_effort"),
					resource.TestCheckResourceAttr(name, "spec.message_of_the_day", "test message"),
					resource.TestCheckResourceAttr(name, "spec.second_factor", "otp"),
					resource.TestCheckResourceAttr(name, "spec.type", "local"),
					resource.TestCheckResourceAttr(name, "spec.webauthn.rp_id", "example.com"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestAuthPreference() {
	name := "teleport_auth_preference.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("auth_preference_0_set.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_auth_preference"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.disconnect_expired_cert", "true"),
				),
			},
			{
				Config:   s.getFixture("auth_preference_0_set.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("auth_preference_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_auth_preference"),
					resource.TestCheckResourceAttr(name, "spec.disconnect_expired_cert", "false"),
				),
			},
			{
				Config:   s.getFixture("auth_preference_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportAuthPreference() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_auth_preference"
	id := "test_import"
	name := r + "." + id

	authPreference := &types.AuthPreferenceV2{
		Metadata: types.Metadata{},
		Spec: types.AuthPreferenceSpecV2{
			DisconnectExpiredCert: types.NewBoolOption(true),
		},
	}
	err := authPreference.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	authPreferencesBefore, err := s.client.GetAuthPreference(ctx)
	require.NoError(s.T(), err)

	_, err = s.client.ClusterConfigClient().UpsertAuthPreference(ctx, &clusterconfigv1.UpsertAuthPreferenceRequest{AuthPreference: authPreference})
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		authPreferencesCurrent, err := s.client.GetAuthPreference(ctx)
		require.NoError(s.T(), err)

		return authPreferencesBefore.GetMetadata().Revision != authPreferencesCurrent.GetMetadata().Revision
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "cluster_auth_preference", state[0].Attributes["kind"])
					require.Equal(s.T(), "true", state[0].Attributes["spec.disconnect_expired_cert"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSWithCache) TestAuthPreferenceAddLabel() {
	name := "teleport_auth_preference.cluster_auth_preference"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("auth_preference_0_cluster.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_auth_preference"),
					resource.TestCheckNoResourceAttr(name, "metadata.labels"),
					resource.TestCheckResourceAttr(name, "spec.type", "oidc"),
				),
			},
			{
				Config:   s.getFixture("auth_preference_0_cluster.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("auth_preference_1_cluster.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_auth_preference"),
					resource.TestCheckResourceAttr(name, "metadata.labels.provisioner", "terraform"),
					resource.TestCheckResourceAttr(name, "spec.type", "oidc"),
				),
			},
			{
				Config:   s.getFixture("auth_preference_1_cluster.tf"),
				PlanOnly: true,
			},
		},
	})
}
