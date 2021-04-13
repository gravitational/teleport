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
	"github.com/gravitational/teleport"

	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

type RoleMapSuite struct{}

var _ = check.Suite(&RoleMapSuite{})

func (s *RoleMapSuite) TestRoleParsing(c *check.C) {
	testCases := []struct {
		roleMap RoleMap
		err     error
	}{
		{
			roleMap: nil,
		},
		{
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
		{
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs"}},
				{Remote: Wildcard, Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
	}

	for i, tc := range testCases {
		comment := check.Commentf("test case '%v'", i)
		_, err := parseRoleMap(tc.roleMap)
		if tc.err != nil {
			c.Assert(err, check.NotNil, comment)
			c.Assert(err, check.FitsTypeOf, tc.err)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}

func (s *RoleMapSuite) TestRoleMap(c *check.C) {
	testCases := []struct {
		remote  []string
		local   []string
		roleMap RoleMap
		name    string
		err     error
	}{
		{
			name:    "all empty",
			remote:  nil,
			local:   nil,
			roleMap: nil,
		},
		{
			name:   "wildcard matches empty as well",
			remote: nil,
			local:  []string{"local-devs", "local-admins"},
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			name:   "direct match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			name:   "direct match for multiple roles",
			remote: []string{"remote-devs", "remote-logs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-logs", Local: []string{"local-logs"}},
			},
		},
		{
			name:   "direct match and wildcard",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: Wildcard, Local: []string{"local-logs"}},
			},
		},
		{
			name:   "glob capture match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: RoleMap{
				{Remote: "remote-*", Local: []string{"local-$1"}},
			},
		},
		{
			name:   "passthrough match",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "passthrough match ignores implicit role",
			remote: []string{"remote-devs", teleport.DefaultImplicitRole},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial match",
			remote: []string{"remote-devs", "something-else"},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial empty expand section is removed",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "remote-"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1", "remote-$2", "$2"}},
			},
		},
		{
			name:   "multiple matches yield different results",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "test"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
				{Remote: `^\Aremote-.*$`, Local: []string{"test"}},
			},
		},
		{
			name:   "different expand groups can be referred",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "devs"},
			roleMap: RoleMap{
				{Remote: "^(remote-(.*))$", Local: []string{"$1", "$2"}},
			},
		},
	}

	for _, tc := range testCases {
		comment := check.Commentf("test case '%v'", tc.name)
		local, err := MapRoles(tc.roleMap, tc.remote)
		if tc.err != nil {
			c.Assert(err, check.NotNil, comment)
			c.Assert(err, check.FitsTypeOf, tc.err)
		} else {
			c.Assert(err, check.IsNil, comment)
			c.Assert(local, check.DeepEquals, tc.local, comment)
		}
	}
}
