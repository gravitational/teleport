package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/membk"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type APISuite struct {
	srv  *httptest.Server
	clt  *Client
	bk   *membk.MemBackend
	scrt *secret.Service
	a    *AuthServer
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

func (s *APISuite) SetUpTest(c *C) {
	s.bk = membk.New()
	s.a = NewAuthServer(s.bk, openssh.New(), s.scrt)
	s.srv = httptest.NewServer(NewAPIServer(s.a, memlog.New()))
	clt, err := NewClient(s.srv.URL)
	c.Assert(err, IsNil)
	s.clt = clt
}

func (s *APISuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *APISuite) TestHostCACRUD(c *C) {
	c.Assert(s.clt.ResetHostCA(), IsNil)
	ca := s.bk.HostCA
	c.Assert(s.clt.ResetHostCA(), IsNil)
	c.Assert(ca, Not(DeepEquals), s.bk.HostCA)

	key, err := s.clt.GetHostCAPub()
	c.Assert(err, IsNil)
	c.Assert(key, DeepEquals, s.bk.HostCA.Pub)
}

func (s *APISuite) TestUserCACRUD(c *C) {
	c.Assert(s.clt.ResetUserCA(), IsNil)
	ca := s.bk.UserCA
	c.Assert(s.clt.ResetUserCA(), IsNil)
	c.Assert(ca, Not(DeepEquals), s.bk.UserCA)

	key, err := s.clt.GetUserCAPub()
	c.Assert(err, IsNil)
	c.Assert(key, DeepEquals, s.bk.UserCA.Pub)
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
	c.Assert(string(s.bk.Users["user1"].Keys["id"].Value), DeepEquals, string(cert))

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	keys, err := s.clt.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 1)
	c.Assert(string(keys[0].Value), DeepEquals, string(cert))

	c.Assert(s.clt.DeleteUserKey("user1", "id"), IsNil)
	_, ok := s.bk.Users["user1"].Keys["id"]
	c.Assert(ok, Equals, false)
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
	err := s.clt.SubmitEvents(
		[][]byte{
			[]byte(`{"e": "event 1"}`),
			[]byte(`{"e": "event 2"}`)})
	c.Assert(err, IsNil)

	out, err := s.clt.GetEvents()
	c.Assert(err, IsNil)
	expected := []interface{}{
		map[string]interface{}{"e": "event 2"},
		map[string]interface{}{"e": "event 1"},
	}
	c.Assert(out, DeepEquals, expected)
}

func (s *APISuite) TestTokens(c *C) {
	out, err := s.clt.GenerateToken("a.example.com", 0)
	c.Assert(err, IsNil)
	c.Assert(len(out), Not(Equals), 0)
}
