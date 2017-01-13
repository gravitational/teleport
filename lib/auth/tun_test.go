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
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gokyle/hotp"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

type TunSuite struct {
	bk backend.Backend

	tsrv          *AuthTunnel
	a             *AuthServer
	signer        ssh.Signer
	dir           string
	alog          events.IAuditLog
	conf          *APIConfig
	sessionServer session.Service
}

var _ = Suite(&TunSuite{})

func (s *TunSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *TunSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.bk, err = dir.New(backend.Params{"path": s.dir})
	c.Assert(err, IsNil)

	s.alog, err = events.NewAuditLog(s.dir)
	c.Assert(err, IsNil)

	s.sessionServer, err = session.New(s.bk)
	c.Assert(err, IsNil)

	access := local.NewAccessService(s.bk)
	identity := local.NewIdentityService(s.bk)

	s.a = NewAuthServer(&InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: "localhost",
		Access:     access,
		Identity:   identity,
	})

	// set up host private key and certificate
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "localhost"), backend.Forever), IsNil)

	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)

	newChecker, err := NewAccessChecker(s.a.Access, s.a.Identity)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)
	s.signer = signer
	s.conf = &APIConfig{
		AuthServer:     s.a,
		NewChecker:     newChecker,
		SessionService: s.sessionServer,
		AuditLog:       s.alog,
	}

	tsrv, err := NewTunnel(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}, signer, s.conf)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv
}

func (s *TunSuite) TestUnixServerClient(c *C) {
	newChecker, err := NewAccessChecker(s.a.Access, s.a.Identity)
	c.Assert(err, IsNil)

	tsrv, err := NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.signer,
		&APIConfig{
			AuthServer:     s.a,
			NewChecker:     newChecker,
			SessionService: s.sessionServer,
			AuditLog:       s.alog,
		},
	)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	userName := "test"
	pass := []byte("pwd123")

	user, role := createUserAndRole(s.a, userName, []string{userName})
	role.SetResource(services.KindNode, services.RW())
	err = s.a.UpsertRole(role)
	c.Assert(err, IsNil)

	hotpURL, _, err := s.a.UpsertPassword(user.GetName(), pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user.GetName(), pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}},
		"test", authMethod)
	c.Assert(err, IsNil)

	err = clt.UpsertNode(newServer(services.KindNode, "a.example.com", "hello", "hello", defaults.Namespace), backend.Forever)
	c.Assert(err, IsNil)
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")

	createUserAndRole(s.a, user, []string{user})

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws.ID))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	out, err := cltw.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = cltw.DeleteWebSession(user, ws.ID)
	c.Assert(err, IsNil)

	_, err = clt.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestWebCreatingNewUser(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "user456"
	user2 := "zxzx"
	user3 := "wrwr"

	mappings := []string{"admin", "db"}

	// Generate token
	token, err := s.a.CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings})
	c.Assert(err, IsNil)
	// Generate token2
	token2, err := s.a.CreateSignupToken(services.UserV1{Name: user2, AllowedLogins: mappings})
	c.Assert(err, IsNil)
	// Generate token3
	token3, err := s.a.CreateSignupToken(services.UserV1{Name: user3, AllowedLogins: mappings})
	c.Assert(err, IsNil)

	// Connect to auth server using wrong token
	authMethod0, err := NewSignupTokenAuth("some_wrong_token")
	c.Assert(err, IsNil)

	clt0, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod0)
	c.Assert(err, IsNil)
	_, _, _, err = clt0.GetSignupTokenData(token2)
	c.Assert(err, NotNil) // valid token, but invalid client

	// Connect to auth server using valid token
	authMethod, err := NewSignupTokenAuth(token)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	// User will scan QRcode, here we just loads the OTP generator
	// right from the backend
	tokenData, err := s.a.Identity.GetSignupToken(token)
	c.Assert(err, IsNil)
	otp, err := hotp.Unmarshal(tokenData.Hotp)
	c.Assert(err, IsNil)

	hotpTokens := make([]string, defaults.HOTPFirstTokensRange)
	for i := 0; i < defaults.HOTPFirstTokensRange; i++ {
		hotpTokens[i] = otp.OTP()
	}

	tokenData3, err := s.a.Identity.GetSignupToken(token3)
	c.Assert(err, IsNil)
	otp3, err := hotp.Unmarshal(tokenData3.Hotp)
	c.Assert(err, IsNil)

	hotpTokens3 := make([]string, defaults.HOTPFirstTokensRange+1)
	for i := 0; i < defaults.HOTPFirstTokensRange+1; i++ {
		hotpTokens3[i] = otp3.OTP()
	}

	// Loading what the web page loads (username and QR image)
	_, _, _, err = clt.GetSignupTokenData("wrong_token")
	c.Assert(err, NotNil)

	_, err = clt.GetUsers() //no permissions
	c.Assert(err, NotNil)

	user1, _, _, err := clt.GetSignupTokenData(token)
	c.Assert(err, IsNil)
	c.Assert(user, Equals, user1)

	// Saving new password
	clt2, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt2.Close()

	password := "valid_password"

	_, err = clt2.CreateUserWithToken(token, password, hotpTokens[0])
	c.Assert(err, IsNil)

	_, err = clt2.CreateUserWithToken(token, "another_user_signup_attempt", hotpTokens[0])
	c.Assert(err, NotNil)

	_, err = s.a.Identity.GetSignupToken(token)
	c.Assert(err, NotNil) // token was deleted

	// token out of scan range
	_, err = clt2.CreateUserWithToken(token3, "newpassword123", hotpTokens3[defaults.HOTPFirstTokensRange])
	c.Assert(err, NotNil)

	_, err = clt2.CreateUserWithToken(token3, "newpassword45665", hotpTokens3[2])
	c.Assert(err, IsNil)

	// trying to connect to the auth server using used token
	clt0, err = NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil) // shouldn't accept such connection twice
	_, _, _, err = clt0.GetSignupTokenData(token2)
	c.Assert(err, NotNil) // valid token, but invalid client

	// User was created. Now trying to login
	authMethod3, err := NewWebPasswordAuth(user, []byte(password), hotpTokens[1])
	c.Assert(err, IsNil)

	clt3, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod3)
	c.Assert(err, IsNil)
	defer clt3.Close()

	ws, err := clt3.SignIn(user, []byte(password))
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")
}

