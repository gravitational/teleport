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
	"path/filepath"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/mailgun/lemma/secret"

	"github.com/gokyle/hotp"

	. "gopkg.in/check.v1"
)

type AuthSuite struct {
	bk   *encryptedbk.ReplicatedBackend
	scrt secret.SecretService
	a    *AuthServer

	dir string
}

var _ = Suite(&AuthSuite{})

func (s *AuthSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

func (s *AuthSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()
	var err error
	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(s.bk, authority.New(), s.scrt, "host1")
}

// TODO(klizhentas) introduce more thorough tests, test more edge cases
func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCertificateAuthority(""), IsNil)

	user := "user1"
	pass := []byte("abc123")

	ws, err := s.a.SignIn(user, pass)
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)
	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

	ws, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSession(user, ws.SID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = s.a.DeleteWebSession(user, ws.SID)
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(user, ws.SID)
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	tok, err := s.a.GenerateToken("a.example.com", "Node", 0)
	c.Assert(err, IsNil)

	role, err := s.a.ValidateToken(tok, "a.example.com")
	c.Assert(err, IsNil)
	c.Assert(role, Equals, "Node")

	c.Assert(s.a.DeleteToken(tok), IsNil)
	c.Assert(s.a.DeleteToken(tok), FitsTypeOf, &teleport.NotFoundError{})

	_, err = s.a.ValidateToken(tok, "a.example.com")
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestBadTokens(c *C) {
	// empty
	_, err := s.a.ValidateToken("", "")
	c.Assert(err, NotNil)

	// garbage
	_, err = s.a.ValidateToken("bla bla", " hello !!<")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken("a.example.com", "Auth", 0)
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(tampered, "a.example.com")
	c.Assert(err, NotNil)

	// wrong domainName
	_, err = s.a.ValidateToken(tok, "b.example.com")
	c.Assert(err, NotNil)
}
