/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
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

func TestMarshal(t *testing.T) {
	connectorWithPublicEndpoint, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:       "aaa",
		ClientSecret:   "bbb",
		RedirectURL:    "https://localhost:3080/v1/webapi/github/callback",
		Display:        "GitHub",
		EndpointURL:    "https://github.com",
		APIEndpointURL: "https://api.github.com",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Roles:        []string{teleport.PresetAccessRoleName},
			},
		},
	})
	require.NoError(t, err)

	connectorWithPrivateEndpoint, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:       "aaa",
		ClientSecret:   "bbb",
		RedirectURL:    "https://localhost:3080/v1/webapi/github/callback",
		Display:        "GitHub",
		EndpointURL:    "https://my-private-github.com",
		APIEndpointURL: "https://api.my-private-github.com",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Roles:        []string{teleport.PresetAccessRoleName},
			},
		},
	})
	require.NoError(t, err)

	t.Run("oss with public endpoint", func(t *testing.T) {
		marshaled, err := MarshalGithubConnector(connectorWithPublicEndpoint)
		require.NoError(t, err)

		unmarshaled, err := UnmarshalGithubConnector(marshaled)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(connectorWithPublicEndpoint, unmarshaled))
	})

	t.Run("oss with private endpoint", func(t *testing.T) {
		_, err := MarshalGithubConnector(connectorWithPrivateEndpoint)
		require.ErrorIs(t, err, ErrRequiresEnterprise, "expected ErrRequiresEnterprise, got %T", err)
	})

	t.Run("enterprise", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

		marshaled, err := MarshalGithubConnector(connectorWithPrivateEndpoint)
		require.NoError(t, err)

		unmarshaled, err := UnmarshalGithubConnector(marshaled)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(connectorWithPrivateEndpoint, unmarshaled))
	})
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