func (s *TunSuite) TestPermissions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	userName := "ws-test2"
	pass := []byte("ws-abc1234")

	user, _ := createUserAndRole(s.a, userName, []string{userName})

	hotpURL, _, err := s.a.UpsertPassword(user.GetName(), pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test2")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user.GetName(), pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user.GetName(), authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user.GetName(), pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	// Requesting forbidden for User action
	server := newServer(services.KindNode, "name", "host", "host", defaults.Namespace)
	err = clt.UpsertNode(server, time.Second)
	c.Assert(err, NotNil)

	// Requesting forbidden for User action
	_, err = clt.GetWebSessionInfo(user.GetName(), ws.ID)
	c.Assert(err, NotNil)

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user.GetName(), []byte(ws.ID))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user.GetName(), authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	// Requesting forbidden for Web action
	_, err = cltw.GetNodes(defaults.Namespace)
	c.Assert(err, NotNil)

	// Requesting sign in forbidden for Web action
	_, err = cltw.SignIn(user.GetName(), pass)
	c.Assert(err, NotNil)

	out, err := cltw.GetWebSessionInfo(user.GetName(), ws.ID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = cltw.DeleteWebSession(user.GetName(), ws.ID)
	c.Assert(err, IsNil)

	_, err = clt.GetWebSessionInfo(user.GetName(), ws.ID)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestSessionsBadPassword(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "system-test"
	pass := []byte("system-abc123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "system-test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, []byte("different-pass"))
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)

	ws, err = clt.SignIn("not-exists", pass)
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)
}

func (s *TunSuite) TestFailover(c *C) {
	node := newServer(
		services.KindNode,
		"node1",
		"node.example.com:12345",
		"node.example.com",
		defaults.Namespace,
	)
	c.Assert(s.a.UpsertNode(node, backend.Forever), IsNil)

	ports, err := utils.GetFreeTCPPorts(1)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{
			{AddrNetwork: "tcp", Addr: fmt.Sprintf("127.0.0.1:%v", ports.Pop())},
			{AddrNetwork: "tcp", Addr: s.tsrv.Addr()},
		}, "localhost", []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer clt.Close()

	nodes, err := clt.GetNodes(defaults.Namespace)
	c.Assert(err, IsNil)
	c.Assert(nodes, DeepEquals, []services.Server{node})
}

func (s *TunSuite) TestSync(c *C) {
	authServer := newServer(
		services.KindAuthServer,
		"node1",
		"node.example.com:12345",
		"node.example.com",
		defaults.Namespace,
	)
	c.Assert(s.a.UpsertAuthServer(authServer, backend.Forever), IsNil)

	storage := utils.NewFileAddrStorage(filepath.Join(c.MkDir(), "addr.json"))

	// authAddr is 'statically' configured CA address:
	authAddr := s.tsrv.Addr()

	clt, err := NewTunClient("test",
		[]utils.NetAddr{
			{AddrNetwork: "tcp", Addr: authAddr},
		}, "localhost", []ssh.AuthMethod{ssh.PublicKeys(s.signer)},
		TunClientStorage(storage),
	)
	c.Assert(err, IsNil)
	defer clt.Close()

	err = clt.fetchAndSync()
	c.Assert(err, IsNil)

	allServers := []utils.NetAddr{
		{Addr: authAddr, AddrNetwork: "tcp", Path: ""},
		{Addr: "node.example.com:12345", AddrNetwork: "tcp"},
	}
	discoveredServers := []utils.NetAddr{
		{Addr: "node.example.com:12345", AddrNetwork: "tcp"},
	}
	c.Assert(clt.getAuthServers(), DeepEquals, allServers)

	syncedServers, err := storage.GetAddresses()
	c.Assert(err, IsNil)
	c.Assert(syncedServers, DeepEquals, discoveredServers)

	// test sorting
	unsorted := []utils.NetAddr{
		{Addr: "2", AddrNetwork: "udp"},
		{Addr: "1", AddrNetwork: "tcp"},
		{Addr: "4", AddrNetwork: "smtp"},
		{Addr: "3", AddrNetwork: "http"},
	}
	clt.setAuthServers(unsorted)
	sorted := clt.getAuthServers()
	c.Assert(sorted[0].Addr, Equals, authAddr) // the statically set CA addr is always 1st
	c.Assert(sorted[1].Addr, Equals, "1")
	c.Assert(sorted[2].Addr, Equals, "2")
	c.Assert(sorted[3].Addr, Equals, "3")
}
