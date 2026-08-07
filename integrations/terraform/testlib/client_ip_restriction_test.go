/*
Copyright 2026 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	clientiprestrictionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// SetupSuite grants the terraform test user access to client_ip_restriction and
// builds a user-authenticated client for direct API calls.
//
// The preset Terraform provider role does not include this Cloud-only resource,
// so we add the rule here rather than relying on the preset. The client_ip_restriction
// API also rejects non-user identities, so the suite's builtin-admin client
// (s.client) cannot call it; we build a client authenticated as the terraform
// test user instead.
func (s *TerraformSuiteEnterpriseCloud) SetupSuite() {
	s.TerraformBaseSuite.SetupSuite()

	ctx := context.Background()
	role, err := s.client.GetRole(ctx, teleport.PresetTerraformProviderRoleName)
	require.NoError(s.T(), err)

	rules := role.GetRules(types.Allow)
	rules = append(rules, types.NewRule(types.KindClientIPRestriction, []string{
		types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete,
	}))
	role.SetRules(types.Allow, rules)

	_, err = s.client.UpsertRole(ctx, role)
	require.NoError(s.T(), err)

	userClient, err := client.New(ctx, client.Config{
		Addrs:       []string{s.teleportConfig.Addr},
		Credentials: []client.Credentials{client.LoadIdentityFile(s.teleportConfig.Identity)},
	})
	require.NoError(s.T(), err)
	s.userClient = userClient
	s.T().Cleanup(func() { userClient.Close() })
}

func (s *TerraformSuiteEnterpriseCloud) TestClientIPRestriction() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(_ *terraform.State) error {
		_, err := s.userClient.GetClientIPRestriction(ctx)
		if trace.IsNotFound(err) {
			return nil
		}
		return err
	}

	name := "teleport_client_ip_restriction.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("client_ip_restriction_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindClientIPRestriction),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.#", "1"),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.0", "10.0.0.0/8"),
				),
			},
			{
				Config:   s.getFixture("client_ip_restriction_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("client_ip_restriction_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindClientIPRestriction),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.#", "2"),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.0", "10.0.0.0/8"),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.1", "192.168.0.0/16"),
				),
			},
			{
				Config:   s.getFixture("client_ip_restriction_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterpriseCloud) TestClientIPRestrictionDataSource() {
	name := "data.teleport_client_ip_restriction.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("client_ip_restriction_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindClientIPRestriction),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", types.MetaNameClientIPRestriction),
					resource.TestCheckResourceAttr(name, "spec.allowed_cidrs.0", "10.0.0.0/8"),
				),
			},
		},
	})
}

func (s *TerraformSuiteEnterpriseCloud) TestImportClientIPRestriction() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_client_ip_restriction"
	id := "test_import"
	name := r + "." + id

	// Seed the singleton via the API so there is something to import. Upsert is
	// used so the test is robust to leftover state from earlier steps.
	cir := clientiprestrictionv1.ClientIPRestriction_builder{
		Kind:    types.KindClientIPRestriction,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: types.MetaNameClientIPRestriction,
		}.Build(),
		Spec: clientiprestrictionv1.ClientIPRestrictionSpec_builder{
			AllowedCidrs: []string{"10.0.0.0/8"},
		}.Build(),
	}.Build()
	_, err := s.userClient.UpsertClientIPRestriction(ctx, cir)
	require.NoError(s.T(), err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: types.MetaNameClientIPRestriction,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), types.KindClientIPRestriction, state[0].Attributes["kind"])
					require.Equal(s.T(), "10.0.0.0/8", state[0].Attributes["spec.allowed_cidrs.0"])
					return nil
				},
			},
		},
	})
}
