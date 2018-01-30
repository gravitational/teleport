/*
Copyright 2017 Gravitational, Inc.

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
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/pquerna/otp/totp"
	"gopkg.in/check.v1"
)

type TLSSuite struct {
	server *TestTLSServer
}

var _ = check.Suite(&TLSSuite{})

func (s *TLSSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *TLSSuite) SetUpTest(c *check.C) {
	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: c.MkDir(),
	})
	c.Assert(err, check.IsNil)
	s.server, err = testAuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
}

func (s *TLSSuite) TearDownTest(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

// TestRemoteBuiltinRole tests remote builtin role
// that gets mapped to remote proxy readonly role
func (s *TLSSuite) TestRemoteBuiltinRole(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// without trust, proxy server will get rejected
	// remote auth server will get rejected because it is not supported
	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = remoteProxy.GetNodes(defaults.Namespace)
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)

	_, err = remoteProxy.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)

	// remote auth server will get rejected even with established trust
	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = remoteAuth.GetDomainName()
	fixtures.ExpectAccessDenied(c, err)
}

// TestRemoteUser tests scenario when remote user connects to the local
// auth server and some edge cases.
func (s *TLSSuite) TestRemoteUser(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	remoteUser, remoteRole, err := CreateUserAndRole(remoteServer.AuthServer, "remote-user", []string{"remote-role"})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	remoteClient, err := remoteServer.NewRemoteClient(
		TestUser(remoteUser.GetName()), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// User is not authorized to perform any actions
	// as local cluster does not trust the remote cluster yet
	_, err = remoteClient.GetDomainName()
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// Establish trust, the request will still fail, there is
	// no role mapping set up
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)
	_, err = remoteClient.GetDomainName()
	fixtures.ExpectAccessDenied(c, err)

	// Establish trust and map remote role to local admin role
	_, localRole, err := CreateUserAndRole(s.server.Auth(), "local-user", []string{"local-role"})
	c.Assert(err, check.IsNil)

	err = s.server.AuthServer.Trust(remoteServer, services.RoleMap{{Remote: remoteRole.GetName(), Local: []string{localRole.GetName()}}})
	c.Assert(err, check.IsNil)

	_, err = remoteClient.GetDomainName()
	c.Assert(err, check.IsNil)
}

// TestNopUser tests user with no permissions except
// the ones that require other authentication methods ("nop" user)
func (s *TLSSuite) TestNopUser(c *check.C) {
	client, err := s.server.NewClient(TestNop())
	c.Assert(err, check.IsNil)

	// Nop User can get cluster name
	_, err = client.GetDomainName()
	c.Assert(err, check.IsNil)

	// But can not get users or nodes
	_, err = client.GetUsers()
	fixtures.ExpectAccessDenied(c, err)

	_, err = client.GetNodes(defaults.Namespace)
	fixtures.ExpectAccessDenied(c, err)
}

// TestOwnRole tests that user can read roles assigned to them
func (s *TLSSuite) TestReadOwnRole(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user1, userRole, err := CreateUserAndRoleWithoutRoles(clt, "user1", []string{"user1"})
	c.Assert(err, check.IsNil)

	user2, _, err := CreateUserAndRoleWithoutRoles(clt, "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	// user should be able to read their own roles
	userClient, err := s.server.NewClient(TestUser(user1.GetName()))
	c.Assert(err, check.IsNil)

	_, err = userClient.GetRole(userRole.GetName())
	c.Assert(err, check.IsNil)

	// user2 can't read user1 role
	userClient2, err := s.server.NewClient(TestIdentity{I: LocalUser{Username: user2.GetName()}})
	c.Assert(err, check.IsNil)

	_, err = userClient2.GetRole(userRole.GetName())
	fixtures.ExpectAccessDenied(c, err)
}

func (s *TLSSuite) TestTunnelConnectionsCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.TunnelConnectionsCRUD(c)
}

func (s *TLSSuite) TestRemoteClustersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.RemoteClustersCRUD(c)
}

func (s *TLSSuite) TestServersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ServerCRUD(c)
}

func (s *TLSSuite) TestReverseTunnelsCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ReverseTunnelsCRUD(c)
}

func (s *TLSSuite) TestUsersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	err = clt.UpsertPassword("user1", []byte("some pass"))
	c.Assert(err, check.IsNil)

	users, err := clt.GetUsers()
	c.Assert(err, check.IsNil)
	c.Assert(len(users), check.Equals, 1)
	c.Assert(users[0].GetName(), check.Equals, "user1")

	c.Assert(clt.DeleteUser("user1"), check.IsNil)

	users, err = clt.GetUsers()
	c.Assert(err, check.IsNil)
	c.Assert(len(users), check.Equals, 0)
}

func (s *TLSSuite) TestPasswordGarbage(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)
	garbage := [][]byte{
		nil,
		make([]byte, defaults.MaxPasswordLength+1),
		make([]byte, defaults.MinPasswordLength-1),
	}
	for _, g := range garbage {
		err := clt.CheckPassword("user1", g, "123456")
		fixtures.ExpectBadParameter(c, err)
	}
}

func (s *TLSSuite) TestPasswordCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	err = clt.CheckPassword("user1", pass, "123456")
	c.Assert(err, check.NotNil)

	err = clt.UpsertPassword("user1", pass)
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertTOTP("user1", otpSecret)
	c.Assert(err, check.IsNil)

	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.IsNil)
}

func (s *TLSSuite) TestTokens(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}})
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Not(check.Equals), 0)
}

func (s *TLSSuite) TestSharedSessions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GetSessions(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, []session.Session{})

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
	c.Assert(clt.CreateSession(sess), check.IsNil)

	out, err = clt.GetSessions(defaults.Namespace)
	c.Assert(err, check.IsNil)

	c.Assert(out, check.DeepEquals, []session.Session{sess})

	// emit two events: "one" and "two" for this session, and event "three"
	// for some other session
	err = clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		events.EventNamespace: defaults.Namespace,
		"val": "one",
	})
	c.Assert(err, check.IsNil)
	err = clt.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID: sess.ID,
		events.EventNamespace: defaults.Namespace,
		"val": "two",
	})
	c.Assert(err, check.IsNil)
	anotherSessionID := session.NewID()
	err = clt.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
		events.SessionEventID: anotherSessionID,
		"val": "three",
		events.EventNamespace: defaults.Namespace,
	})
	c.Assert(err, check.IsNil)
	// ask for strictly session events:
	e, err := clt.GetSessionEvents(defaults.Namespace, sess.ID, 0)
	c.Assert(err, check.IsNil)
	c.Assert(len(e), check.Equals, 2)
	c.Assert(e[0].GetString("val"), check.Equals, "one")
	c.Assert(e[1].GetString("val"), check.Equals, "two")

	// try searching for events with no filter (empty query) - should get all 3 events:
	to := time.Now().In(time.UTC).Add(time.Hour)
	from := to.Add(-time.Hour * 2)
	history, err := clt.SearchEvents(from, to, "")
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	c.Assert(len(history), check.Equals, 3)

	// try searching for only "session.end" events (real query)
	history, err = clt.SearchEvents(from, to,
		fmt.Sprintf("%s=%s", events.EventType, events.SessionEndEvent))
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	c.Assert(len(history), check.Equals, 1)
	c.Assert(history[0].GetString(events.SessionEventID), check.Equals, string(anotherSessionID))
	c.Assert(history[0].GetString("val"), check.Equals, "three")
}

func (s *TLSSuite) TestOTPCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	// upsert a password and totp secret
	err = clt.UpsertPassword("user1", pass)
	c.Assert(err, check.IsNil)
	err = s.server.Auth().UpsertTOTP(user, otpSecret)
	c.Assert(err, check.IsNil)

	// make sure the otp url we get back is valid url issued to the correct user
	otpURL, _, err := s.server.Auth().GetOTPData(user)
	c.Assert(err, check.IsNil)
	u, err := url.Parse(otpURL)
	c.Assert(err, check.IsNil)
	c.Assert(u.Path, check.Equals, "/user1")

	// a completely invalid token should return access denied
	err = clt.CheckPassword("user1", pass, "123456")
	c.Assert(err, check.NotNil)

	// an invalid token should return access denied
	//
	// this tests makes the token 61 seconds in the future (but from a valid key)
	// even though the validity period is 30 seconds. this is because a token is
	// valid for 30 seconds + 30 second skew before and after for a usability
	// reasons. so a token made between seconds 31 and 60 is still valid, and
	// invalidity starts at 61 seconds in the future.
	invalidToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now().Add(61*time.Second))
	c.Assert(err, check.IsNil)
	err = clt.CheckPassword("user1", pass, invalidToken)
	c.Assert(err, check.NotNil)

	// a valid token (created right now and from a valid key) should return success
	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.IsNil)

	// try the same valid token now it should fail because we don't allow re-use of tokens
	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.NotNil)
}

// TestWebSessions tests web sessions flow for web user,
// that logs in, extends web session and tries to perform administratvie action
// but fails
func (s *TLSSuite) TestWebSessions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	req := AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	}
	// authentication attempt fails with no password set up
	_, err = proxy.AuthenticateWebUser(req)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.UpsertPassword(user, pass)

	// success with password set up
	ws, err := proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)
	c.Assert(ws, check.Not(check.Equals), "")

	web, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)

	new, err := web.ExtendWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(new, check.NotNil)

	// Requesting forbidden action for user fails
	err = web.DeleteUser(user)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.NotNil)

	_, err = web.ExtendWebSession(user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestGenerateCerts tests edge cases around authorization of
// certificate generation for servers and users
func (s *TLSSuite) TestGenerateCerts(c *check.C) {
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, check.IsNil)
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	// generate server keys for node
	hostID := "00000000-0000-0000-0000-000000000000"
	hostClient, err := s.server.NewClient(TestIdentity{I: BuiltinRole{Username: hostID, Role: teleport.RoleNode}})
	c.Assert(err, check.IsNil)

	certs, err := hostClient.GenerateServerKeys(
		GenerateServerKeysRequest{
			HostID:               hostID,
			NodeName:             s.server.AuthServer.ClusterName,
			Roles:                teleport.Roles{teleport.RoleNode},
			AdditionalPrincipals: []string{"example.com"},
		})
	c.Assert(err, check.IsNil)

	key, _, _, _, err := ssh.ParseAuthorizedKey(certs.Cert)
	c.Assert(err, check.IsNil)
	hostCert := key.(*ssh.Certificate)
	comment := check.Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(utils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), check.Equals, true, comment)

	// attempt to elevate privileges by getting admin role in the certificate
	_, err = hostClient.GenerateServerKeys(
		GenerateServerKeysRequest{
			HostID:   hostID,
			NodeName: s.server.AuthServer.ClusterName,
			Roles:    teleport.Roles{teleport.RoleAdmin},
		})
	fixtures.ExpectAccessDenied(c, err)

	// attempt to get certificate for different host id
	_, err = hostClient.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:   "some-other-host-id",
		NodeName: s.server.AuthServer.ClusterName,
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	fixtures.ExpectAccessDenied(c, err)

	user1, userRole, err := CreateUserAndRole(clt, "user1", []string{"user1"})
	c.Assert(err, check.IsNil)

	user2, _, err := CreateUserAndRole(clt, "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	// unauthenticated client should NOT be able to generate a user cert without auth
	nopClient, err := s.server.NewClient(TestNop())
	c.Assert(err, check.IsNil)

	_, err = nopClient.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, ".*cannot request a certificate for user1")

	// Users don't match
	userClient2, err := s.server.NewClient(TestUser(user2.GetName()))
	c.Assert(err, check.IsNil)

	_, err = userClient2.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, ".*cannot request a certificate for user1")

	// should not be able to generate cert for longer than duration
	userClient1, err := s.server.NewClient(TestUser(user1.GetName()))
	c.Assert(err, check.IsNil)

	cert, err := userClient1.GenerateUserCert(pub, user1.GetName(), 40*time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)

	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
	parsedCert, _ := parsedKey.(*ssh.Certificate)
	validBefore := time.Unix(int64(parsedCert.ValidBefore), 0)
	diff := validBefore.Sub(time.Now())
	c.Assert(diff < defaults.MaxCertDuration, check.Equals, true, check.Commentf("expected %v < %v", diff, defaults.CertDuration))

	// user should have agent forwarding (default setting)
	_, exists := parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// now update role to permit agent forwarding
	roleOptions := userRole.GetOptions()
	roleOptions.Set(services.ForwardAgent, true)
	userRole.SetOptions(roleOptions)
	err = clt.UpsertRole(userRole, backend.Forever)
	c.Assert(err, check.IsNil)

	cert, err = userClient1.GenerateUserCert(pub, user1.GetName(), 1*time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)
	parsedKey, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
	parsedCert, _ = parsedKey.(*ssh.Certificate)

	// user should get agent forwarding
	_, exists = parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// apply HTTP Auth to generate user cert:
	cert, err = userClient1.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
}

// TestCertificateFormat makes sure that certificates are generated with the
// correct format.
func (s *TLSSuite) TestCertificateFormat(c *check.C) {
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, check.IsNil)
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// create an admin client
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	// use admin client to create user and role
	user, userRole, err := CreateUserAndRole(clt, "user", []string{"user"})
	c.Assert(err, check.IsNil)

	var tests = []struct {
		inRoleCertificateFormat   string
		inClientCertificateFormat string
		outCertContainsRole       bool
	}{
		// 0 - take whatever the role has
		{
			teleport.CertificateFormatOldSSH,
			teleport.CertificateFormatUnspecified,
			false,
		},
		// 1 - override the role
		{
			teleport.CertificateFormatOldSSH,
			teleport.CertificateFormatStandard,
			true,
		},
	}

	for _, tt := range tests {
		roleOptions := userRole.GetOptions()
		roleOptions.Set(services.CertificateFormat, tt.inRoleCertificateFormat)
		userRole.SetOptions(roleOptions)
		err := clt.UpsertRole(userRole, backend.Forever)
		c.Assert(err, check.IsNil)

		// get a user client
		userClient, err := s.server.NewClient(TestUser(user.GetName()))
		c.Assert(err, check.IsNil)

		cert, err := userClient.GenerateUserCert(pub, user.GetName(), defaults.CertDuration, tt.inClientCertificateFormat)
		c.Assert(err, check.IsNil)
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
		c.Assert(err, check.IsNil)
		parsedCert, _ := parsedKey.(*ssh.Certificate)

		_, ok := parsedCert.Extensions[teleport.CertExtensionTeleportRoles]
		c.Assert(ok, check.Equals, tt.outCertContainsRole)
	}
}

// TestClusterConfigContext checks that the cluster configuration gets passed
// along in the context and permissions get updated accordingly.
func (s *TLSSuite) TestClusterConfigContext(c *check.C) {
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// try and generate a host cert, this should fail because we are recording
	// at the nodes not at the proxy
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", teleport.Roles{teleport.RoleProxy}, 0)
	fixtures.ExpectAccessDenied(c, err)

	// update cluster config to record at the proxy
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtProxy,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	// try and generate a host cert, now the proxy should be able to generate a
	// host cert because it's in recording mode.
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", teleport.Roles{teleport.RoleProxy}, 0)
	c.Assert(err, check.IsNil)
}

// TestAuthenticateWebUserOTP tests web authentication flow for password + OTP
func (s *TLSSuite) TestAuthenticateWebUserOTP(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertTOTP(user, otpSecret)
	c.Assert(err, check.IsNil)

	otpURL, _, err := s.server.Auth().GetOTPData(user)
	c.Assert(err, check.IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, check.IsNil)
	c.Assert(u.Path, check.Equals, "/ws-test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, check.IsNil)

	// authentication attempt fails with wrong passwrod
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: []byte("wrong123"), Token: validToken},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication attempt fails with wrong otp
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: pass, Token: "wrong123"},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication attempt fails with password auth only
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication succeeds
	ws, err := proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: pass, Token: validToken},
	})
	c.Assert(err, check.IsNil)

	userClient, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestTokenSignupFlow tests signup flow using invite token
func (s *TLSSuite) TestTokenSignupFlow(c *check.C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, check.IsNil)

	user := "foobar"
	mappings := []string{"admin", "db"}

	token, err := s.server.Auth().CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings}, 0)
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// invalid token
	_, _, err = proxy.GetSignupTokenData("bad_token_data")
	c.Assert(err, check.NotNil)

	// valid token - success
	_, _, err = proxy.GetSignupTokenData(token)
	c.Assert(err, check.IsNil)

	signupToken, err := s.server.Auth().GetSignupToken(token)
	c.Assert(err, check.IsNil)

	otpToken, err := totp.GenerateCode(signupToken.OTPKey, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	// valid token, but missing second factor
	newPassword := "abc123"
	_, err = proxy.CreateUserWithoutOTP(token, newPassword)
	fixtures.ExpectAccessDenied(c, err)

	// invalid signup token
	_, err = proxy.CreateUserWithOTP("what_token?", newPassword, otpToken)
	fixtures.ExpectAccessDenied(c, err)

	// valid signup token, invalid otp token
	_, err = proxy.CreateUserWithOTP(token, newPassword, "badotp")
	fixtures.ExpectAccessDenied(c, err)

	// success
	ws, err := proxy.CreateUserWithOTP(token, newPassword, otpToken)
	c.Assert(err, check.IsNil)

	// attempt to reuse token fails
	_, err = proxy.CreateUserWithOTP(token, newPassword, otpToken)
	fixtures.ExpectAccessDenied(c, err)

	// can login with web session credentials now
	userClient, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)
}

// TestLoginAttempts makes sure the login attempt counter is incremented and
// reset correctly.
func (s *TLSSuite) TestLoginAttempts(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	req := AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: []byte("bad pass"),
		},
	}
	// authentication attempt fails with bad password
	_, err = proxy.AuthenticateWebUser(req)
	fixtures.ExpectAccessDenied(c, err)

	// creates first failed login attempt
	loginAttempts, err := s.server.Auth().GetUserLoginAttempts(user)
	c.Assert(err, check.IsNil)
	c.Assert(loginAttempts, check.HasLen, 1)

	// try second time with wrong pass
	req.Pass.Password = pass
	_, err = proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)

	// clears all failed attempts after success
	loginAttempts, err = s.server.Auth().GetUserLoginAttempts(user)
	c.Assert(err, check.IsNil)
	c.Assert(loginAttempts, check.HasLen, 0)
}

// TestTLSFailover tests client failover between two tls servers
func (s *TLSSuite) TestTLSFailover(c *check.C) {
	otherServer, err := s.server.AuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer otherServer.Close()

	tlsConfig, err := s.server.ClientTLSConfig(TestNop())
	c.Assert(err, check.IsNil)

	addrs := []utils.NetAddr{
		utils.FromAddr(otherServer.Listener.Addr()),
		utils.FromAddr(s.server.Listener.Addr()),
	}
	client, err := NewTLSClient(addrs, tlsConfig)
	c.Assert(err, check.IsNil)

	// couple of runs to get enough connections
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}

	// stop the server to get response
	otherServer.Stop()

	// client detects closed sockets and reconnecte to the backup server
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}
}
