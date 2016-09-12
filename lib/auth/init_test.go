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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/utils"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/trace"
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

	cert, err := t.GenerateHostCert(priv, pub, "id1", "example.com", teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)

	id, err := ReadIdentityFromKeyPair(priv, cert)
	c.Assert(err, IsNil)
	c.Assert(id.AuthorityDomain, Equals, "example.com")
	c.Assert(id.ID, DeepEquals, IdentityID{HostUUID: "id1", Role: teleport.RoleNode})
	c.Assert(id.CertBytes, DeepEquals, cert)
	c.Assert(id.KeyBytes, DeepEquals, priv)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := time.Second * 10
	expiryDate := time.Now().Add(ttl)
	bytes, err := t.GenerateHostCert(priv, pub, "id1", "example.com", teleport.Roles{teleport.RoleNode}, ttl)
	c.Assert(err, IsNil)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(bytes)
	c.Assert(err, IsNil)
	copy, ok := pk.(*ssh.Certificate)
	c.Assert(ok, Equals, true)
	c.Assert(uint64(expiryDate.Unix()), Equals, copy.ValidBefore)
}

func (s *AuthInitSuite) TestBadIdentity(c *C) {
	t := testauthority.New()
	priv, pub, err := t.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// bad cert type
	_, err = ReadIdentityFromKeyPair(priv, pub)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing authority domain
	cert, err := t.GenerateHostCert(priv, pub, "", "id2", teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing host uuid
	cert, err = t.GenerateHostCert(priv, pub, "example.com", "", teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// unrecognized role
	cert, err = t.GenerateHostCert(priv, pub, "example.com", "id1", teleport.Roles{teleport.Role("bad role")}, 0)
	c.Assert(err, IsNil)

	_, err = ReadIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))
}
