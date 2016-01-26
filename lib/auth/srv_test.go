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
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokyle/hotp"

	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	etest "github.com/gravitational/teleport/lib/events/test"
	rtest "github.com/gravitational/teleport/lib/recorder/test"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/session"

	"github.com/mailgun/lemma/secret"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type APISuite struct {
	srv  *httptest.Server
	clt  *Client
	bk   *encryptedbk.ReplicatedBackend
	bl   *boltlog.BoltLog
	scrt secret.SecretService
	rec  recorder.Recorder
	a    *AuthServer
	dir  string

	CAS           *services.CAService
	LockS         *services.LockService
	PresenceS     *services.PresenceService
	ProvisioningS *services.ProvisioningService
	UserS         *services.UserService
	WebS          *services.WebService
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
	authority.PrecalculatedKeysNum = 1
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

func (s *APISuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.bl, err = boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	s.rec, err = boltrec.New(s.dir)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(s.bk, authority.New(), s.scrt, "host1")
	s.srv = httptest.NewServer(NewAPIServer(
		&AuthWithRoles{
			authServer:  s.a,
			elog:        s.bl,
			sessions:    session.New(s.bk),
			recorder:    s.rec,
			permChecker: NewAllowAllPermissions(),
		}))
	clt, err := NewClient(s.srv.URL)
	c.Assert(err, IsNil)
	s.clt = clt

	s.CAS = services.NewCAService(s.bk)
	s.LockS = services.NewLockService(s.bk)
	s.PresenceS = services.NewPresenceService(s.bk)
	s.ProvisioningS = services.NewProvisioningService(s.bk)
	s.UserS = services.NewUserService(s.bk)
	s.WebS = services.NewWebService(s.bk)
}

func (s *APISuite) TearDownTest(c *C) {
	s.srv.Close()
	s.bl.Close()
}

func (s *APISuite) TestHostCACRUD(c *C) {
	c.Assert(s.clt.ResetHostCertificateAuthority(), IsNil)

	hca, err := s.CAS.GetHostPrivateCertificateAuthority()
	c.Assert(err, IsNil)

	c.Assert(s.clt.ResetHostCertificateAuthority(), IsNil)

	hca2, err := s.CAS.GetHostPrivateCertificateAuthority()
	c.Assert(err, IsNil)

	c.Assert(hca, Not(DeepEquals), hca2)

	key, err := s.clt.GetHostCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(key.PublicKey, DeepEquals, hca2.PublicKey)
}

func (s *APISuite) TestUserCACRUD(c *C) {
	c.Assert(s.clt.ResetUserCertificateAuthority(), IsNil)

	uca, err := s.CAS.GetUserPrivateCertificateAuthority()
	c.Assert(err, IsNil)

	c.Assert(s.clt.ResetUserCertificateAuthority(), IsNil)
	uca2, err := s.CAS.GetUserPrivateCertificateAuthority()
	c.Assert(err, IsNil)

	c.Assert(uca, Not(DeepEquals), uca2)

	key, err := s.clt.GetUserCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(key.PublicKey, DeepEquals, uca2.PublicKey)
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
	c.Assert(s.clt.ResetHostCertificateAuthority(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateHostCert(pub, "id1", "a.example.com", "RoleExample", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestGenerateUserCert(c *C) {
	c.Assert(s.clt.ResetUserCertificateAuthority(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateUserCert(pub, "id1", "user1", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestKeysCRUD(c *C) {
	c.Assert(s.clt.ResetUserCertificateAuthority(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateUserCert(pub, "id1", "user1", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestUserKeyCRUD(c *C) {
	c.Assert(s.clt.ResetUserCertificateAuthority(), IsNil)

	_, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	key := services.AuthorizedKey{ID: "id", Value: pub}
	cert, err := s.clt.UpsertUserKey("user1", key, time.Minute)
	c.Assert(err, IsNil)

	keys, err := s.UserS.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(string(keys[0].Value), DeepEquals, string(cert))

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	c.Assert(s.clt.DeleteUserKey("user1", "id"), IsNil)
	keys, err = s.UserS.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 0)
}

func (s *APISuite) TestPasswordCRUD(c *C) {
	pass := []byte("abc123")

	err := s.clt.CheckPassword("user1", pass, "123456")
	c.Assert(err, NotNil)

	hotpURL, _, err := s.clt.UpsertPassword("user1", pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

	token1 := otp.OTP()
	c.Assert(s.clt.CheckPassword("user1", pass, "123456"), NotNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token1), IsNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token1), NotNil)

	token2 := otp.OTP()
	c.Assert(s.clt.CheckPassword("user1", []byte("abc123123"), token2), NotNil)
	c.Assert(s.clt.CheckPassword("user1", pass, "123456"), NotNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token2), IsNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token1), NotNil)

	_ = otp.OTP()
	_ = otp.OTP()
	_ = otp.OTP()
	token6 := otp.OTP()
	token7 := otp.OTP()
	c.Assert(s.clt.CheckPassword("user1", pass, token7), NotNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token6), IsNil)
	c.Assert(s.clt.CheckPassword("user1", pass, "123456"), NotNil)
	c.Assert(s.clt.CheckPassword("user1", pass, token7), IsNil)

	_ = otp.OTP()
	token9 := otp.OTP()
	c.Assert(s.clt.CheckPassword("user1", pass, token9), IsNil)
}

func (s *APISuite) TestSessions(c *C) {
	user := "user1"
	pass := []byte("abc123")

	c.Assert(s.a.ResetUserCertificateAuthority(""), IsNil)

	ws, err := s.clt.SignIn(user, pass)
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")

	hotpURL, _, err := s.clt.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

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

	t := services.WebTun{
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
	c.Assert(tuns, DeepEquals, []services.WebTun{t})

	c.Assert(s.clt.DeleteWebTun("p1"), IsNil)

	_, err = s.clt.GetWebTun("p1")
	c.Assert(err, NotNil)
}

func (s *APISuite) TestServers(c *C) {
	out, err := s.clt.GetServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := services.Server{ID: "id1", Addr: "host:1233", Hostname: "host1"}
	c.Assert(s.clt.UpsertServer(srv, 0), IsNil)

	srv1 := services.Server{ID: "id2", Addr: "host:1234", Hostname: "host2"}
	c.Assert(s.clt.UpsertServer(srv1, 0), IsNil)

	out, err = s.clt.GetServers()
	c.Assert(err, IsNil)

	if out[0].ID == "id1" {
		c.Assert(out[0], DeepEquals, services.Server{ID: "id1", Addr: "host:1233", Hostname: "host1"})
		c.Assert(out[1], DeepEquals, services.Server{ID: "id2", Addr: "host:1234", Hostname: "host2"})
	} else {
		c.Assert(out[1], DeepEquals, services.Server{ID: "id1", Addr: "host:1233", Hostname: "host1"})
		c.Assert(out[0], DeepEquals, services.Server{ID: "id2", Addr: "host:1234", Hostname: "host2"})
	}
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
	out, err := s.clt.GenerateToken("a.example.com", "RoleExample", 0)
	c.Assert(err, IsNil)
	c.Assert(len(out), Not(Equals), 0)
}

func (s *APISuite) TestRemoteCACRUD(c *C) {
	key := services.CertificateAuthority{
		DomainName: "example.com",
		ID:         "id",
		PublicKey:  []byte("hello1"),
		Type:       services.UserCert,
	}
	err := s.clt.UpsertRemoteCertificate(key, 0)
	c.Assert(err, IsNil)

	certs, err := s.clt.GetRemoteCertificates(key.Type, key.DomainName)
	c.Assert(err, IsNil)
	c.Assert(certs[0], DeepEquals, key)

	err = s.clt.DeleteRemoteCertificate(key.Type, key.DomainName, key.ID)
	c.Assert(err, IsNil)

	err = s.clt.DeleteRemoteCertificate(key.Type, key.DomainName, key.ID)
	c.Assert(err, NotNil)
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
