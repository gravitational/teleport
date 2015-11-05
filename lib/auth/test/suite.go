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
// package test contains CA authority acceptance test suite
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/lib/auth"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestAuth(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	A auth.Authority
}

func (s *AuthSuite) GenerateKeypairEmptyPass(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateKeypairPass(c *C) {
	_, pub, err := s.A.GenerateKeyPair("pass1")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	// TODO(klizhentas) test the private key actually
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateHostCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateHostCert(priv, pub, "auth", "auth.example.com", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateUserCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateUserCert(priv, pub, "user", "user", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	_, err = s.A.GenerateUserCert(priv, pub, "user", "user", -20)
	c.Assert(err, NotNil)

	_, err = s.A.GenerateUserCert(priv, pub, "user", "user", 0)
	c.Assert(err, NotNil)

	_, err = s.A.GenerateUserCert(priv, pub, "user", "user", 40*time.Hour)
	c.Assert(err, NotNil)

	_, err = s.A.GenerateUserCert(priv, pub, "user", "user", time.Hour)
	c.Assert(err, IsNil)
}
