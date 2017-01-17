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
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/davecgh/go-spew/spew"
	"github.com/gokyle/hotp"
	"github.com/kylelemons/godebug/diff"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

type APISuite struct {
	srv      *httptest.Server
	clt      *Client
	bk       backend.Backend
	a        *AuthServer
	alog     events.IAuditLog
	sessions session.Service

	CAS           services.Trust
	PresenceS     services.Presence
	ProvisioningS services.Provisioner
	WebS          services.Identity
	AccessS       services.Access
}

var _ = Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
	authority.PrecalculatedKeysNum = 1
}

func (s *APISuite) SetUpTest(c *C) {
	dir := c.MkDir()
	var err error

	s.bk, err = boltbk.New(backend.Params{"path": dir})
	c.Assert(err, IsNil)

	s.alog, err = events.NewAuditLog(dir)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(&InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: "localhost",
	})
	s.sessions, err = session.New(s.bk)
	c.Assert(err, IsNil)

	s.AccessS = local.NewAccessService(s.bk)
	s.WebS = local.NewIdentityService(s.bk)

	newChecker, err := NewAccessChecker(s.AccessS, s.WebS)
	c.Assert(err, IsNil)

	apiServer := NewAPIServer(&APIConfig{
		AuthServer:     s.a,
		NewChecker:     newChecker,
		SessionService: s.sessions,
		AuditLog:       s.alog,
	})
	s.srv = httptest.NewServer(apiServer)

	// apiserver receives authentication information passed by SSH,
	// this is to pass auth info to auth user
	clt, err := NewClient(s.srv.URL, nil, roundtrip.BasicAuth(teleport.RoleAdmin.User(), "<something>"))
	c.Assert(err, IsNil)
	s.clt = clt

	s.CAS = local.NewCAService(s.bk)
	s.PresenceS = local.NewPresenceService(s.bk)
	s.ProvisioningS = local.NewProvisioningService(s.bk)
}

func (s *APISuite) TearDownTest(c *C) {
	fileBasedLog, ok := s.alog.(*events.AuditLog)
	c.Assert(ok, Equals, true)
	if ok {
		fileBasedLog.Close()
	}
	s.srv.Close()
}

type clt interface {
	UpsertRole(services.Role) error
	UpsertUser(services.User) error
}

func createUserAndRole(clt clt, username string, allowedLogins []string) (services.User, services.Role) {
	user, err := services.NewUser(username)
	if err != nil {
		panic(err)
	}
	role := services.RoleForUser(user)
	role.SetLogins([]string{user.GetName()})
	err = clt.UpsertRole(role)
	if err != nil {
		panic(err)
	}
	user.AddRole(role.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		panic(err)
	}
	return user, role
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
		suite.NewTestCA(services.HostCA, "localhost"), backend.Forever), IsNil)

	_, pub, err = s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	cert, err := s.clt.GenerateHostCert(
		pub, "localhost", "localhost", teleport.Roles{teleport.RoleNode}, time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	c.Assert(s.clt.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	_, pub, err = s.clt.GenerateKeyPair("")
	c.Assert(err, IsNil)

	user1, _ := createUserAndRole(s.clt, "user1", []string{"user1"})
	user2, _ := createUserAndRole(s.clt, "user2", []string{"user2"})
	_, _, err = s.clt.UpsertPassword(user1.GetName(), []byte("abc1231"))
	c.Assert(err, IsNil)
	_, _, err = s.clt.UpsertPassword(user2.GetName(), []byte("abc1232"))
	c.Assert(err, IsNil)

	newChecker, err := NewAccessChecker(s.AccessS, s.WebS)
	c.Assert(err, IsNil)

	userServer := NewAPIServer(&APIConfig{
		AuthServer:     s.a,
		NewChecker:     newChecker,
		SessionService: s.sessions,
		AuditLog:       s.alog,
	})
	authServer := httptest.NewServer(userServer)
	defer authServer.Close()

	userClient, err := NewClient(authServer.URL, nil)
	c.Assert(err, IsNil)

	// should NOT be able to generate a user cert without basic HTTP auth
	cert, err = userClient.GenerateUserCert(pub, "user1", time.Hour)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, ".*username or password")

	// Users don't match
	roundtrip.BasicAuth("user2", "two")(&userClient.Client)
	cert, err = userClient.GenerateUserCert(pub, "user1", time.Hour)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, ".*cannot request a certificate for user1")

	// should not be able to generate cert for longer than duration
	roundtrip.BasicAuth("user1", "two")(&userClient.Client)
	cert, err = userClient.GenerateUserCert(pub, "user1", 40*time.Hour)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, ".*cannot request a certificate for 40h0m0s")

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
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	createUserAndRole(s.clt, user, []string{user})

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

