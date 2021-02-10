/*
Copyright 2021 Gravitational, Inc.

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

package local

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/auth/test/suite"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/pborman/uuid"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

// ServicesTestSuite is an acceptance test suite
// for services. It is used for local implementations and implementations
// using GRPC to guarantee consistency between local and remote services
type ServicesTestSuite struct {
	WebS          *IdentityService
	ConfigS       *ClusterConfigurationService
	ProvisioningS *ProvisioningService
	Trust         *CA
	Access        *AccessService
	PresenceS     *PresenceService

	bk    backend.Backend
	suite *suite.ServicesTestSuite
}

var _ = check.Suite(&ServicesTestSuite{})

func (s *ServicesTestSuite) SetUpTest(c *check.C) {
	var err error

	clock := clockwork.NewFakeClock()

	s.bk, err = lite.NewWithConfig(context.TODO(), lite.Config{
		Path:             c.MkDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	c.Assert(err, check.IsNil)

	s.Access = NewAccessService(s.bk)
	s.PresenceS = NewPresenceService(s.bk)
	s.ConfigS = NewClusterConfigurationService(s.bk)
	s.Trust = NewCAService(s.bk)
	s.WebS = NewIdentityService(s.bk)
	s.ProvisioningS = NewProvisioningService(s.bk)
	events := NewEventsService(s.bk)

	s.suite = &suite.ServicesTestSuite{
		PresenceS:     s.PresenceS,
		ProvisioningS: s.ProvisioningS,
		Access:        s.Access,
		EventsS:       events,
		ChangesC:      make(chan interface{}),
		ConfigS:       s.ConfigS,
		UsersS:        s.WebS,
		Clock:         clock,
	}
}

func (s *ServicesTestSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *ServicesTestSuite) TestUsersCRUD(c *check.C) {
	u, err := s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	c.Assert(len(u), check.Equals, 0)

	c.Assert(s.WebS.UpsertPasswordHash("user1", []byte("hash")), check.IsNil)
	c.Assert(s.WebS.UpsertPasswordHash("user2", []byte("hash2")), check.IsNil)

	u, err = s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	userSlicesEqual(c, u, []services.User{newUser("user1"), newUser("user2")})

	out, err := s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, u[0])

	user := newUser("user1", "admin", "user")
	c.Assert(s.WebS.UpsertUser(user), check.IsNil)

	out, err = s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, user)

	out, err = s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, user)

	c.Assert(s.WebS.DeleteUser(context.TODO(), "user1"), check.IsNil)

	u, err = s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	userSlicesEqual(c, u, []services.User{newUser("user2")})

	err = s.WebS.DeleteUser(context.TODO(), "user1")
	fixtures.ExpectNotFound(c, err)

	// bad username
	err = s.WebS.UpsertUser(newUser(""))
	fixtures.ExpectBadParameter(c, err)
}

func (s *ServicesTestSuite) TestUsersExpiry(c *check.C) {
	expiresAt := s.suite.Clock.Now().Add(1 * time.Minute)

	err := s.WebS.UpsertUser(&types.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "foo",
			Namespace: defaults.Namespace,
			Expires:   &expiresAt,
		},
		Spec: services.UserSpecV2{},
	})
	c.Assert(err, check.IsNil)

	// Make sure the user exists.
	u, err := s.WebS.GetUser("foo", false)
	c.Assert(err, check.IsNil)
	c.Assert(u.GetName(), check.Equals, "foo")

	s.suite.Clock.Advance(2 * time.Minute)

	// Make sure the user is now gone.
	_, err = s.WebS.GetUser("foo", false)
	c.Assert(err, check.NotNil)
}

func (s *ServicesTestSuite) TestLoginAttempts(c *check.C) {
	user := newUser("user1", "admin", "user")
	c.Assert(s.WebS.UpsertUser(user), check.IsNil)

	attempts, err := s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(len(attempts), check.Equals, 0)

	clock := clockwork.NewFakeClock()
	attempt1 := auth.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt1, defaults.AttemptTTL)
	c.Assert(err, check.IsNil)

	attempt2 := auth.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt2, defaults.AttemptTTL)
	c.Assert(err, check.IsNil)

	attempts, err = s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(attempts, check.DeepEquals, []auth.LoginAttempt{attempt1, attempt2})
	c.Assert(auth.LastFailed(3, attempts), check.Equals, false)
	c.Assert(auth.LastFailed(2, attempts), check.Equals, true)
}

func (s *ServicesTestSuite) TestClusterConfig(c *check.C) {
	s.suite.ClusterConfig(c)

	err := s.ConfigS.DeleteClusterConfig()
	c.Assert(err, check.IsNil)

	_, err = s.ConfigS.GetClusterConfig()
	fixtures.ExpectNotFound(c, err)

	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	gotName, err := s.ConfigS.GetClusterName()
	c.Assert(err, check.IsNil)
	clusterName.SetResourceID(gotName.GetResourceID())
	fixtures.DeepCompare(c, clusterName, gotName)

	err = s.ConfigS.DeleteClusterName()
	c.Assert(err, check.IsNil)

	_, err = s.ConfigS.GetClusterName()
	fixtures.ExpectNotFound(c, err)

	err = s.ConfigS.UpsertClusterName(clusterName)
	c.Assert(err, check.IsNil)

	gotName, err = s.ConfigS.GetClusterName()
	c.Assert(err, check.IsNil)
	clusterName.SetResourceID(gotName.GetResourceID())
	fixtures.DeepCompare(c, clusterName, gotName)
}

// EventsClusterConfig tests cluster config resource events
func (s *ServicesTestSuite) EventsClusterConfig(c *check.C) {
	testCases := []suite.EventTest{
		{
			Name: "Cluster config",
			Kind: types.WatchKind{
				Kind: types.KindClusterConfig,
			},
			CRUD: func() types.Resource {
				config, err := types.NewClusterConfig(types.ClusterConfigSpecV3{})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetClusterConfig(config)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetClusterConfig()
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteClusterConfig()
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			Name: "Cluster name",
			Kind: types.WatchKind{
				Kind: types.KindClusterName,
			},
			CRUD: func() types.Resource {
				clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
					ClusterName: "example.com",
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetClusterName(clusterName)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetClusterName()
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteClusterName()
				c.Assert(err, check.IsNil)
				return out
			},
		},
	}
	s.suite.RunEventsTests(c, testCases)
}

func (s *ServicesTestSuite) TestCertAuthCRUD(c *check.C) {
	ca := test.NewCA(types.UserCA, "example.com")
	c.Assert(s.Trust.UpsertCertAuthority(ca), check.IsNil)

	out, err := s.Trust.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, check.IsNil)
	ca.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, out, ca)

	cas, err := s.Trust.GetCertAuthorities(types.UserCA, false)
	c.Assert(err, check.IsNil)
	ca2 := *ca
	ca2.Spec.SigningKeys = nil
	ca2.Spec.TLSKeyPairs = []types.TLSKeyPair{{Cert: ca2.Spec.TLSKeyPairs[0].Cert}}
	ca2.Spec.JWTKeyPairs = []types.JWTKeyPair{{PublicKey: ca2.Spec.JWTKeyPairs[0].PublicKey}}
	fixtures.DeepCompare(c, cas[0], &ca2)

	cas, err = s.Trust.GetCertAuthorities(types.UserCA, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, cas[0], ca)

	cas, err = s.Trust.GetCertAuthorities(types.UserCA, true, resource.SkipValidation())
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, cas[0], ca)

	err = s.Trust.DeleteCertAuthority(*ca.ID())
	c.Assert(err, check.IsNil)

	// test compare and swap
	ca = test.NewCA(types.UserCA, "example.com")
	c.Assert(s.Trust.CreateCertAuthority(ca), check.IsNil)

	clock := clockwork.NewFakeClock()
	newCA := *ca
	rotation := types.Rotation{
		State:       types.RotationStateInProgress,
		CurrentID:   "id1",
		GracePeriod: types.NewDuration(time.Hour),
		Started:     clock.Now(),
	}
	newCA.SetRotation(rotation)

	err = s.Trust.CompareAndSwapCertAuthority(&newCA, ca)
	c.Assert(err, check.IsNil)

	out, err = s.Trust.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, check.IsNil)
	newCA.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, &newCA, out)
}

func (s *ServicesTestSuite) TestPasswordHashCRUD(c *check.C) {
	_, err := s.WebS.GetPasswordHash("user1")
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("%#v", err))

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello123"))
	c.Assert(err, check.IsNil)

	hash, err := s.WebS.GetPasswordHash("user1")
	c.Assert(err, check.IsNil)
	c.Assert(hash, check.DeepEquals, []byte("hello123"))

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello321"))
	c.Assert(err, check.IsNil)

	hash, err = s.WebS.GetPasswordHash("user1")
	c.Assert(err, check.IsNil)
	c.Assert(hash, check.DeepEquals, []byte("hello321"))
}

func (s *ServicesTestSuite) TestWebSessionCRUD(c *check.C) {
	req := types.GetWebSessionRequest{User: "user1", SessionID: "sid1"}
	_, err := s.WebS.WebSessions().Get(context.TODO(), req)
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("%#v", err))

	dt := s.suite.Clock.Now().Add(1 * time.Minute)
	ws := types.NewWebSession("sid1", services.KindWebSession, services.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub123"),
			Priv:    []byte("priv123"),
			Expires: dt,
		})
	err = s.WebS.WebSessions().Upsert(context.TODO(), ws)
	c.Assert(err, check.IsNil)

	out, err := s.WebS.WebSessions().Get(context.TODO(), req)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, ws)

	ws1 := types.NewWebSession("sid1", services.KindWebSession, services.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub321"),
			Priv:    []byte("priv321"),
			Expires: dt,
		})
	err = s.WebS.WebSessions().Upsert(context.TODO(), ws1)
	c.Assert(err, check.IsNil)

	out2, err := s.WebS.WebSessions().Get(context.TODO(), req)
	c.Assert(err, check.IsNil)
	c.Assert(out2, check.DeepEquals, ws1)

	c.Assert(s.WebS.WebSessions().Delete(context.TODO(), types.DeleteWebSessionRequest{
		User:      req.User,
		SessionID: req.SessionID,
	}), check.IsNil)

	_, err = s.WebS.WebSessions().Get(context.TODO(), req)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) TestTokenCRUD(c *check.C) {
	ctx := context.Background()
	_, err := s.ProvisioningS.GetToken(ctx, "token")
	fixtures.ExpectNotFound(c, err)

	t, err := services.NewProvisionToken("token", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, time.Time{})
	c.Assert(err, check.IsNil)

	c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

	token, err := s.ProvisioningS.GetToken(ctx, "token")
	c.Assert(err, check.IsNil)
	c.Assert(token.GetRoles().Include(teleport.RoleAuth), check.Equals, true)
	c.Assert(token.GetRoles().Include(teleport.RoleNode), check.Equals, true)
	c.Assert(token.GetRoles().Include(teleport.RoleProxy), check.Equals, false)
	diff := s.suite.Clock.Now().UTC().Add(defaults.ProvisioningTokenTTL).Second() - token.Expiry().Second()
	if diff > 1 {
		c.Fatalf("expected diff to be within one second, got %v instead", diff)
	}

	c.Assert(s.ProvisioningS.DeleteToken(ctx, "token"), check.IsNil)

	_, err = s.ProvisioningS.GetToken(ctx, "token")
	fixtures.ExpectNotFound(c, err)

	// check tokens backwards compatibility and marshal/unmarshal
	expiry := time.Now().UTC().Add(time.Hour)
	v1 := &services.ProvisionTokenV1{
		Token:   "old",
		Roles:   teleport.Roles{teleport.RoleNode, teleport.RoleProxy},
		Expires: expiry,
	}
	v2, err := services.NewProvisionToken(v1.Token, v1.Roles, expiry)
	c.Assert(err, check.IsNil)

	// Tokens in different version formats are backwards and forwards
	// compatible
	fixtures.DeepCompare(c, v1.V2(), v2)
	fixtures.DeepCompare(c, v2.V1(), v1)

	// Marshal V1, unmarshal V2
	data, err := resource.MarshalProvisionToken(v2, resource.WithVersion(services.V1))
	c.Assert(err, check.IsNil)

	out, err := resource.UnmarshalProvisionToken(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, out, v2)

	// Test delete all tokens
	t, err = services.NewProvisionToken("token1", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, time.Time{})
	c.Assert(err, check.IsNil)
	c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

	t, err = services.NewProvisionToken("token2", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, time.Time{})
	c.Assert(err, check.IsNil)
	c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

	tokens, err := s.ProvisioningS.GetTokens(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(tokens, check.HasLen, 2)

	err = s.ProvisioningS.DeleteAllTokens()
	c.Assert(err, check.IsNil)

	tokens, err = s.ProvisioningS.GetTokens(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(tokens, check.HasLen, 0)
}

func (s *ServicesTestSuite) TestRolesCRUD(c *check.C) {
	ctx := context.Background()

	out, err := s.Access.GetRoles(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	role := services.RoleV3{
		Kind:    services.KindRole,
		Version: services.V3,
		Metadata: services.Metadata{
			Name:      "role1",
			Namespace: defaults.Namespace,
		},
		Spec: services.RoleSpecV3{
			Options: services.RoleOptions{
				MaxSessionTTL:     services.Duration(time.Hour),
				PortForwarding:    services.NewBoolOption(true),
				CertificateFormat: teleport.CertificateFormatStandard,
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: services.RoleConditions{
				Logins:           []string{"root", "bob"},
				NodeLabels:       services.Labels{services.Wildcard: []string{services.Wildcard}},
				AppLabels:        services.Labels{services.Wildcard: []string{services.Wildcard}},
				KubernetesLabels: services.Labels{services.Wildcard: []string{services.Wildcard}},
				DatabaseLabels:   services.Labels{services.Wildcard: []string{services.Wildcard}},
				Namespaces:       []string{defaults.Namespace},
				Rules: []services.Rule{
					services.NewRule(services.KindRole, auth.RO()),
				},
			},
			Deny: services.RoleConditions{
				Namespaces: []string{defaults.Namespace},
			},
		},
	}

	err = s.Access.UpsertRole(ctx, &role)
	c.Assert(err, check.IsNil)
	rout, err := s.Access.GetRole(ctx, role.Metadata.Name)
	c.Assert(err, check.IsNil)
	role.SetResourceID(rout.GetResourceID())
	fixtures.DeepCompare(c, rout, &role)

	role.Spec.Allow.Logins = []string{"bob"}
	err = s.Access.UpsertRole(ctx, &role)
	c.Assert(err, check.IsNil)
	rout, err = s.Access.GetRole(ctx, role.Metadata.Name)
	c.Assert(err, check.IsNil)
	role.SetResourceID(rout.GetResourceID())
	c.Assert(rout, check.DeepEquals, &role)

	err = s.Access.DeleteRole(ctx, role.Metadata.Name)
	c.Assert(err, check.IsNil)

	_, err = s.Access.GetRole(ctx, role.Metadata.Name)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) TestU2FCRUD(c *check.C) {
	token := "tok1"
	appID := "https://localhost"
	user1 := "user1"

	challenge, err := u2f.NewChallenge(appID, []string{appID})
	c.Assert(err, check.IsNil)

	err = s.WebS.UpsertU2FRegisterChallenge(token, challenge)
	c.Assert(err, check.IsNil)

	challengeOut, err := s.WebS.GetU2FRegisterChallenge(token)
	c.Assert(err, check.IsNil)
	c.Assert(challenge.Challenge, check.DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), check.Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, check.Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, check.DeepEquals, challengeOut.TrustedFacets)

	err = s.WebS.UpsertU2FSignChallenge(user1, challenge)
	c.Assert(err, check.IsNil)

	challengeOut, err = s.WebS.GetU2FSignChallenge(user1)
	c.Assert(err, check.IsNil)
	c.Assert(challenge.Challenge, check.DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), check.Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, check.Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, check.DeepEquals, challengeOut.TrustedFacets)

	derKey, err := base64.StdEncoding.DecodeString("MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEGOi54Eun0r3Xrj8PjyOGYzJObENYI/t/Lr9g9PsHTHnp1qI2ysIhsdMPd7x/vpsL6cr+2EPVik7921OSsVjEMw==")
	c.Assert(err, check.IsNil)
	pubkeyInterface, err := x509.ParsePKIXPublicKey(derKey)
	c.Assert(err, check.IsNil)

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	c.Assert(ok, check.Equals, true)

	registration := u2f.Registration{
		Raw:       []byte("BQQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzQIJ+PVFsYjEa0Xgnx+siQaxdlku+U+J2W55U5NrN1iGIc0Amh+0HwhbV2W90G79cxIYS2SVIFAdqTTDXvPXJbeAwggE8MIHkoAMCAQICChWIR0AwlYJZQHcwCgYIKoZIzj0EAwIwFzEVMBMGA1UEAxMMRlQgRklETyAwMTAwMB4XDTE0MDgxNDE4MjkzMloXDTI0MDgxNDE4MjkzMlowMTEvMC0GA1UEAxMmUGlsb3RHbnViYnktMC40LjEtMTU4ODQ3NDAzMDk1ODI1OTQwNzcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzMAoGCCqGSM49BAMCA0cAMEQCIIbmYKu6I2L4pgZCBms9NIo9yo5EO9f2irp0ahvLlZudAiC8RN/N+WHAFdq8Z+CBBOMsRBFDDJy3l5EDR83B5GAfrjBEAiBl6R6gAmlbudVpW2jSn3gfjmA8EcWq0JsGZX9oFM/RJwIgb9b01avBY5jBeVIqw5KzClLzbRDMY4K+Ds6uprHyA1Y="),
		KeyHandle: []byte("gn49UWxiMRrReCfH6yJBrF2WS75T4nZbnlTk2s3WIYhzQCaH7QfCFtXZb3Qbv1zEhhLZJUgUB2pNMNe89clt4A=="),
		PubKey:    *pubkey,
	}
	dev, err := u2f.NewDevice("u2f", &registration, s.suite.Clock.Now())
	c.Assert(err, check.IsNil)
	ctx := context.Background()
	err = s.WebS.UpsertMFADevice(ctx, user1, dev)
	c.Assert(err, check.IsNil)

	devs, err := s.WebS.GetMFADevices(ctx, user1)
	c.Assert(err, check.IsNil)
	c.Assert(devs, check.HasLen, 1)
	// Raw registration output is not stored - it's not used for
	// authentication.
	registration.Raw = nil
	registrationOut, err := u2f.DeviceToRegistration(devs[0].GetU2F())
	c.Assert(err, check.IsNil)
	c.Assert(&registration, check.DeepEquals, registrationOut)

	// Attempt to upsert the same device name with a different ID.
	dev.Id = uuid.New()
	err = s.WebS.UpsertMFADevice(ctx, user1, dev)
	c.Assert(trace.IsAlreadyExists(err), check.Equals, true)

	// Attempt to upsert a new device with different name and ID.
	dev.Metadata.Name = "u2f-2"
	err = s.WebS.UpsertMFADevice(ctx, user1, dev)
	c.Assert(err, check.IsNil)
	devs, err = s.WebS.GetMFADevices(ctx, user1)
	c.Assert(err, check.IsNil)
	c.Assert(devs, check.HasLen, 2)
}

func (s *ServicesTestSuite) TestSAMLCRUD(c *check.C) {
	ctx := context.Background()
	connector := &services.SAMLConnectorV2{
		Kind:    services.KindSAML,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "saml1",
			Namespace: defaults.Namespace,
		},
		Spec: services.SAMLConnectorSpecV2{
			Issuer:                   "http://example.com",
			SSO:                      "https://example.com/saml/sso",
			AssertionConsumerService: "https://localhost/acs",
			Audience:                 "https://localhost/aud",
			ServiceProviderIssuer:    "https://localhost/iss",
			AttributesToRoles: []services.AttributeMapping{
				{Name: "groups", Value: "admin", Roles: []string{"admin"}},
			},
			Cert: fixtures.SigningCertPEM,
			SigningKeyPair: &services.AsymmetricKeyPair{
				PrivateKey: fixtures.SigningKeyPEM,
				Cert:       fixtures.SigningCertPEM,
			},
		},
	}
	err := auth.ValidateSAMLConnector(connector)
	c.Assert(err, check.IsNil)
	err = s.WebS.UpsertSAMLConnector(ctx, connector)
	c.Assert(err, check.IsNil)
	out, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, out, connector)

	connectors, err := s.WebS.GetSAMLConnectors(ctx, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []services.SAMLConnector{connector}, connectors)

	out2, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), false)
	c.Assert(err, check.IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.SigningKeyPair.PrivateKey = ""
	fixtures.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.WebS.GetSAMLConnectors(ctx, false)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []services.SAMLConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	c.Assert(err, check.IsNil)

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))

	_, err = s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) TestStaticTokens(c *check.C) {
	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{
			{
				Token:   "tok1",
				Roles:   teleport.Roles{teleport.RoleNode},
				Expires: s.suite.Clock.Now().UTC().Add(time.Hour),
			},
		},
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetStaticTokens(staticTokens)
	c.Assert(err, check.IsNil)

	out, err := s.ConfigS.GetStaticTokens()
	c.Assert(err, check.IsNil)
	staticTokens.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, staticTokens, out)

	err = s.ConfigS.DeleteStaticTokens()
	c.Assert(err, check.IsNil)

	_, err = s.ConfigS.GetStaticTokens()
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) TestSemaphoreFlakiness(c *check.C) {
	const renewals = 3
	// wrap our services.Semaphores instance to cause two out of three lease
	// keepalive attempts fail with a meaningless error.  Locks should make
	// at *least* three attempts before their expiry, so locks should not
	// fail under these conditions.
	keepAlives := new(uint64)
	wrapper := &semWrapper{
		Semaphores: s.PresenceS,
		keepAlive: func(ctx context.Context, lease services.SemaphoreLease) error {
			kn := atomic.AddUint64(keepAlives, 1)
			if kn%3 == 0 {
				return s.PresenceS.KeepAliveSemaphoreLease(ctx, lease)
			}
			return trace.Errorf("uh-oh!")
		},
	}

	cfg := auth.SemaphoreLockConfig{
		Service:  wrapper,
		Expiry:   time.Second,
		TickRate: time.Millisecond * 50,
		Params: services.AcquireSemaphoreRequest{
			SemaphoreKind: services.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     1,
		},
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	lock, err := auth.AcquireSemaphoreLock(ctx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(ctx)

	for i := 0; i < renewals; i++ {
		select {
		case <-lock.Renewed():
			continue
		case <-lock.Done():
			c.Fatalf("Lost semaphore lock: %v", lock.Wait())
		case <-time.After(time.Second):
			c.Fatalf("Timeout waiting for renewals")
		}
	}
}

// SemaphoreContention checks that a large number of concurrent acquisitions
// all succeed if MaxLeases is sufficiently high.  Note that we do not test
// the same property holds for releasing semaphores; release operations are
// best-effort and allowed to fail.  Also, "large" in this context is still
// fairly small.  Semaphores aren't cheap and the auth server is expected
// to start returning "too much contention" errors at around 100 concurrent
// attempts.
func (s *ServicesTestSuite) TestSemaphoreContention(c *check.C) {
	const locks int64 = 50
	const iters = 5
	for i := 0; i < iters; i++ {
		cfg := auth.SemaphoreLockConfig{
			Service: s.PresenceS,
			Expiry:  time.Hour,
			Params: services.AcquireSemaphoreRequest{
				SemaphoreKind: services.SemaphoreKindConnection,
				SemaphoreName: "alice",
				MaxLeases:     locks,
			},
		}
		// we leak lock handles in the spawned goroutines, so
		// context-based cancellation is needed to cleanup the
		// background keepalive activity.
		ctx, cancel := context.WithCancel(context.TODO())
		var wg sync.WaitGroup
		for i := int64(0); i < locks; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lock, err := auth.AcquireSemaphoreLock(ctx, cfg)
				c.Assert(err, check.IsNil)
				go lock.KeepAlive(ctx)
			}()
		}
		wg.Wait()
		cancel()
		c.Assert(s.PresenceS.DeleteSemaphore(context.TODO(), services.SemaphoreFilter{
			SemaphoreKind: cfg.Params.SemaphoreKind,
			SemaphoreName: cfg.Params.SemaphoreName,
		}), check.IsNil)
	}
}

// SemaphoreConcurrency verifies that a large number of concurrent
// acquisitions result in the correct number of successful acquisitions.
func (s *ServicesTestSuite) TestSemaphoreConcurrency(c *check.C) {
	const maxLeases int64 = 20
	const attempts int64 = 200
	cfg := auth.SemaphoreLockConfig{
		Service: s.PresenceS,
		Expiry:  time.Hour,
		Params: services.AcquireSemaphoreRequest{
			SemaphoreKind: services.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     maxLeases,
		},
	}
	// we leak lock handles in the spawned goroutines, so
	// context-based cancellation is needed to cleanup the
	// background keepalive activity.
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	var success int64
	var failure int64
	var wg sync.WaitGroup
	for i := int64(0); i < attempts; i++ {
		wg.Add(1)
		go func() {
			lock, err := auth.AcquireSemaphoreLock(ctx, cfg)
			if err == nil {
				go lock.KeepAlive(ctx)
				atomic.AddInt64(&success, 1)
			} else {
				atomic.AddInt64(&failure, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	c.Assert(atomic.LoadInt64(&success), check.Equals, maxLeases)
	c.Assert(atomic.LoadInt64(&failure), check.Equals, attempts-maxLeases)
}

// SemaphoreLock verifies correct functionality of the basic
// semaphore lock scenarios.
func (s *ServicesTestSuite) TestSemaphoreLock(c *check.C) {
	cfg := auth.SemaphoreLockConfig{
		Service: s.PresenceS,
		Expiry:  time.Hour,
		Params: services.AcquireSemaphoreRequest{
			SemaphoreKind: services.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     1,
		},
	}
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	lock, err := auth.AcquireSemaphoreLock(ctx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(ctx)

	// MaxLeases is 1, so second acquire op fails.
	_, err = auth.AcquireSemaphoreLock(ctx, cfg)
	fixtures.ExpectLimitExceeded(c, err)

	// Lock is successfully released.
	lock.Stop()
	c.Assert(lock.Wait(), check.IsNil)

	// Acquire new lock with short expiry
	// and high tick rate to verify renewals.
	cfg.Expiry = time.Second
	cfg.TickRate = time.Millisecond * 50
	lock, err = auth.AcquireSemaphoreLock(ctx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(ctx)

	timeout := time.After(time.Second)

	for i := 0; i < 3; i++ {
		select {
		case <-lock.Done():
			c.Fatalf("Unexpected lock failure: %v", lock.Wait())
		case <-timeout:
			c.Fatalf("Timeout waiting for lock renewal %d", i)
		case <-lock.Renewed():
		}
	}

	// forcibly delete the semaphore
	c.Assert(s.PresenceS.DeleteSemaphore(context.TODO(), services.SemaphoreFilter{
		SemaphoreKind: cfg.Params.SemaphoreKind,
		SemaphoreName: cfg.Params.SemaphoreName,
	}), check.IsNil)

	select {
	case <-lock.Done():
		fixtures.ExpectNotFound(c, lock.Wait())
	case <-time.After(time.Millisecond * 1500):
		c.Errorf("timeout waiting for semaphore lock failure")
	}
}

// ProxyWatcher tests proxy watcher
func (s *ServicesTestSuite) TestProxyWatcher(c *check.C) {
	type client struct {
		*PresenceService
		*EventsService
	}
	eventsService := NewEventsService(s.bk)
	w, err := auth.NewProxyWatcher(auth.ProxyWatcherConfig{
		Context:     context.TODO(),
		Component:   "test",
		RetryPeriod: 200 * time.Millisecond,
		Client: &client{
			PresenceService: s.PresenceS,
			EventsService:   eventsService,
		},
		ProxiesC: make(chan []types.Server, 10),
	})

	c.Assert(err, check.IsNil)
	defer w.Close()

	// since no proxy is yet present, the ProxyWatcher should immediately
	// yield back to its retry loop.
	select {
	case <-w.Reset():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for ProxyWatcher reset")
	}

	proxy := suite.NewServer(services.KindProxy, "proxy1", "127.0.0.1:2023", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertProxy(proxy), check.IsNil)

	// the first event is always the current list of proxies
	select {
	case changeset := <-w.ProxiesC:
		c.Assert(changeset, check.HasLen, 1)
		out, err := s.PresenceS.GetProxies()
		c.Assert(err, check.IsNil)
		fixtures.DeepCompare(c, changeset[0], out[0])
	case <-w.Done():
		c.Fatalf("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for the first event")
	}

	// add a second proxy
	proxy2 := suite.NewServer(services.KindProxy, "proxy2", "127.0.0.1:2023", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertProxy(proxy2), check.IsNil)

	// watcher should detect the proxy list change
	select {
	case changeset := <-w.ProxiesC:
		c.Assert(changeset, check.HasLen, 2)
	case <-w.Done():
		c.Fatalf("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for the first event")
	}

	c.Assert(s.PresenceS.DeleteProxy(proxy.GetName()), check.IsNil)

	// watcher should detect the proxy list change
	select {
	case changeset := <-w.ProxiesC:
		c.Assert(changeset, check.HasLen, 1)
		out, err := s.PresenceS.GetProxies()
		c.Assert(err, check.IsNil)
		fixtures.DeepCompare(c, changeset[0], out[0])
	case <-w.Done():
		c.Fatalf("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for the first event")
	}
}

func (s *ServicesTestSuite) TestServerCRUD(c *check.C) {
	s.suite.ServerCRUD(c)
}

func (s *ServicesTestSuite) TestAppServerCRUD(c *check.C) {
	s.suite.AppServerCRUD(c)
}

func (s *ServicesTestSuite) TestReverseTunnelsCRUD(c *check.C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *ServicesTestSuite) TestTunnelConnectionsCRUD(c *check.C) {
	s.suite.TunnelConnectionsCRUD(c)
}

func (s *ServicesTestSuite) TestRemoteClustersCRUD(c *check.C) {
	s.suite.RemoteClustersCRUD(c)
}

func (s *ServicesTestSuite) TestAuthPreference(c *check.C) {
	s.suite.AuthPreference(c)
}

func userSlicesEqual(c *check.C, a, b []types.User) {
	comment := check.Commentf("a: %#v b: %#v", a, b)
	c.Assert(len(a), check.Equals, len(b), comment)
	sort.Sort(auth.Users(a))
	sort.Sort(auth.Users(b))
	for i := range a {
		usersEqual(c, a[i], b[i])
	}
}

func usersEqual(c *check.C, a, b types.User) {
	comment := check.Commentf("a: %#v b: %#v", a, b)
	c.Assert(auth.UsersEquals(a, b), check.Equals, true, comment)
}

func newUser(name string, roles ...string) types.User {
	return &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: types.UserSpecV2{
			Roles: roles,
		},
	}
}

// sem wrapper is a helper for overriding the keepalive
// method on the semaphore service.
type semWrapper struct {
	types.Semaphores
	keepAlive func(context.Context, types.SemaphoreLease) error
}

func (w *semWrapper) KeepAliveSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return w.keepAlive(ctx, lease)
}
