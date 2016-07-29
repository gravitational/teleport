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
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gokyle/hotp"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

type APISuite struct {
	srv      *httptest.Server
	clt      *Client
	bk       backend.Backend
	a        *AuthServer
	dir      string
	alog     *events.AuditLog
	sessions session.Service

	CAS           services.Trust
	LockS         services.Lock
	PresenceS     services.Presence
	ProvisioningS services.Provisioner
	WebS          services.Identity
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
	authority.PrecalculatedKeysNum = 1
}

func (s *APISuite) SetUpTest(c *C) {
	s.dir = c.MkDir()
	var err error

	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.alog, err = events.NewAuditLog(s.dir)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(&InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: "localhost",
	})
	s.sessions, err = session.New(s.bk)
	c.Assert(err, IsNil)

	apiServer := NewAPIServer(&APIConfig{
		AuthServer:        s.a,
		PermissionChecker: NewAllowAllPermissions(),
		SessionService:    s.sessions,
		AuditLog:          s.alog,
	}, teleport.RoleAdmin)
	s.srv = httptest.NewServer(&apiServer)

	clt, err := NewClient(s.srv.URL, nil)
	c.Assert(err, IsNil)
	s.clt = clt

	s.CAS = local.NewCAService(s.bk)
	s.LockS = local.NewLockService(s.bk)
	s.PresenceS = local.NewPresenceService(s.bk)
	s.ProvisioningS = local.NewProvisioningService(s.bk)
	s.WebS = local.NewIdentityService(s.bk, 10, time.Duration(time.Hour))
}

func (s *APISuite) TearDownTest(c *C) {
	s.srv.Close()
	s.alog.Close()
	os.RemoveAll(s.dir)
}

func (s *APISuite) TestGenerateKeysAndCerts(c *C) {
	priv, pub, err := s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, IsNil)
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)

	c.Assert(s.clt.UpsertCertAuthority(
		*suite.NewTestCA(services.HostCA, "localhost"), backend.Forever), IsNil)

	_, pub, err = s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateHostCert(pub, "localhost", "localhost", teleport.Roles{teleport.RoleNode}, time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	c.Assert(s.clt.UpsertCertAuthority(
		*suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	_, pub, err = s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	err = s.clt.UpsertUser(
		&services.TeleportUser{Name: "user1", AllowedLogins: []string{"user1"}})
	c.Assert(err, IsNil)

	userServer := NewAPIServer(&APIConfig{
		AuthServer:        s.a,
		PermissionChecker: NewAllowAllPermissions(),
		SessionService:    s.sessions,
		AuditLog:          s.alog,
	}, teleport.RoleUser)
	authServer := httptest.NewServer(&userServer)
	defer authServer.Close()

	userClient, err := NewClient(authServer.URL, nil)
	c.Assert(err, IsNil)

	// should NOT be able to generate a user cert without basic HTTP auth
	cert, err = userClient.GenerateUserCert(pub, "user1", time.Hour)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, ".*cannot request a certificate for user1")

	// Users don't match
	roundtrip.BasicAuth("user2", "two")(&userClient.Client)
	cert, err = userClient.GenerateUserCert(pub, "user1", time.Hour)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, ".*cannot request a certificate for user1")

	// apply HTTP Auth to generate user cert:
	roundtrip.BasicAuth("user1", "two")(&userClient.Client)
	cert, err = userClient.GenerateUserCert(pub, "user1", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *APISuite) TestUserCRUD(c *C) {
	_, _, err := s.clt.UpsertPassword("user1", []byte("some pass"))
	c.Assert(err, IsNil)

	users, err := s.WebS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(users), Equals, 1)
	c.Assert(users[0].GetName(), Equals, "user1")

	c.Assert(s.clt.DeleteUser("user1"), IsNil)

	users, err = s.WebS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(users), Equals, 0)
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

	c.Assert(s.a.UpsertCertAuthority(
		*suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	s.a.UpsertUser(
		&services.TeleportUser{Name: "user1", AllowedLogins: []string{"user1"}})

	ws, err := s.clt.SignIn(user, pass)
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)

	hotpURL, _, err := s.clt.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

	ws, err = s.clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	out, err := s.clt.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	new, err := s.clt.ExtendWebSession(user, ws.ID)
	c.Assert(err, IsNil)
	c.Assert(new, NotNil)

	err = s.clt.DeleteWebSession(user, ws.ID)
	c.Assert(err, IsNil)

	_, err = s.clt.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, NotNil)

	_, err = s.clt.ExtendWebSession(user, ws.ID)
	c.Assert(err, NotNil)
}

