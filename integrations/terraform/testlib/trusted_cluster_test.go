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
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authcatest"
)

func (s *TerraformSuiteOSS) TestTrustedClusterDataSource() {
	trustedCluster := s.createTrustedClusterForTest(s.T().Context(), "test-data-source")
	s.T().Cleanup(func() {
		err := s.client.DeleteTrustedCluster(s.T().Context(), trustedCluster.GetName())
		require.True(s.T(), err == nil || trace.IsNotFound(err), "unexpected cleanup error: %v", err)
	})

	name := "data.teleport_trusted_cluster.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("trusted_cluster_data_source.tf", trustedCluster.GetName()),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "trusted_cluster"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "metadata.name", trustedCluster.GetName()),
					resource.TestCheckResourceAttr(name, "spec.enabled", "false"),
					resource.TestCheckResourceAttr(name, "spec.web_proxy_addr", "root.example.com:443"),
					resource.TestCheckResourceAttr(name, "spec.tunnel_addr", "root.example.com:3024"),
					resource.TestCheckResourceAttr(name, "spec.role_map.0.remote", "admin"),
					resource.TestCheckResourceAttr(name, "spec.role_map.0.local.0", "terraform-provider"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportTrustedCluster() {
	r := "teleport_trusted_cluster"
	id := "test_import"
	name := r + "." + id

	trustedCluster := s.createTrustedClusterForTest(s.T().Context(), "test-import")

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetTrustedCluster(s.T().Context(), trustedCluster.GetName())
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
				ImportStateId: trustedCluster.GetName(),
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "trusted_cluster", state[0].Attributes["kind"])
					require.Equal(s.T(), "root.example.com:443", state[0].Attributes["spec.web_proxy_addr"])
					require.Equal(s.T(), "root.example.com:3024", state[0].Attributes["spec.tunnel_addr"])
					require.Equal(s.T(), "admin", state[0].Attributes["spec.role_map.0.remote"])
					require.Equal(s.T(), "terraform-provider", state[0].Attributes["spec.role_map.0.local.0"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) createTrustedClusterForTest(ctx context.Context, name string) *types.TrustedClusterV2 {
	s.T().Helper()

	trustedCluster := &types.TrustedClusterV2{
		Metadata: types.Metadata{
			Name:        name,
			Description: "Trusted cluster managed by Terraform.",
		},
		Spec: types.TrustedClusterSpecV2{
			Enabled:              false,
			Token:                "trusted-cluster-token",
			ProxyAddress:         "root.example.com:443",
			ReverseTunnelAddress: "root.example.com:3024",
			RoleMap: types.RoleMap{{
				Remote: "admin",
				Local:  []string{"terraform-provider"},
			}},
		},
	}
	require.NoError(s.T(), trustedCluster.CheckAndSetDefaults())

	userCA, err := authcatest.NewCA(types.UserCA, trustedCluster.GetName())
	require.NoError(s.T(), err)
	hostCA, err := authcatest.NewCA(types.HostCA, trustedCluster.GetName())
	require.NoError(s.T(), err)

	revision, err := s.AuthHelper.Auth().Services.CreateTrustedCluster(ctx, trustedCluster, []types.CertAuthority{userCA, hostCA})
	require.NoError(s.T(), err)
	trustedCluster.SetRevision(revision)
	return trustedCluster
}
