package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/membk"

	. "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type APISuite struct {
	srv *httptest.Server
	clt *Client
	bk  *membk.MemBackend
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
}

func (s *APISuite) SetUpTest(c *C) {
	s.bk = membk.New()
	s.srv = httptest.NewServer(
		NewAPIServer(
			NewAuthServer(s.bk, openssh.New())))
	s.clt = NewClient(s.srv.URL)
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
	c.Assert(string(s.bk.Keys["user1"]["id"].Value), DeepEquals, string(cert))

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	keys, err := s.clt.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 1)
	c.Assert(string(keys[0].Value), DeepEquals, string(cert))

	c.Assert(s.clt.DeleteUserKey("user1", "id"), IsNil)
	_, ok := s.bk.Keys["user1"]["id"]
	c.Assert(ok, Equals, false)
}
