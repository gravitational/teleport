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

	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestScopedRole() {
	t := s.T()
	ctx := t.Context()

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetScopedRole(ctx, "test-scoped-role")
		if !trace.IsNotFound(err) {
			return trace.Errorf("expected not found, actual: %v", err)
		}
		return nil
	}

	name := "teleport_scoped_role.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("scoped_role_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindScopedRole),
					resource.TestCheckResourceAttr(name, "scope", "/staging"),
					resource.TestCheckResourceAttr(name, "spec.assignable_scopes.0", "/staging/aa"),
					resource.TestCheckResourceAttr(name, "spec.rules.0.resources.0", "scoped_token"),
					resource.TestCheckResourceAttr(name, "spec.rules.0.verbs.0", "read"),
					resource.TestCheckResourceAttr(name, "spec.rules.0.verbs.1", "list"),
				),
			},
			{
				Config:   s.getFixture("scoped_role_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("scoped_role_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "metadata.labels.env", "staging"),
					resource.TestCheckResourceAttr(name, "spec.assignable_scopes.0", "/staging/aa"),
					resource.TestCheckResourceAttr(name, "spec.assignable_scopes.1", "/staging/bb"),
					resource.TestCheckResourceAttr(name, "spec.rules.0.verbs.2", "create"),
					resource.TestCheckResourceAttr(name, "spec.ssh.logins.0", "root"),
				),
			},
			{
				Config:   s.getFixture("scoped_role_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportScopedRole() {
	t := s.T()
	ctx := t.Context()

	r := "teleport_scoped_role"
	id := "test_import_scoped_role"
	name := r + "." + id

	role := &accessv1.ScopedRole{
		Kind:    types.KindScopedRole,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Scope: "/staging",
		Spec: &accessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/staging/aa"},
			Rules: []*accessv1.ScopedRule{
				{
					Resources: []string{"scoped_token"},
					Verbs:     []string{"read", "list"},
				},
			},
		},
	}

	_, err := s.client.CreateScopedRole(ctx, role)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := s.client.GetScopedRole(ctx, id)
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
					require.Equal(t, types.KindScopedRole, state[0].Attributes["kind"])
					require.Equal(t, "/staging", state[0].Attributes["scope"])
					require.Equal(t, "/staging/aa", state[0].Attributes["spec.assignable_scopes.0"])
					return nil
				},
			},
		},
	})
}
