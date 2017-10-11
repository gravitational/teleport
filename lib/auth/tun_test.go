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
	"encoding/base32"
	"fmt"
	"net/url"
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

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

var _ = fmt.Printf // for testing

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
		Backend:   s.bk,
		Authority: authority.New(),
		Access:    access,
		Identity:  identity,
	})

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	c.Assert(err, IsNil)
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	// create the default role
	c.Assert(s.a.UpsertRole(services.NewAdminRole(false), backend.Forever), IsNil)

	// set up host private key and certificate
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "localhost")), IsNil)

	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "00000000-0000-0000-0000-000000000000", "localhost", "localhost", teleport.Roles{teleport.RoleNode}, 0)
	c.Assert(err, IsNil)

	authorizer, err := NewAuthorizer(s.a.Access, s.a.Identity, s.a.Trust)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)
	s.signer = signer
	s.conf = &APIConfig{
		AuthServer:     s.a,
		Authorizer:     authorizer,
		SessionService: s.sessionServer,
		AuditLog:       s.alog,
	}

	tsrv, err := NewTunnel(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}, signer, s.conf)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv
}

func (s *TunSuite) TestUnixServerClient(c *C) {
	authorizer, err := NewAuthorizer(s.a.Access, s.a.Identity, s.a.Trust)
	c.Assert(err, IsNil)

	tsrv, err := NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.signer,
		&APIConfig{
			AuthServer:     s.a,
			Authorizer:     authorizer,
			SessionService: s.sessionServer,
			AuditLog:       s.alog,
		},
	)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	userName := "test"
	pass := []byte("pwd123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	user, role := createUserAndRole(s.a, userName, []string{userName})
	rules := role.GetRules(services.Allow)
	rules = append(rules, services.NewRule(services.KindNode, services.RW()))
	role.SetRules(services.Allow, rules)
	err = s.a.UpsertRole(role, backend.Forever)
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user.GetName(), pass)
	c.Assert(err, IsNil)

	err = s.a.UpsertTOTP(user.GetName(), otpSecret)
	c.Assert(err, IsNil)

	otpURL, _, err := s.a.GetOTPData(userName)
	c.Assert(err, IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, IsNil)
	c.Assert(u.Path, Equals, "/test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	authMethod, err := NewWebPasswordAuth(user.GetName(), pass, validToken)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}},
		"test", authMethod)
	c.Assert(err, IsNil)

	// call some endpoint
	_, err = clt.GetUser(userName)
	c.Assert(err, IsNil)
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	createUserAndRole(s.a, user, []string{user})

	err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	err = s.a.UpsertTOTP(user, otpSecret)
	c.Assert(err, IsNil)

	otpURL, _, err := s.a.GetOTPData(user)
	c.Assert(err, IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, IsNil)
	c.Assert(u.Path, Equals, "/ws-test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	authMethod, err := NewWebPasswordAuth(user, pass, validToken)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws.GetName()))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	out, err := cltw.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = cltw.DeleteWebSession(user, ws.GetName())
	c.Assert(err, IsNil)

	_, err = clt.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, NotNil)
}

// TestWebCreatingNewUserInvalidClientValidToken tries to connect to the auth server
// using an invalid singup token but then tries to pull back signup token data
// using a valid token. This should fail.
func (s *TunSuite) TestWebCreatingNewUserInvalidClientValidToken(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "foobar"
	mappings := []string{"admin", "db"}

	token, err := s.a.CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings})
	c.Assert(err, IsNil)

	authMethod, err := NewSignupTokenAuth("invalid_signup_token")
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	_, _, err = clt.GetSignupTokenData(token)
	c.Assert(err, NotNil)
}

// TestWebCreatingNewUserValidClientInvalidToken connects to the auth server using a
// valid signup token but then tries to get invalid token data back. This should fail.
func (s *TunSuite) TestWebCreatingNewUserValidClientInvalidToken(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "foobar"
	mappings := []string{"admin", "db"}

	token, err := s.a.CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings})
	c.Assert(err, IsNil)

	authMethod, err := NewSignupTokenAuth(token)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	_, _, err = clt.GetSignupTokenData("invalid_signup_token")
	c.Assert(err, NotNil)

	// no permissions to do this
	_, err = clt.GetUsers()
	c.Assert(err, NotNil)
}

// TestWebCreatingNewUserValidClientValidToken connects to the auth server using a
// valid signup token and then tries to get a valid token back. Then try and login
// as the new user. This should all succeed.
func (s *TunSuite) TestWebCreatingNewUserValidClientValidToken(c *C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, IsNil)
	err = s.a.SetAuthPreference(ap)
	c.Assert(err, IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "foobar"
	password := "bazqux"
	mappings := []string{"admin", "db"}

	token, err := s.a.CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings})
	c.Assert(err, IsNil)

	authMethod, err := NewSignupTokenAuth(token)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	// check that the usernames are the same
	userInToken, _, err := clt.GetSignupTokenData(token)
	c.Assert(err, IsNil)
	c.Assert(user, Equals, userInToken)

	// get otp token
	tokenData, err := s.a.Identity.GetSignupToken(token)
	validToken, err := totp.GenerateCode(tokenData.OTPKey, time.Now())
	c.Assert(err, IsNil)

	// create a user
	_, err = clt.CreateUserWithOTP(token, password, validToken)
	c.Assert(err, IsNil)

	// delete token so we can re-use it without messing with clocks
	err = s.a.Identity.DeleteUsedTOTPToken(user)
	c.Assert(err, IsNil)

	// now login as the fresh new user
	authMethod, err = NewWebPasswordAuth(user, []byte(password), validToken)
	c.Assert(err, IsNil)

	clt, err = NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, []byte(password))
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")
}

