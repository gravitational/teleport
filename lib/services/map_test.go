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
	"fmt"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type RoleMapSuite struct{}

var _ = check.Suite(&RoleMapSuite{})
var _ = fmt.Printf

func (s *RoleMapSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

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
		err := tc.roleMap.Check()
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
	}

	for _, tc := range testCases {
		comment := check.Commentf("test case '%v'", tc.name)
		local, err := tc.roleMap.Map(tc.remote)
		if tc.err != nil {
			c.Assert(err, check.NotNil, comment)
			c.Assert(err, check.FitsTypeOf, tc.err)
		} else {
			c.Assert(err, check.IsNil)
			c.Assert(local, check.DeepEquals, tc.local)
		}
	}
}
