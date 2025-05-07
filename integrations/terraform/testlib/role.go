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
	"regexp"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestRoleDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	role := &types.RoleV6{
		Metadata: types.Metadata{
			Name: "test",
		},
	}
	err := role.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertRole(ctx, role)
	require.NoError(s.T(), err)

	name := "data.teleport_role.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v8"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRole() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_role.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckNoResourceAttr(name, "spec.options"),
					resource.TestCheckResourceAttr(name, "version", "v7"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "anonymous"),
				),
			},
			{
				Config:   s.getFixture("role_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "spec.options.forward_agent", "true"),
					resource.TestCheckResourceAttr(name, "spec.options.max_session_ttl", "2h3m"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "known"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.1", "anonymous"),
					resource.TestCheckResourceAttr(name, "spec.allow.request.roles.0", "example"),
					resource.TestCheckResourceAttr(name, "spec.allow.request.claims_to_roles.0.claim", "example"),
					resource.TestCheckResourceAttr(name, "spec.allow.request.claims_to_roles.0.value", "example"),
					resource.TestCheckResourceAttr(name, "spec.allow.request.claims_to_roles.0.roles.0", "example"),
					resource.TestCheckResourceAttr(name, "spec.allow.node_labels.example.0", "yes"),
					resource.TestCheckResourceAttr(name, "spec.allow.node_labels.example.1", "no"),

					resource.TestCheckResourceAttr(name, "version", "v7"),
				),
			},
			{
				Config:   s.getFixture("role_1_update.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_2_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckNoResourceAttr(name, "spec.options"),
					resource.TestCheckResourceAttr(name, "spec.allow.node_labels.example.0", "no"),
					resource.TestCheckResourceAttr(name, "spec.allow.node_labels.sample.0", "yes"),
					resource.TestCheckResourceAttr(name, "spec.allow.node_labels.sample.1", "no"),
				),
			},
			{
				Config:   s.getFixture("role_2_update.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_3_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckNoResourceAttr(name, "spec.options"),
				),
			},
			{
				Config:   s.getFixture("role_3_update.tf"), // Check that there is no state drift
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterpriseWithCache) TestRoleMultipleReviewers() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "test_multiple_reviewers")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_role.test_decrease_reviewers"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_reviewers_0_two_roles.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckNoResourceAttr(name, "spec.options"),
					resource.TestCheckResourceAttr(name, "spec.allow.review_requests.roles.0", "rolea"),
					resource.TestCheckResourceAttr(name, "spec.allow.review_requests.roles.1", "roleb"),
				),
			},
			{
				Config: s.getFixture("role_reviewers_1_one_role.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckNoResourceAttr(name, "spec.options"),
					resource.TestCheckResourceAttr(name, "spec.allow.review_requests.roles.0", "roleb"),
					resource.TestCheckNoResourceAttr(name, "spec.allow.review_requests.roles.1"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportRole() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_role"
	id := "test_import"
	name := r + "." + id

	role := &types.RoleV6{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.RoleSpecV6{},
	}
	err := role.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertRole(ctx, role)
	require.NoError(s.T(), err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "role", state[0].Attributes["kind"])
					require.Equal(s.T(), "test_import", state[0].Attributes["metadata.name"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRoleLoginsSplitBrain() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "splitbrain")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_role.splitbrain"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_drift_0.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v7"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "one"),
				),
			},
			{
				Config:   s.getFixture("role_drift_0.tf"),
				PlanOnly: true,
			},
			{
				// Step to add an extra login
				PreConfig: func() {
					currentRole, err := s.client.GetRole(ctx, "splitbrain")
					require.NoError(s.T(), err)

					logins := currentRole.GetLogins(types.Allow)
					logins = append(logins, "extraOne")
					currentRole.SetLogins(types.Allow, logins)

					_, err = s.client.UpsertRole(ctx, currentRole)
					require.NoError(s.T(), err)
				},
				Config: s.getFixture("role_drift_0.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v7"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "one"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRoleVersionUpgrade() {
	// TODO(hugoShaka) Re-enable this test when we fix the role defaults in v16
	// We had a bug in v14 and below that caused the defaults to be badly computed.
	// We tried to fix this bug in v15 but it was too aggressive (forcing replacement is too destructive).
	// In v16 we'll push a new plan modifier to fix this issue, this might be a
	// breaking change for users who relied on the bug.
	s.T().Skip("Test temporarily disabled until v16")

	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "upgrade")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	var noAccess []types.KubernetesResource

	defaultV5Wildcard := []types.KubernetesResource{
		{
			Kind:      "pod",
			Namespace: types.Wildcard,
			Name:      types.Wildcard,
			Verbs:     []string{types.Wildcard},
		},
	}

	defaultV7Wildcard := []types.KubernetesResource{
		{
			Kind:      types.Wildcard,
			Namespace: types.Wildcard,
			Name:      types.Wildcard,
			Verbs:     []string{types.Wildcard},
		},
	}

	customWildcard := []types.KubernetesResource{
		{
			Kind:      types.KindKubePod,
			Namespace: "myns",
			Name:      types.Wildcard,
			Verbs:     []string{types.Wildcard},
		},
	}

	checkRoleResource := func(version string, expected []types.KubernetesResource) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			role, err := s.client.GetRole(ctx, "upgrade")
			if err != nil {
				return trace.Wrap(err)
			}

			if role.GetVersion() != version {
				return trace.CompareFailed("wrong role version, was expecting %q, got %q", version, role.GetVersion())
			}

			rolev6, ok := (role).(*types.RoleV6)
			if !ok {
				return trace.CompareFailed("failed to convert role to rolve6")
			}

			diff := cmp.Diff(expected, rolev6.Spec.Allow.KubernetesResources, cmpopts.EquateEmpty())
			if diff != "" {
				return trace.CompareFailed("kube resources allow rules differs from expected result: %s", diff)
			}
			return nil
		}
	}

	name := "teleport_role.upgrade"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_upgrade_v4.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v4"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev4"),
					checkRoleResource(types.V4, defaultV5Wildcard),
				),
			},
			{
				Config:   s.getFixture("role_upgrade_v4.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_upgrade_v5.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v5"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev5"),
					checkRoleResource(types.V5, defaultV5Wildcard),
				),
			},
			{
				Config:   s.getFixture("role_upgrade_v5.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_upgrade_v6.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v6"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev6"),
					checkRoleResource(types.V6, noAccess),
				),
			},
			{
				Config:   s.getFixture("role_upgrade_v6.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_with_kube_resources.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v6"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev6"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.kind", "pod"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.name", "*"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.namespace", "myns"),
					checkRoleResource(types.V6, customWildcard),
				),
			},
			{
				Config:   s.getFixture("role_with_kube_resources.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("role_upgrade_v7.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v7"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev7"),
					checkRoleResource(types.V7, defaultV7Wildcard),
				),
			},
			{
				Config:   s.getFixture("role_upgrade_v7.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRoleWithKubernetesResources() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "upgrade")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_role.upgrade"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_with_kube_resources.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v6"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev6"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.kind", "pod"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.name", "*"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.namespace", "myns"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.verbs.0", "*"),
				),
			},
			{
				Config:   s.getFixture("role_with_kube_resources.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRoleWithKubernetesVerbs() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetRole(ctx, "kube_verbs")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_role.kube_verbs"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("role_with_kube_verbs.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "role"),
					resource.TestCheckResourceAttr(name, "version", "v7"),
					resource.TestCheckResourceAttr(name, "spec.allow.logins.0", "onev6"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.kind", "pod"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.name", "*"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.namespace", "myns"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.verbs.0", "get"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.verbs.1", "watch"),
					resource.TestCheckResourceAttr(name, "spec.allow.kubernetes_resources.0.verbs.2", "list"),
				),
			},
			{
				Config:   s.getFixture("role_with_kube_verbs.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestRoleNoVersion() {
	re, err := regexp.Compile(".*The argument \"version\" is required, but no definition was found.*")
	require.NoError(s.T(), err)
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:      s.getFixture("role_no_version.tf"),
				ExpectError: re,
			},
		},
	})
}
