package auth

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"

	authority "github.com/gravitational/teleport/auth/native"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/membk"
	"github.com/gravitational/teleport/events/boltlog"
	"github.com/gravitational/teleport/session"
	"github.com/gravitational/teleport/sshutils"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type TunSuite struct {
	bk   *membk.MemBackend
	scrt *secret.Service

	srv    *httptest.Server
	tsrv   *TunServer
	a      *AuthServer
	signer ssh.Signer
	bl     *boltlog.BoltLog
	dir    string
}

var _ = Suite(&TunSuite{})

func (s *TunSuite) SetUpSuite(c *C) {
	s.dir = c.MkDir()

	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	s.bl, err = boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)
}

func (s *TunSuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *TunSuite) SetUpTest(c *C) {
	s.bk = membk.New()
	s.a = NewAuthServer(s.bk, authority.New(), s.scrt)
	s.srv = httptest.NewServer(NewAPIServer(s.a, s.bl, session.New(s.bk)))

	// set up host private key and certificate
	c.Assert(s.a.ResetHostCA(""), IsNil)
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", 0)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)
	s.signer = signer
	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	tsrv, err := NewTunServer(
		utils.NetAddr{Network: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{signer},
		utils.NetAddr{Network: "tcp", Addr: u.Host}, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv
}

func (s *TunSuite) TestUnixServerClient(c *C) {
	d, err := ioutil.TempDir("", "teleport-test")
	c.Assert(err, IsNil)
	socketPath := filepath.Join(d, "unix.sock")

	l, err := net.Listen("unix", socketPath)
	c.Assert(err, IsNil)

	h := NewAPIServer(s.a, s.bl, session.New(s.bk))
	srv := &httptest.Server{
		Listener: l,
		Config: &http.Server{
			Handler: h,
		},
	}
	srv.Start()
	defer srv.Close()

	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	tsrv, err := NewTunServer(
		utils.NetAddr{Network: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{s.signer},
		utils.NetAddr{Network: "tcp", Addr: u.Host}, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	user := "test"
	pass := []byte("pwd123")

	s.a.UpsertPassword(user, pass)

	authMethod, err := NewWebPasswordAuth(user, pass)
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: tsrv.Addr()},
		"test", authMethod)
	c.Assert(err, IsNil)

	err = clt.UpsertServer(
		backend.Server{ID: "a.example.com", Addr: "hello"}, 0)
	c.Assert(err, IsNil)
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")

	c.Assert(s.a.UpsertPassword(user, pass), IsNil)

	authMethod, err := NewWebPasswordAuth(user, pass)
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	c.Assert(clt.UpsertPassword(user, pass), IsNil)

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	out, err := clt.GetWebSession(user, ws)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	err = cltw.DeleteWebSession(user, ws)
	c.Assert(err, IsNil)

	_, err = clt.GetWebSession(user, ws)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestSessionsBadPassword(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := "system-test"
	pass := []byte("system-abc123")

	c.Assert(s.a.UpsertPassword(user, pass), IsNil)

	authMethod, err := NewWebPasswordAuth(user, pass)
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, []byte("different-pass"))
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")

	ws, err = clt.SignIn("not-exitsts", pass)
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")
}
