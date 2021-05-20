/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"github.com/gravitational/teleport/api/types"
	"gopkg.in/check.v1"
)

type RolesTestSuite struct {
}

var _ = check.Suite(&RolesTestSuite{})

func (s *RolesTestSuite) TestParsing(c *check.C) {
	roles, err := types.ParseTeleportRoles("auth, Proxy,nODE")
	c.Assert(err, check.IsNil)
	c.Assert(roles, check.DeepEquals, types.SystemRoles{
		"Auth",
		"Proxy",
		"Node",
	})
	c.Assert(roles[0].Check(), check.IsNil)
	c.Assert(roles[1].Check(), check.IsNil)
	c.Assert(roles[2].Check(), check.IsNil)
	c.Assert(roles.Check(), check.IsNil)
	c.Assert(roles.String(), check.Equals, "Auth,Proxy,Node")
	c.Assert(roles[0].String(), check.Equals, "Auth")
}

func (s *RolesTestSuite) TestBadRoles(c *check.C) {
	bad := types.SystemRole("bad-role")
	c.Assert(bad.Check(), check.ErrorMatches, "role bad-role is not registered")
	badRoles := types.SystemRoles{
		bad,
		types.RoleAdmin,
	}
	c.Assert(badRoles.Check(), check.ErrorMatches, "role bad-role is not registered")
}

func (s *RolesTestSuite) TestEquivalence(c *check.C) {
	nodeProxyRole := types.SystemRoles{
		types.RoleNode,
		types.RoleProxy,
	}
	authRole := types.SystemRoles{
		types.RoleAdmin,
		types.RoleAuth,
	}

	c.Assert(authRole.Include(types.RoleAdmin), check.Equals, true)
	c.Assert(authRole.Include(types.RoleProxy), check.Equals, false)
	c.Assert(authRole.Equals(nodeProxyRole), check.Equals, false)
	c.Assert(authRole.Equals(types.SystemRoles{types.RoleAuth, types.RoleAdmin}),
		check.Equals, true)
}