func (s *APISuite) TestServers(c *C) {
	out, err := s.clt.GetNodes()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := services.Server{ID: "id1", Addr: "host:1233", Hostname: "host1"}
	c.Assert(s.clt.UpsertNode(srv, 0), IsNil)

	srv1 := services.Server{ID: "id2", Addr: "host:1234", Hostname: "host2"}
	c.Assert(s.clt.UpsertNode(srv1, 0), IsNil)

	out, err = s.clt.GetNodes()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})

	out, err = s.clt.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv = services.Server{ID: "proxy1", Addr: "host:1233", Hostname: "host1"}
	c.Assert(s.clt.UpsertProxy(srv, 0), IsNil)

	srv1 = services.Server{ID: "proxy2", Addr: "host:1234", Hostname: "host2"}
	c.Assert(s.clt.UpsertProxy(srv1, 0), IsNil)

	out, err = s.clt.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})

	out, err = s.clt.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv = services.Server{ID: "auth1", Addr: "host:1233", Hostname: "host1"}
	c.Assert(s.clt.UpsertAuthServer(srv, 0), IsNil)

	srv1 = services.Server{ID: "auth2", Addr: "host:1234", Hostname: "host2"}
	c.Assert(s.clt.UpsertAuthServer(srv1, 0), IsNil)

	out, err = s.clt.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})
}

func (s *APISuite) TestReverseTunnels(c *C) {
	out, err := s.clt.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	tunnel := services.ReverseTunnel{DomainName: "example.com", DialAddrs: []string{"example.com:2023"}}
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel, 0), IsNil)

	out, err = s.clt.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.ReverseTunnel{tunnel})

	err = s.clt.DeleteReverseTunnel(tunnel.DomainName)
	c.Assert(err, IsNil)

	out, err = s.clt.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)
}

func (s *APISuite) TestTokens(c *C) {
	out, err := s.clt.GenerateToken(teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)
	c.Assert(len(out), Not(Equals), 0)
}

func (s *APISuite) TestSharedSessions(c *C) {
	out, err := s.clt.GetSessions()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []session.Session{})

	date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	sess := session.Session{
		Active:         true,
		ID:             session.NewID(),
		TerminalParams: session.TerminalParams{W: 100, H: 100},
		Created:        date,
		LastActive:     date,
		Login:          "bob",
	}
	c.Assert(s.clt.CreateSession(sess), IsNil)

	out, err = s.clt.GetSessions()
	c.Assert(err, IsNil)

	c.Assert(out, DeepEquals, []session.Session{sess})

	// emit two events: "one" and "two" for this session, and event "three"
	// for some other session
	s.clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		"val": "one",
	})
	s.clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		"val": "two",
	})
	anotherSessionID := session.NewID()
	s.clt.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
		events.SessionEventID: anotherSessionID,
		"val": "three",
	})
	// ask for strictly session events:
	e, err := s.clt.GetSessionEvents(sess.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(len(e), Equals, 2)
	c.Assert(e[0].GetString("val"), Equals, "one")
	c.Assert(e[1].GetString("val"), Equals, "two")

	// try searching for events with no filter (empty query) - shuld get all 3 events:
	to := time.Now().In(time.UTC).Add(time.Hour)
	from := to.Add(-time.Hour * 2)
	history, err := s.clt.SearchEvents(from, to, "")
	c.Assert(err, IsNil)
	c.Assert(history, NotNil)
	c.Assert(len(history), Equals, 3)

	// try searching for only "session.end" events (real query)
	history, err = s.clt.SearchEvents(from, to,
		fmt.Sprintf("%s=%s", events.EventType, events.SessionEndEvent))
	c.Assert(err, IsNil)
	c.Assert(history, NotNil)
	c.Assert(len(history), Equals, 1)
	c.Assert(history[0].GetString(events.SessionEventID), Equals, string(anotherSessionID))
	c.Assert(history[0].GetString("val"), Equals, "three")
}
