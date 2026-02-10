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

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	crownJewelCreateQuery = "resources.where(kind == \"db\")"
	crownJewelUpdateQuery = "resources.where(kind == \"app\")"
)

func (s *TerraformSuiteOSS) resetCrownJewels(ctx context.Context) {
	nextToken := ""
	for {
		jewels, token, err := s.client.CrownJewelServiceClient().ListCrownJewels(ctx, 1000, nextToken)
		require.NoError(s.T(), err)

		for _, jewel := range jewels {
			err := s.client.CrownJewelServiceClient().DeleteCrownJewel(ctx, jewel.GetMetadata().GetName())
			if trace.IsNotFound(err) {
				continue
			}
			require.NoError(s.T(), err)
		}

		if token == "" {
			return
		}
		nextToken = token
	}
}

func (s *TerraformSuiteOSS) TestCrownJewelDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetCrownJewels(ctx)
	s.T().Cleanup(func() { s.resetCrownJewels(ctx) })

	_, err := s.client.CrownJewelServiceClient().CreateCrownJewel(ctx, &crownjewelv1.CrownJewel{
		Kind:    types.KindCrownJewel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Spec: &crownjewelv1.CrownJewelSpec{
			Query: crownJewelCreateQuery,
		},
	})
	require.NoError(s.T(), err)

	name := "data.teleport_crown_jewel.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("crown_jewel_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "crown_jewel"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", "test"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "spec.query", crownJewelCreateQuery),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestCrownJewel() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetCrownJewels(ctx)
	s.T().Cleanup(func() { s.resetCrownJewels(ctx) })

	name := "teleport_crown_jewel.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("crown_jewel_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "crown_jewel"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "id", "test"),
					resource.TestCheckResourceAttr(name, "spec.query", crownJewelCreateQuery),
				),
			},
			{
				Config:   s.getFixture("crown_jewel_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("crown_jewel_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "crown_jewel"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "id", "test"),
					resource.TestCheckResourceAttr(name, "spec.query", crownJewelUpdateQuery),
				),
			},
			{
				Config:   s.getFixture("crown_jewel_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportCrownJewel() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetCrownJewels(ctx)
	s.T().Cleanup(func() { s.resetCrownJewels(ctx) })

	r := "teleport_crown_jewel"
	id := "test_import"
	name := r + "." + id

	_, err := s.client.CrownJewelServiceClient().CreateCrownJewel(ctx, &crownjewelv1.CrownJewel{
		Kind:    types.KindCrownJewel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &crownjewelv1.CrownJewelSpec{
			Query: crownJewelUpdateQuery,
		},
	})
	require.NoError(s.T(), err)

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
					require.Equal(s.T(), "crown_jewel", state[0].Attributes["kind"])
					require.Equal(s.T(), "v1", state[0].Attributes["version"])
					require.Equal(s.T(), id, state[0].Attributes["metadata.name"])
					require.Equal(s.T(), id, state[0].Attributes["id"])
					require.Equal(s.T(), crownJewelUpdateQuery, state[0].Attributes["spec.query"])

					return nil
				},
			},
		},
	})
}
