/*
Copyright 2017 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/utils"

	check "gopkg.in/check.v1"
)

type GithubSuite struct{}

var _ = check.Suite(&GithubSuite{})

func (s *GithubSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *GithubSuite) TestUnmarshal(c *check.C) {
	data := []byte(`{"kind": "github",
"version": "v3",
"metadata": {
  "name": "github"
},
"spec": {
  "client_id": "aaa",
  "client_secret": "bbb",
  "display": "Github",
  "redirect_url": "https://localhost:3080/v1/webapi/github/callback",
  "teams_to_logins": [{
    "organization": "gravitational",
    "team": "admins",
    "logins": ["admin"]
  }]
}}`)
	connector, err := GetGithubConnectorMarshaler().Unmarshal(data)
	c.Assert(err, check.IsNil)
	expected := NewGithubConnector("github", GithubConnectorSpecV3{
		ClientID:     "aaa",
		ClientSecret: "bbb",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToLogins: []TeamMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Logins:       []string{"admin"},
			},
		},
	})
	c.Assert(expected, check.DeepEquals, connector)
}

func (s *GithubSuite) TestMapClaims(c *check.C) {
	connector := NewGithubConnector("github", GithubConnectorSpecV3{
		ClientID:     "aaa",
		ClientSecret: "bbb",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToLogins: []TeamMapping{
			{
				Organization: "gravitational",
				Team:         "admins",
				Logins:       []string{"admin", "dev"},
			},
			{
				Organization: "gravitational",
				Team:         "devs",
				Logins:       []string{"dev", "test"},
			},
		},
	})
	c.Assert(connector.MapClaims(GithubClaims{
		OrganizationToTeams: map[string][]string{
			"gravitational": []string{"admins"},
		},
	}), check.DeepEquals, []string{"admin", "dev"})
	c.Assert(connector.MapClaims(GithubClaims{
		OrganizationToTeams: map[string][]string{
			"gravitational": []string{"admins", "devs"},
		},
	}), check.DeepEquals, []string{"admin", "dev", "test"})
}
