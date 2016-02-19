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
package auth

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type PermCheckerSuite struct {
	c PermissionChecker
}

var _ = Suite(&PermCheckerSuite{})

func (s *PermCheckerSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
}

func (s *PermCheckerSuite) TestSessions(c *C) {
	p := NewStandardPermissions()

	c.Assert(p.HasPermission(teleport.RoleAdmin, ActionGenerateToken), IsNil)
	c.Assert(p.HasPermission(teleport.RoleUser, ActionSignIn), IsNil)

	c.Assert(p.HasPermission(teleport.RoleUser, ActionUpsertServer), NotNil)
	c.Assert(p.HasPermission("", ActionSignIn), NotNil)
	c.Assert(p.HasPermission(teleport.RoleUser, "noAction"), NotNil)
}
