package auth

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/membk"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type AuthSuite struct {
	bk   *membk.MemBackend
	scrt *secret.Service
	a    *AuthServer
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
	s.bk = membk.New()
	s.a = NewAuthServer(s.bk, openssh.New(), s.scrt)
}

func (s *AuthSuite) TestPasswordCRUD(c *C) {
	pass := []byte("abc123")

	err := s.a.CheckPassword("user1", pass)
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})

	c.Assert(s.a.UpsertPassword("user1", pass), IsNil)
	c.Assert(s.a.CheckPassword("user1", pass), IsNil)
	c.Assert(s.a.CheckPassword("user1", []byte("abc123123")), FitsTypeOf, &BadParameterError{})
}

func (s *AuthSuite) TestPasswordGarbage(c *C) {
	garbage := [][]byte{
		nil,
		make([]byte, MaxPasswordLength+1),
		make([]byte, MinPasswordLength-1),
	}
	for _, g := range garbage {
		err := s.a.CheckPassword("user1", g)
		c.Assert(err, FitsTypeOf, &BadParameterError{})
	}
}

// TODO(klizhentas) introduce more thorough tests, test more edge cases
func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := "user1"
	pass := []byte("abc123")

	ws, err := s.a.SignIn(user, pass)
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
	c.Assert(ws, IsNil)

	c.Assert(s.a.UpsertPassword(user, pass), IsNil)

	ws, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSession(user, ws.SID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = s.a.DeleteWebSession(user, ws.SID)
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(user, ws.SID)
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	tok, err := s.a.GenerateToken("a.example.com", 0)
	c.Assert(err, IsNil)

	c.Assert(s.a.ValidateToken(tok, "a.example.com"), IsNil)

	c.Assert(s.a.DeleteToken(tok), IsNil)
	c.Assert(s.a.DeleteToken(tok), FitsTypeOf, &backend.NotFoundError{})
	c.Assert(s.a.ValidateToken(tok, "a.example.com"),
		FitsTypeOf, &backend.NotFoundError{})
}

func (s *AuthSuite) TestBadTokens(c *C) {
	// empty
	err := s.a.ValidateToken("", "")
	c.Assert(err, NotNil)

	// garbage
	err = s.a.ValidateToken("bla bla", " hello !!<")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken("a.example.com", 0)
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	err = s.a.ValidateToken(tampered, "a.example.com")
	c.Assert(err, NotNil)

	// wrong fqdn
	err = s.a.ValidateToken(tok, "b.example.com")
	c.Assert(err, NotNil)
}
