// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestScopedToken() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetScopedToken(ctx, "test-scoped-token", false)
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}
		return nil
	}

	name := "teleport_scoped_token.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("scoped_token_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindScopedToken),
					resource.TestCheckResourceAttr(name, "scope", "/staging/aa"),
					resource.TestCheckResourceAttr(name, "spec.assigned_scope", "/staging/aa/nodes"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "token"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.usage_mode", "unlimited"),
				),
			},
			{
				Config:   s.getFixture("scoped_token_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("scoped_token_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "metadata.labels.env", "staging"),
					resource.TestCheckResourceAttr(name, "spec.assigned_scope", "/staging/aa/nodes"),
				),
			},
			{
				Config:   s.getFixture("scoped_token_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportScopedToken() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_scoped_token"
	id := "test_import_scoped_token"
	name := r + "." + id

	token := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Scope: "/staging",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/staging/nodes",
			JoinMethod:    "token",
			Roles:         []string{"Node"},
			UsageMode:     "unlimited",
		},
	}

	_, err := s.client.CreateScopedToken(ctx, token)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := s.client.GetScopedToken(ctx, id, true)
		require.NoError(t, err)
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
					require.Equal(t, types.KindScopedToken, state[0].Attributes["kind"])
					require.Equal(t, "/staging", state[0].Attributes["scope"])
					require.Equal(t, "/staging/nodes", state[0].Attributes["spec.assigned_scope"])
					return nil
				},
			},
		},
	})
}
