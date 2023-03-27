/*
Copyright 2017-2021 Gravitational, Inc.

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

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	data := []byte(`{"kind": "github",
"version": "v3",
"metadata": {
  "name": "github"
},
"spec": {
  "client_id": "aaa",
  "client_secret": "bbb",
  "display": "GitHub",
  "redirect_url": "https://localhost:3080/v1/webapi/github/callback",
  "teams_to_logins": [{
    "organization": "gravitational",
    "team": "admins",
    "logins": ["admin"]
  }]
}}`)
	connector, err := UnmarshalGithubConnector(data)
	require.NoError(t, err)
	expected, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "aaa",
		ClientSecret: "bbb",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "GitHub",
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Logins:       []string{"admin"},
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, connector))
}

func TestMapClaims(t *testing.T) {
	t.Parallel()

	connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "aaa",
		ClientSecret: "bbb",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "GitHub",
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Logins:       []string{"admin", "dev"},
				KubeGroups:   []string{"system:masters", "kube-devs"},
				KubeUsers:    []string{"alice@example.com"},
			},
			{
				Organization: "gravitational",
				Team:         "devs",
				Logins:       []string{"dev", "test"},
				KubeGroups:   []string{"kube-devs"},
			},
		},
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Roles:        []string{"system"},
			},
		},
	})
	require.NoError(t, err)

	roles, kubeGroups, kubeUsers := connector.MapClaims(types.GithubClaims{
		OrganizationToTeams: map[string][]string{
			"gravitational": {"admins"},
		},
	})
	require.Empty(t, cmp.Diff(roles, []string{"admin", "dev", "system"}))
	require.Empty(t, cmp.Diff(kubeGroups, []string{"system:masters", "kube-devs"}))
	require.Empty(t, cmp.Diff(kubeUsers, []string{"alice@example.com"}))

	roles, kubeGroups, kubeUsers = connector.MapClaims(types.GithubClaims{
		OrganizationToTeams: map[string][]string{
			"gravitational": {"devs"},
		},
	})

	require.Empty(t, cmp.Diff(roles, []string{"dev", "test"}))
	require.Empty(t, cmp.Diff(kubeGroups, []string{"kube-devs"}))
	require.Empty(t, cmp.Diff(kubeUsers, []string(nil)))

	roles, kubeGroups, kubeUsers = connector.MapClaims(types.GithubClaims{
		OrganizationToTeams: map[string][]string{
			"gravitational": {"admins", "devs"},
		},
	})
	require.Empty(t, cmp.Diff(roles, []string{"admin", "dev", "test", "system"}))
	require.Empty(t, cmp.Diff(kubeGroups, []string{"system:masters", "kube-devs"}))
	require.Empty(t, cmp.Diff(kubeUsers, []string{"alice@example.com"}))
}
