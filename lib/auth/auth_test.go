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
	"testing"
	"time"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	bk backend.Backend
	a  *AuthServer

	dir string
}

var _ = Suite(&AuthSuite{})

func (s *AuthSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *AuthSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()
	var err error
	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	authConfig := &InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: "me.localhost",
	}
	s.a = NewAuthServer(authConfig)
}

func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		*suite.NewTestCA(services.UserCA, "me.localhost"), backend.Forever), IsNil)

	user := "user1"
	pass := []byte("abc123")

	ws, err := s.a.SignIn(user, pass)
	c.Assert(err, NotNil)

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)
	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

	ws, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = s.a.DeleteWebSession(user, ws.ID)
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(user, ws.ID)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		*suite.NewTestCA(services.HostCA, "me.localhost"), backend.Forever), IsNil)

	// generate single-use token (TTL is 0)
	tok, err := s.a.GenerateToken(teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)
	c.Assert(len(tok), Equals, 2*TokenLenBytes)

	roles, err := s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(roles.Include(teleport.RoleProxy), Equals, false)

	// unsuccessful registration (wrong role)
	keys, err := s.a.RegisterUsingToken(tok, "bad-host", teleport.RoleProxy)
	c.Assert(keys, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, "token.Role: role does not match")

	roles, err = s.a.ValidateToken(tok)
	c.Assert(err, IsNil)

	// successful registration:
	keys, err = s.a.RegisterUsingToken(tok, "good-host", teleport.RoleNode)
	c.Assert(err, IsNil)
	c.Assert(keys, NotNil)

	// unsuccessful registration (single-use token can't be used twice)
	keys, err = s.a.RegisterUsingToken(tok, "good-host-2", teleport.RoleNode)
	c.Assert(err, NotNil)
	c.Assert(keys, IsNil)

	// token should be gone by now:
	err = s.a.DeleteToken(tok)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	_, err = s.a.ValidateToken(tok)
	c.Assert(err, NotNil)

	// generate multi-use token with long TTL:
	multiUseToken, err := s.a.GenerateToken(teleport.Roles{teleport.RoleProxy}, time.Hour)
	c.Assert(err, IsNil)
	_, err = s.a.ValidateToken(multiUseToken)
	c.Assert(err, IsNil)

	// use it twice:
	_, err = s.a.RegisterUsingToken(multiUseToken, "once", teleport.RoleProxy)
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(multiUseToken, "twice", teleport.RoleProxy)
	c.Assert(err, IsNil)

	// try to use after TTL:
	s.a.clock = clockwork.NewFakeClockAt(time.Now().UTC().Add(time.Hour + 1))
	_, err = s.a.RegisterUsingToken(multiUseToken, "late.bird", teleport.RoleProxy)
	c.Assert(err, ErrorMatches, "token expired")

	// expired token should be gone now
	err = s.a.DeleteToken(multiUseToken)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	// lets use static tokens now
	roles = teleport.Roles{teleport.RoleProxy}
	s.a.StaticTokens = append(s.a.StaticTokens, StaticToken{Value: "static-token-value", Roles: roles})
	_, err = s.a.RegisterUsingToken("static-token-value", "static.host", teleport.RoleProxy)
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken("static-token-value", "wrong.role", teleport.RoleAuth)
	c.Assert(err, NotNil)
	r, err := s.a.ValidateToken("static-token-value")
	c.Assert(err, IsNil)
	c.Assert(r, DeepEquals, roles)
}

func (s *AuthSuite) TestBadTokens(c *C) {
	// empty
	_, err := s.a.ValidateToken("")
	c.Assert(err, NotNil)

	// garbage
	_, err = s.a.ValidateToken("bla bla")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken(teleport.Roles{teleport.RoleAuth}, 0)
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(tampered)
	c.Assert(err, NotNil)
}
