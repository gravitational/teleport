package auth

import (
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"

	"github.com/gokyle/hotp"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
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

	log.Initialize("console", "WARN")
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

	s.a = NewAuthServer(s.bk, authority.New(), s.scrt)
}

// TODO(klizhentas) introduce more thorough tests, test more edge cases
func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

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
	tok, err := s.a.GenerateToken("a.example.com", 0)
	c.Assert(err, IsNil)

	c.Assert(s.a.ValidateToken(tok, "a.example.com"), IsNil)

	c.Assert(s.a.DeleteToken(tok), IsNil)
	c.Assert(s.a.DeleteToken(tok), FitsTypeOf, &teleport.NotFoundError{})
	c.Assert(s.a.ValidateToken(tok, "a.example.com"),
		FitsTypeOf, &teleport.NotFoundError{})
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
