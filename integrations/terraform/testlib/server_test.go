/*
Copyright 2024 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestOpenSSHServer() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkServerDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetNode(ctx, defaults.Namespace, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_server.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkServerDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("server_openssh_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHNode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:22"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
				),
			},
			{
				Config:   s.getFixture("server_openssh_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("server_openssh_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHNode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:23"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
				),
			},
			{
				Config:   s.getFixture("server_openssh_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestOpenSSHServerNameless() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkServerDestroyed := func(state *terraform.State) error {
		// The name is a UUID but we can lookup by hostname as well.
		_, err := s.client.GetNode(ctx, defaults.Namespace, "test.local")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_server.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkServerDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("server_openssh_nameless_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHNode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:22"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
				),
			},
			{
				Config:   s.getFixture("server_openssh_nameless_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("server_openssh_nameless_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHNode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:23"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
				),
			},
			{
				Config:   s.getFixture("server_openssh_nameless_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportOpenSSHServer() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_server"
	id := "test_import"
	name := r + "." + id

	server := &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:22",
			Hostname: "foobar",
		},
	}
	err := server.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertNode(ctx, server)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err = s.client.GetNode(ctx, defaults.Namespace, server.GetName())
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(s.T(), err)
		return true
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), types.KindNode, state[0].Attributes["kind"])
					require.Equal(s.T(), types.SubKindOpenSSHNode, state[0].Attributes["sub_kind"])
					require.Equal(s.T(), "127.0.0.1:22", state[0].Attributes["spec.addr"])
					require.Equal(s.T(), "foobar", state[0].Attributes["spec.hostname"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestOpenSSHEICEServer() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkServerDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetNode(ctx, defaults.Namespace, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_server.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkServerDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("server_openssheice_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHEICENode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:22"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.account_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.instance_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.region", "us-east-1"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.vpc_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.integration", "foo"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.subnet_id", "123"),
				),
			},
			{
				Config:   s.getFixture("server_openssheice_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("server_openssheice_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindNode),
					resource.TestCheckResourceAttr(name, "sub_kind", types.SubKindOpenSSHEICENode),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "spec.addr", "127.0.0.1:23"),
					resource.TestCheckResourceAttr(name, "spec.hostname", "test.local"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.account_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.instance_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.region", "us-east-1"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.vpc_id", "123"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.integration", "foo"),
					resource.TestCheckResourceAttr(name, "spec.cloud_metadata.aws.subnet_id", "123"),
				),
			},
			{
				Config:   s.getFixture("server_openssheice_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportOpenSSHEICEServer() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_server"
	id := "test_import"
	name := r + "." + id

	server := &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHEICENode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:22",
			Hostname: "foobar",
			CloudMetadata: &types.CloudMetadata{
				AWS: &types.AWSInfo{
					AccountID:   "123",
					InstanceID:  "123",
					Region:      "us-east-1",
					VPCID:       "123",
					Integration: "foo",
					SubnetID:    "123",
				},
			},
		},
	}
	err := server.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertNode(ctx, server)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err = s.client.GetNode(ctx, defaults.Namespace, server.GetName())
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(s.T(), err)
		return true
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), types.KindNode, state[0].Attributes["kind"])
					require.Equal(s.T(), types.SubKindOpenSSHEICENode, state[0].Attributes["sub_kind"])
					require.Equal(s.T(), "127.0.0.1:22", state[0].Attributes["spec.addr"])
					require.Equal(s.T(), "foobar", state[0].Attributes["spec.hostname"])
					require.Equal(s.T(), "123", state[0].Attributes["spec.cloud_metadata.aws.account_id"])
					require.Equal(s.T(), "123", state[0].Attributes["spec.cloud_metadata.aws.instance_id"])
					require.Equal(s.T(), "us-east-1", state[0].Attributes["spec.cloud_metadata.aws.region"])
					require.Equal(s.T(), "123", state[0].Attributes["spec.cloud_metadata.aws.vpc_id"])
					require.Equal(s.T(), "foo", state[0].Attributes["spec.cloud_metadata.aws.integration"])
					require.Equal(s.T(), "123", state[0].Attributes["spec.cloud_metadata.aws.subnet_id"])
					return nil
				},
			},
		},
	})
}
