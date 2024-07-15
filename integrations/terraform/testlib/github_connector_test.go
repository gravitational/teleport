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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestGithubConnector() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetGithubConnector(ctx, "test", false)
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_github_connector.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("github_connector_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "github"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2032-10-12T07:20:50Z"),
					resource.TestCheckResourceAttr(name, "spec.client_id", "Iv1.3386eee92ff932a4"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.organization", "evilmartians"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.team", "devs"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.logins.0", "terraform"),
				),
			},
			{
				Config:   s.getFixture("github_connector_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("github_connector_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "github"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2032-10-12T07:20:50Z"),
					resource.TestCheckResourceAttr(name, "spec.client_id", "Iv1.3386eee92ff932a4"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.organization", "octocat"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.team", "devs"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_logins.0.logins.0", "terraform"),
				),
			},
			{
				Config:   s.getFixture("github_connector_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportGithubConnector() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_github_connector"
	id := "test_import"
	name := r + "." + id

	githubConnector := &types.GithubConnectorV3{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.GithubConnectorSpecV3{
			ClientID:     "Iv1.3386eee92ff932a4",
			ClientSecret: "secret",
			TeamsToLogins: []types.TeamMapping{
				{
					Organization: "evilmartians",
					Team:         "devs",
					Logins:       []string{"terraform"},
				},
			},
		},
	}

	err := githubConnector.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertGithubConnector(ctx, githubConnector)
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
					require.Equal(s.T(), "github", state[0].Attributes["kind"])
					require.Equal(s.T(), "Iv1.3386eee92ff932a4", state[0].Attributes["spec.client_id"])
					require.Equal(s.T(), "evilmartians", state[0].Attributes["spec.teams_to_logins.0.organization"])
					require.Equal(s.T(), "devs", state[0].Attributes["spec.teams_to_logins.0.team"])
					require.Equal(s.T(), "terraform", state[0].Attributes["spec.teams_to_logins.0.logins.0"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestGithubConnectorTeamsToRoles() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetGithubConnector(ctx, "test", false)
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_github_connector.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("github_connector_teams_to_roles.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "github"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2032-10-12T07:20:50Z"),
					resource.TestCheckResourceAttr(name, "spec.client_id", "Iv1.3386eee92ff932a4"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_roles.0.organization", "evilmartians"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_roles.0.team", "devs"),
					resource.TestCheckResourceAttr(name, "spec.teams_to_roles.0.roles.0", "myrole"),
				),
			},
			{
				Config:   s.getFixture("github_connector_teams_to_roles.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestGithubConnectorWithoutMapping() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetGithubConnector(ctx, "test", false)
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config:      s.getFixture("github_connector_without_mapping.tf"),
				ExpectError: regexp.MustCompile("team_to_logins or team_to_roles mapping is invalid, no mappings defined"),
			},
		},
	})
}