// TestWebCreatingNewUserValidClientValidTokenReuseToken connects to teh auth server
// using a valid signup token and then uses a valid token to create a user. Then
// try to create another user. This should fail.
func (s *TunSuite) TestWebCreatingNewUserValidClientValidTokenReuseToken(c *C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, IsNil)
	err = s.a.SetAuthPreference(ap)
	c.Assert(err, IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "foobar"
	mappings := []string{"admin", "db"}

	token, err := s.a.CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings})
	c.Assert(err, IsNil)

	authMethod, err := NewSignupTokenAuth(token)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	validPassword := "valid_password"
	tokenData, err := s.a.Identity.GetSignupToken(token)
	validToken, err := totp.GenerateCode(tokenData.OTPKey, time.Now())
	c.Assert(err, IsNil)

	// first time we should be able to create a user
	_, err = clt.CreateUserWithOTP(token, validPassword, validToken)
	c.Assert(err, IsNil)

	// second time it should fail
	_, err = clt.CreateUserWithOTP(token, validPassword, validToken)
	c.Assert(err, NotNil)

	// signup token should be gone now
	_, err = s.a.Identity.GetSignupToken(token)
	c.Assert(err, NotNil)

	// try and connect again this should fail as well
	clt, err = NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod)
	c.Assert(err, IsNil)

	_, _, err = clt.GetSignupTokenData(token)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestPermissions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	userName := "ws-test2"
	pass := []byte("ws-abc1234")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	user, _ := createUserAndRole(s.a, userName, []string{userName})

	err := s.a.UpsertPassword(user.GetName(), pass)
	c.Assert(err, IsNil)

	err = s.a.UpsertTOTP(user.GetName(), otpSecret)
	c.Assert(err, IsNil)

	otpURL, _, err := s.a.GetOTPData(userName)
	c.Assert(err, IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, IsNil)
	c.Assert(u.Path, Equals, "/ws-test2")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	authMethod, err := NewWebPasswordAuth(user.GetName(), pass, validToken)
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
	err = clt.UpsertNode(server)
	c.Assert(err, NotNil)

	// Requesting forbidden for User action
	err = clt.DeleteUser(user.GetName())
	c.Assert(err, NotNil)

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user.GetName(), []byte(ws.GetName()))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user.GetName(), authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	// Requesting forbidden for Web action
	err = cltw.DeleteUser(user.GetName())
	c.Assert(err, NotNil)

	out, err := cltw.GetWebSessionInfo(user.GetName(), ws.GetName())
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = cltw.DeleteWebSession(user.GetName(), ws.GetName())
	c.Assert(err, IsNil)

	_, err = clt.GetWebSessionInfo(user.GetName(), ws.GetName())
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestSessionsBadPassword(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "system-test"
	pass := []byte("system-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	err = s.a.UpsertTOTP(user, otpSecret)
	c.Assert(err, IsNil)

	otpURL, _, err := s.a.GetOTPData(user)
	c.Assert(err, IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, IsNil)
	c.Assert(u.Path, Equals, "/system-test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	authMethod, err := NewWebPasswordAuth(user, pass, validToken)
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

// TestLoginAttempts makes sure the login attempt counter is incremented and
// reset correctly.
func (s *TunSuite) TestLoginAttempts(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "localhost")), IsNil)

	user := "system-test"
	pass := []byte("system-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	err = s.a.UpsertTOTP(user, otpSecret)
	c.Assert(err, IsNil)

	otpURL, _, err := s.a.GetOTPData(user)
	c.Assert(err, IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, IsNil)
	c.Assert(u.Path, Equals, "/system-test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	// try first to login with an invalid password
	authMethod, err := NewWebPasswordAuth(user, []byte("invalid-password"), validToken)
	c.Assert(err, IsNil)

	clt, err := NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod, TunDisableRefresh())
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)

	// we can make any request and don't care about the result. the code keyAuth
	// code we care about is run during the ssh handshake.
	clt.GetUsers()
	err = clt.Close()
	c.Assert(err, IsNil)

	// should only create a single failed login attempt
	loginAttempts, err := s.a.GetUserLoginAttempts(user)
	c.Assert(err, IsNil)
	c.Assert(loginAttempts, HasLen, 1)

	// try again with the correct password
	authMethod, err = NewWebPasswordAuth(user, pass, validToken)
	c.Assert(err, IsNil)

	clt, err = NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}}, user, authMethod, TunDisableRefresh())
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)

	// once again, we can make any request and don't care about the result. the
	// code keyAuth code we care about is run during the ssh handshake.
	clt.GetUsers()
	err = clt.Close()
	c.Assert(err, IsNil)

	// login was successful, attempts should be reset back to 0
	loginAttempts, err = s.a.GetUserLoginAttempts(user)
	c.Assert(err, IsNil)
	c.Assert(loginAttempts, HasLen, 0)
}

func (s *TunSuite) TestFailover(c *C) {
	node := newServer(
		services.KindNode,
		"node1",
		"node.example.com:12345",
		"node.example.com",
		defaults.Namespace,
	)
	c.Assert(s.a.UpsertNode(node), IsNil)

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
	c.Assert(s.a.UpsertAuthServer(authServer), IsNil)

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
