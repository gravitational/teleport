package auth

import (
	"net/http/httptest"

	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend/membk"
	"github.com/gravitational/teleport/sshutils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type TunSuite struct {
	clt  *TunClient
	bk   *membk.MemBackend
	scrt *secret.Service

	srv   *httptest.Server
	tsrv  *TunServer
	a     *AuthServer
	creds AuthMethod
}

var _ = Suite(&TunSuite{})

func (s *TunSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *TunSuite) SetUpTest(c *C) {
	s.bk = membk.New()
	s.a = NewAuthServer(s.bk, openssh.New(), s.scrt)
	s.srv = httptest.NewServer(NewAPIServer(s.a))

	// set up host private key and certificate
	c.Assert(s.a.ResetHostCA(""), IsNil)
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", 0)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewHostSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	tsrv, err := NewTunServer(
		sshutils.Addr{Net: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{signer},
		s.srv.URL, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	s.creds = AuthMethod{
		User: "test",
		Type: "password",
		Pass: []byte("pwd123"),
	}
	s.a.UpsertPassword(s.creds.User, s.creds.Pass)

	clt, err := NewTunClient(tsrv.Addr(), s.creds)
	c.Assert(err, IsNil)
	s.clt = clt
}

func (s *TunSuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := s.creds.User
	pass := []byte("abc123")

	ws, err := s.clt.SignIn(user, pass)
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")

	c.Assert(s.clt.UpsertPassword(user, pass), IsNil)

	ws, err = s.clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	out, err := s.clt.GetWebSession(user, ws)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	// Resume session via sesison id
	clt, err := NewTunClient(s.tsrv.Addr(), AuthMethod{User: user, Type: "session", Pass: []byte(ws)})
	c.Assert(err, IsNil)

	err = clt.DeleteWebSession(user, ws)
	c.Assert(err, IsNil)
	return
	_, err = clt.GetWebSession(user, ws)
	c.Assert(err, NotNil)
}
