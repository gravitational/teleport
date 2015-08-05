package auth

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	authority "github.com/gravitational/teleport/auth/native"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/events/boltlog"
	etest "github.com/gravitational/teleport/events/test"
	rtest "github.com/gravitational/teleport/recorder/test"

	"github.com/gravitational/teleport/recorder"
	"github.com/gravitational/teleport/recorder/boltrec"
	"github.com/gravitational/teleport/session"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type APISuite struct {
	srv  *httptest.Server
	clt  *Client
	bk   *boltbk.BoltBackend
	bl   *boltlog.BoltLog
	scrt *secret.Service
	rec  recorder.Recorder
	a    *AuthServer
	dir  string
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv

	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *APISuite) SetUpTest(c *C) {
	s.dir = c.MkDir()
	var err error
	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.bl, err = boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	s.rec, err = boltrec.New(s.dir)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(s.bk, authority.New(), s.scrt)
	s.srv = httptest.NewServer(NewAPIServer(s.a, s.bl, session.New(s.bk), s.rec))
	clt, err := NewClient(s.srv.URL)
	c.Assert(err, IsNil)
	s.clt = clt
}

func (s *APISuite) TearDownTest(c *C) {
	s.srv.Close()
	s.bk.Close()
	s.bl.Close()
}

func (s *APISuite) TestHostCACRUD(c *C) {
	c.Assert(s.clt.ResetHostCA(), IsNil)

	hca, err := s.bk.GetHostCA()
	c.Assert(err, IsNil)

	c.Assert(s.clt.ResetHostCA(), IsNil)

	hca2, err := s.bk.GetHostCA()
	c.Assert(err, IsNil)

	c.Assert(hca, Not(DeepEquals), hca2)

	key, err := s.clt.GetHostCAPub()
	c.Assert(err, IsNil)
	c.Assert(key, DeepEquals, hca2.Pub)
}

func (s *APISuite) TestUserCACRUD(c *C) {
	c.Assert(s.clt.ResetUserCA(), IsNil)

	uca, err := s.bk.GetUserCA()
	c.Assert(err, IsNil)

	c.Assert(s.clt.ResetUserCA(), IsNil)
	uca2, err := s.bk.GetUserCA()
	c.Assert(err, IsNil)

	c.Assert(uca, Not(DeepEquals), uca2)

	key, err := s.clt.GetUserCAPub()
	c.Assert(err, IsNil)
	c.Assert(key, DeepEquals, uca2.Pub)
}

