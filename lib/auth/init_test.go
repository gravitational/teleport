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
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type AuthInitSuite struct {
}

var _ = Suite(&AuthInitSuite{})

func (s *AuthInitSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

// TestReadIdentity makes parses identity from private key and certificate
// and checks that all parameters are valid
func (s *AuthInitSuite) TestReadIdentity(c *C) {
	t := testauthority.New()
	priv, pub, err := t.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := t.GenerateHostCert(priv, pub, "id1", "example.com", teleport.RoleNode, 0)
	c.Assert(err, IsNil)

	id, err := ReadIdentityFromKeyPair(priv, cert)
	c.Assert(err, IsNil)
	c.Assert(id.AuthorityDomain, Equals, "example.com")
	c.Assert(id.ID, DeepEquals, IdentityID{HostUUID: "id1", Role: teleport.RoleNode})
	c.Assert(id.CertBytes, DeepEquals, cert)
	c.Assert(id.KeyBytes, DeepEquals, priv)
}

func (s *AuthInitSuite) TestBadIdentity(c *C) {
	t := testauthority.New()
	priv, pub, err := t.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// bad cert type
	_, err = ReadIdentityFromKeyPair(priv, pub)
	c.Assert(teleport.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing authority domain
	cert, err := t.GenerateHostCert(priv, pub, "", "id2", teleport.RoleNode, 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(teleport.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing host uuid
	cert, err = t.GenerateHostCert(priv, pub, "example.com", "", teleport.RoleNode, 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(teleport.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// unrecognized role
	cert, err = t.GenerateHostCert(priv, pub, "example.com", "id1", teleport.Role("bad role"), 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(teleport.IsBadParameter(err), Equals, true, Commentf("%#v", err))
}
