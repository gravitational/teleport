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

package auth

import (
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	check "gopkg.in/check.v1"
)

type GithubSuite struct{}

var _ = check.Suite(&GithubSuite{})

func (s *GithubSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *GithubSuite) TestPopulateClaims(c *check.C) {
	claims, err := populateGithubClaims(&testGithubAPIClient{})
	c.Assert(err, check.IsNil)
	c.Assert(claims, check.DeepEquals, &services.GithubClaims{
		Email: "email3@example.com",
		OrganizationToTeams: map[string][]string{
			"org1": []string{"team1", "team2"},
			"org2": []string{"team1"},
		},
	})
}

type testGithubAPIClient struct{}

func (c *testGithubAPIClient) getEmails() ([]emailResponse, error) {
	return []emailResponse{
		{
			Email:    "email1@example.com",
			Primary:  false,
			Verified: false,
		},
		{
			Email:    "email2@example.com",
			Primary:  false,
			Verified: true,
		},
		{
			Email:    "email3@example.com",
			Primary:  true,
			Verified: true,
		},
	}, nil
}

func (c *testGithubAPIClient) getTeams() ([]teamResponse, error) {
	return []teamResponse{
		{
			Name: "team1",
			Slug: "team1",
			Org:  orgResponse{Login: "org1"},
		},
		{
			Name: "team2",
			Slug: "team2",
			Org:  orgResponse{Login: "org1"},
		},
		{
			Name: "team1",
			Slug: "team1",
			Org:  orgResponse{Login: "org2"},
		},
	}, nil
}
