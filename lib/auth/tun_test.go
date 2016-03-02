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
	"path/filepath"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gokyle/hotp"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

type TunSuite struct {
	bk *encryptedbk.ReplicatedBackend

	srv    *APIWithRoles
	tsrv   *AuthTunnel
	a      *AuthServer
	signer ssh.Signer
	bl     *boltlog.BoltLog
	dir    string
	rec    recorder.Recorder
}

var _ = Suite(&TunSuite{})

func (s *TunSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
}

func (s *TunSuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *TunSuite) SetUpTest(c *C) {
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

	sessionServer, err := session.New(s.bk)
	c.Assert(err, IsNil)
	s.a = NewAuthServer(s.bk, authority.New(), "localhost")
	s.srv = NewAPIWithRoles(s.a, s.bl, sessionServer, s.rec,
		NewStandardPermissions(),
		StandardRoles,
	)
	go s.srv.Serve()

	// set up host private key and certificate
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.HostCA, "localhost"), backend.Forever), IsNil)

	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", teleport.RoleNode, 0)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)
	s.signer = signer

	tsrv, err := NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{signer},
		s.srv, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv
}

func (s *TunSuite) TestUnixServerClient(c *C) {
	sessionServer, err := session.New(s.bk)
	c.Assert(err, IsNil)
	srv := NewAPIWithRoles(s.a, s.bl, sessionServer, s.rec,
		NewAllowAllPermissions(),
		StandardRoles,
	)
	go srv.Serve()

	tsrv, err := NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{s.signer},
		srv, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	user := "test"
	pass := []byte("pwd123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()},
		"test", authMethod)
	c.Assert(err, IsNil)

	err = clt.UpsertServer(
		services.Server{ID: "a.example.com", Addr: "hello", Hostname: "hello"}, 0)
	c.Assert(err, IsNil)
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws.ID))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
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
		*services.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "user456"
	user2 := "zxzx"
	user3 := "wrwr"

	mappings := []string{"admin", "db"}

	// Generate token
	token, err := s.a.CreateSignupToken(user, mappings)
	c.Assert(err, IsNil)
	// Generate token2
	token2, err := s.a.CreateSignupToken(user2, mappings)
	c.Assert(err, IsNil)
	// Generate token3
	token3, err := s.a.CreateSignupToken(user3, mappings)
	c.Assert(err, IsNil)

	// Connect to auth server using wrong token
	authMethod0, err := NewSignupTokenAuth("some_wrong_token")
	c.Assert(err, IsNil)

	clt0, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod0)
	c.Assert(err, IsNil)
	_, _, _, err = clt0.GetSignupTokenData(token2)
	c.Assert(err, NotNil) // valid token, but invalid client

	// Connect to auth server using valid token
	authMethod, err := NewSignupTokenAuth(token)
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	// User will scan QRcode, here we just loads the OTP generator
	// right from the backend
	tokenData, ttl, err := s.a.WebService.GetSignupToken(token)
	c.Assert(err, IsNil)
	c.Assert(ttl > SignupTokenUserActionsTTL, Equals, true)
	otp, err := hotp.Unmarshal(tokenData.Hotp)
	c.Assert(err, IsNil)

	hotpTokens := make([]string, 6)
	for i := 0; i < 6; i++ {
		hotpTokens[i] = otp.OTP()
	}

	tokenData3, _, err := s.a.WebService.GetSignupToken(token3)
	c.Assert(err, IsNil)
	otp3, err := hotp.Unmarshal(tokenData3.Hotp)
	c.Assert(err, IsNil)

	hotpTokens3 := make([]string, 6)
	for i := 0; i < 6; i++ {
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
	clt2, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt2.Close()

	password := "valid_password"

	_, err = clt2.CreateUserWithToken(token, password, hotpTokens[0])
	c.Assert(err, IsNil)

	_, err = clt2.CreateUserWithToken(token, "another_user_signup_attempt", hotpTokens[0])
	c.Assert(err, NotNil)

	_, _, err = s.a.WebService.GetSignupToken(token)
	c.Assert(err, NotNil) // token was deleted

	_, err = clt2.CreateUserWithToken(token3, "newpassword123", hotpTokens3[5])
	c.Assert(err, NotNil)

	_, err = clt2.CreateUserWithToken(token3, "newpassword45665", hotpTokens3[4])
	c.Assert(err, IsNil)

	// trying to connect to the auth server using used token
	clt0, err = NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil) // shouldn't accept such connection twice
	_, _, _, err = clt0.GetSignupTokenData(token2)
	c.Assert(err, NotNil) // valid token, but invalid client

	// User was created. Now trying to login

	authMethod3, err := NewWebPasswordAuth(user, []byte(password), hotpTokens[1])
	c.Assert(err, IsNil)

	clt3, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod3)
	c.Assert(err, IsNil)
	defer clt3.Close()

	ws, err := clt3.SignIn(user, []byte(password))
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")
}

func (s *TunSuite) TestPermissions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	user := "ws-test2"
	pass := []byte("ws-abc1234")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test2")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	// Requesting forbidded for User action
	_, err = clt.GetServers()
	c.Assert(err, NotNil)

	// Requesting forbidded for User action
	_, err = clt.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, NotNil)

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws.ID))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	// Requesting forbidden for Web action
	_, err = cltw.GetServers()
	c.Assert(err, NotNil)

	// Requesting forbidden for Web action
	_, err = cltw.SignIn(user, pass)
	c.Assert(err, NotNil)

	out, err := cltw.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = cltw.DeleteWebSession(user, ws.ID)
	c.Assert(err, IsNil)

	_, err = clt.GetWebSessionInfo(user, ws.ID)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestSessionsBadPassword(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

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

	clt, err := NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, []byte("different-pass"))
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)

	ws, err = clt.SignIn("not-exists", pass)
	c.Assert(err, NotNil)
	c.Assert(ws, IsNil)
}