func (s *APISuite) TestGenerateKeyPair(c *C) {
	priv, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestGenerateHostCert(c *C) {
	c.Assert(s.clt.ResetHostCA(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateHostCert(pub, "id1", "a.example.com", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestGenerateUserCert(c *C) {
	c.Assert(s.clt.ResetUserCA(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateUserCert(pub, "id1", "user1", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestKeysCRUD(c *C) {
	c.Assert(s.clt.ResetUserCA(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateUserCert(pub, "id1", "user1", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestUserKeyCRUD(c *C) {
	c.Assert(s.clt.ResetUserCA(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	key := backend.AuthorizedKey{ID: "id", Value: pub}
	cert, err := s.clt.UpsertUserKey("user1", key, 0)
	c.Assert(err, IsNil)

	keys, err := s.bk.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(string(keys[0].Value), DeepEquals, string(cert))

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	c.Assert(s.clt.DeleteUserKey("user1", "id"), IsNil)
	keys, err = s.bk.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 0)
}

func (s *APISuite) TestPasswordCRUD(c *C) {
	pass := []byte("abc123")

	err := s.clt.CheckPassword("user1", pass)
	c.Assert(err, NotNil)

	c.Assert(s.clt.UpsertPassword("user1", pass), IsNil)
	c.Assert(s.clt.CheckPassword("user1", pass), IsNil)
	c.Assert(s.clt.CheckPassword("user1", []byte("abc123123")), NotNil)
}

func (s *APISuite) TestSessions(c *C) {
	user := "user1"
	pass := []byte("abc123")

	c.Assert(s.a.ResetUserCA(""), IsNil)

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

	err = s.clt.DeleteWebSession(user, ws)
	c.Assert(err, IsNil)

	_, err = s.clt.GetWebSession(user, ws)
	c.Assert(err, NotNil)
}

func (s *APISuite) TestWebTuns(c *C) {
	_, err := s.clt.GetWebTun("p1")
	c.Assert(err, NotNil)

	t := backend.WebTun{
		Prefix:     "p1",
		TargetAddr: "http://localhost:5000",
		ProxyAddr:  "node1.gravitational.io",
	}
	c.Assert(s.clt.UpsertWebTun(t, 0), IsNil)

	out, err := s.clt.GetWebTun("p1")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &t)

	tuns, err := s.clt.GetWebTuns()
	c.Assert(err, IsNil)
	c.Assert(tuns, DeepEquals, []backend.WebTun{t})

	c.Assert(s.clt.DeleteWebTun("p1"), IsNil)

	_, err = s.clt.GetWebTun("p1")
	c.Assert(err, NotNil)
}

func (s *APISuite) TestServers(c *C) {
	out, err := s.clt.GetServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := backend.Server{ID: "id1", Addr: "host:1233"}
	c.Assert(s.clt.UpsertServer(srv, 0), IsNil)

	srv1 := backend.Server{ID: "id2", Addr: "host:1234"}
	c.Assert(s.clt.UpsertServer(srv1, 0), IsNil)

	out, err = s.clt.GetServers()
	c.Assert(err, IsNil)

	servers := map[string]string{}
	for _, s := range out {
		servers[s.ID] = s.Addr
	}
	expected := map[string]string{"id1": "host:1233", "id2": "host:1234"}
	c.Assert(servers, DeepEquals, expected)
}

func (s *APISuite) TestEvents(c *C) {
	suite := etest.EventSuite{L: s.clt}
	suite.EventsCRUD(c)
}

func (s *APISuite) TestRecorder(c *C) {
	suite := rtest.RecorderSuite{R: s.clt}
	suite.Recorder(c)
}

func (s *APISuite) TestTokens(c *C) {
	out, err := s.clt.GenerateToken("a.example.com", 0)
	c.Assert(err, IsNil)
	c.Assert(len(out), Not(Equals), 0)
}

func (s *APISuite) TestRemoteCACRUD(c *C) {
	key := backend.RemoteCert{
		FQDN:  "example.com",
		ID:    "id",
		Value: []byte("hello1"),
		Type:  backend.UserCert,
	}
	err := s.clt.UpsertRemoteCert(key, 0)
	c.Assert(err, IsNil)

	certs, err := s.clt.GetRemoteCerts(key.Type, key.FQDN)
	c.Assert(err, IsNil)
	c.Assert(certs[0], DeepEquals, key)

	err = s.clt.DeleteRemoteCert(key.Type, key.FQDN, key.ID)
	c.Assert(err, IsNil)

	err = s.clt.DeleteRemoteCert(key.Type, key.FQDN, key.ID)
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *APISuite) TestSharedSessions(c *C) {
	out, err := s.clt.GetSessions()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []session.Session{})

	c.Assert(s.clt.UpsertSession("s1", 0), IsNil)

	out, err = s.clt.GetSessions()
	c.Assert(err, IsNil)
	sess := session.Session{
		ID:      "s1",
		Parties: []session.Party{},
	}
	c.Assert(out, DeepEquals, []session.Session{sess})
}

func (s *APISuite) TestSharedSessionsParties(c *C) {
	out, err := s.clt.GetSessions()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []session.Session{})

	p1 := session.Party{
		ID:         "p1",
		User:       "bob",
		Site:       "example.com",
		ServerAddr: "localhost:1",
		LastActive: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}
	c.Assert(s.clt.UpsertParty("s1", p1, 0), IsNil)

	out, err = s.clt.GetSessions()
	c.Assert(err, IsNil)
	sess := session.Session{
		ID:      "s1",
		Parties: []session.Party{p1},
	}
	c.Assert(out, DeepEquals, []session.Session{sess})
}