func newServer(kind string, name, addr, hostname, namespace string) services.Server {
	return &services.ServerV2{
		Kind:    kind,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: services.ServerSpecV2{
			Addr:     addr,
			Hostname: hostname,
		},
	}
}

func (s *APISuite) TestServers(c *C) {
	out, err := s.clt.GetNodes(defaults.Namespace)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := newServer(services.KindNode, "id1", "host:1233", "host1", defaults.Namespace)
	c.Assert(s.clt.UpsertNode(srv, 0), IsNil)

	srv1 := newServer(services.KindNode, "id2", "host:1234", "host2", defaults.Namespace)
	c.Assert(s.clt.UpsertNode(srv1, 0), IsNil)

	out, err = s.clt.GetNodes(defaults.Namespace)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})

	out, err = s.clt.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv = newServer(services.KindProxy, "proxy1", "host:1233", "host1", defaults.Namespace)
	c.Assert(s.clt.UpsertProxy(srv, 0), IsNil)

	srv1 = newServer(services.KindProxy, "proxy2", "host:1234", "host2", defaults.Namespace)
	c.Assert(s.clt.UpsertProxy(srv1, 0), IsNil)

	out, err = s.clt.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})

	out, err = s.clt.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv = newServer(services.KindAuthServer, "auth1", "host:1233", "host1", defaults.Namespace)
	c.Assert(s.clt.UpsertAuthServer(srv, 0), IsNil)

	srv1 = newServer(services.KindAuthServer, "auth2", "host:1234", "host2", defaults.Namespace)
	c.Assert(s.clt.UpsertAuthServer(srv1, 0), IsNil)

	out, err = s.clt.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv, srv1})
}

func (s *APISuite) TestReverseTunnels(c *C) {
	out, err := s.clt.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	tunnel := &services.ReverseTunnelV2{
		Kind:     services.KindReverseTunnel,
		Metadata: services.Metadata{Name: "example.com", Namespace: defaults.Namespace},
		Version:  services.V2,
		Spec: services.ReverseTunnelSpecV2{
			ClusterName: "example.com",
			DialAddrs:   []string{"example.com:2023"},
		},
	}
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel, 0), IsNil)

	d := &spew.ConfigState{Indent: " ", DisableMethods: true, DisablePointerMethods: true, DisablePointerAddresses: true}
	out, err = s.clt.GetReverseTunnels()
	c.Assert(err, IsNil)
	expected := []services.ReverseTunnel{tunnel}
	c.Assert(out, DeepEquals, expected, Commentf("%v", diff.Diff(d.Sdump(out), d.Sdump(expected))))

	err = s.clt.DeleteReverseTunnel(tunnel.GetName())
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
	out, err := s.clt.GetSessions(defaults.Namespace)
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
		Namespace:      defaults.Namespace,
	}
	c.Assert(s.clt.CreateSession(sess), IsNil)

	out, err = s.clt.GetSessions(defaults.Namespace)
	c.Assert(err, IsNil)

	c.Assert(out, DeepEquals, []session.Session{sess})

	// emit two events: "one" and "two" for this session, and event "three"
	// for some other session
	s.clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		events.EventNamespace: defaults.Namespace,
		"val": "one",
	})
	s.clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		events.EventNamespace: defaults.Namespace,
		"val": "two",
	})
	anotherSessionID := session.NewID()
	s.clt.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
		events.SessionEventID: anotherSessionID,
		"val": "three",
		events.EventNamespace: defaults.Namespace,
	})
	// ask for strictly session events:
	e, err := s.clt.GetSessionEvents(defaults.Namespace, sess.ID, 0)
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
