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

package services

import (
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	. "gopkg.in/check.v1"
)

type UserSuite struct {
}

var _ = Suite(&UserSuite{})

func (s *UserSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *UserSuite) TestOIDCMapping(c *C) {
	conn := OIDCConnectorV2{}
	c.Assert(conn.MapClaims(jose.Claims{"a": "b"}), DeepEquals, []string(nil))

	conn = OIDCConnectorV2{
		Spec: OIDCConnectorSpecV2{
			ClaimsToRoles: []ClaimMapping{
				{Claim: "role", Value: "admin", Roles: []string{"admin", "bob"}},
				{Claim: "role", Value: "user", Roles: []string{"user"}},
			},
		},
	}
	c.Assert(conn.MapClaims(jose.Claims{"a": "b"}), DeepEquals, []string(nil))
	c.Assert(conn.MapClaims(jose.Claims{"role": "b"}), DeepEquals, []string(nil))
	c.Assert(conn.MapClaims(jose.Claims{"role": "admin"}), DeepEquals, []string{"admin", "bob"})
	c.Assert(conn.MapClaims(jose.Claims{"role": "user"}), DeepEquals, []string{"user"})
	c.Assert(conn.MapClaims(jose.Claims{"role": []string{"user"}}), DeepEquals, []string{"user"})
}
