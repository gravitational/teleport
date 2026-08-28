// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testlib

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestStaticHostUser() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.StaticHostUserClient().GetStaticHostUser(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Errorf("expected not found, actual: %v", err)
	}

	name := "teleport_static_host_user.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("static_host_user_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "static_host_user"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.node_labels.0.name", "foo"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.node_labels.0.values.0", "bar"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.groups.0", "foo"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.groups.1", "bar"),
				),
			},
			{
				Config:   s.getFixture("static_host_user_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("static_host_user_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "static_host_user"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.node_labels.0.name", "baz"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.node_labels.0.values.0", "quux"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.groups.0", "baz"),
					resource.TestCheckResourceAttr(name, "spec.matchers.0.groups.1", "quux"),
				),
			},
			{
				Config:   s.getFixture("static_host_user_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportStaticHostUser() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := "teleport_static_host_user"
	id := "test_import"
	name := r + "." + id

	labels := []*labelv1.Label{
		{
			Name:   "foo",
			Values: []string{"bar"},
		},
	}
	groups := []string{"foo", "bar"}

	shu := &userprovisioningv2.StaticHostUser{
		Metadata: &v1.Metadata{
			Name: id,
		},
		Kind:    types.KindStaticHostUser,
		Version: types.V2,
		Spec: &userprovisioningv2.StaticHostUserSpec{
			Matchers: []*userprovisioningv2.Matcher{
				{
					NodeLabels: labels,
					Groups:     groups,
				},
			},
		},
	}
	shu, err := s.client.StaticHostUserClient().CreateStaticHostUser(ctx, shu)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := s.client.StaticHostUserClient().GetStaticHostUser(ctx, shu.GetMetadata().Name)
		return err == nil
	}, 5*time.Second, time.Second)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        fmt.Sprintf("%s\nresource %q %q { }", s.terraformConfig, r, id),
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(t, types.KindStaticHostUser, state[0].Attributes["kind"])
					require.Equal(t, labels[0].Name, state[0].Attributes["spec.matchers.0.node_labels.0.name"])
					require.Equal(t, labels[0].Values[0], state[0].Attributes["spec.matchers.0.node_labels.0.values.0"])
					require.Equal(t, groups[0], state[0].Attributes["spec.matchers.0.groups.0"])
					require.Equal(t, groups[1], state[0].Attributes["spec.matchers.0.groups.1"])

					return nil
				},
			},
		},
	})
}
