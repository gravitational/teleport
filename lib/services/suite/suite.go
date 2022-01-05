/*
Copyright 2015-2019 Gravitational, Inc.

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

package suite

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"
)

// NewTestCA returns new test authority with a test key as a public and
// signing key
func NewTestCA(caType types.CertAuthType, clusterName string, privateKeys ...[]byte) *types.CertAuthorityV2 {
	return NewTestCAWithConfig(TestCAConfig{
		Type:        caType,
		ClusterName: clusterName,
		PrivateKeys: privateKeys,
		Clock:       clockwork.NewRealClock(),
	})
}

// TestCAConfig defines the configuration for generating
// a test certificate authority
type TestCAConfig struct {
	Type        types.CertAuthType
	ClusterName string
	PrivateKeys [][]byte
	Clock       clockwork.Clock
}

// NewTestCAWithConfig generates a new certificate authority with the specified
// configuration
func NewTestCAWithConfig(config TestCAConfig) *types.CertAuthorityV2 {
	// privateKeys is to specify another RSA private key
	if len(config.PrivateKeys) == 0 {
		config.PrivateKeys = [][]byte{fixtures.PEMBytes["rsa"]}
	}
	keyBytes := config.PrivateKeys[0]
	rsaKey, err := ssh.ParseRawPrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	signer, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		panic(err)
	}

	cert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: rsaKey.(*rsa.PrivateKey),
		Entity: pkix.Name{
			CommonName:   config.ClusterName,
			Organization: []string{config.ClusterName},
		},
		TTL:   defaults.CATTL,
		Clock: config.Clock,
	})
	if err != nil {
		panic(err)
	}

	publicKey, privateKey, err := jwt.GenerateKeyPair()
	if err != nil {
		panic(err)
	}

	ca := &types.CertAuthorityV2{
		Kind:    types.KindCertAuthority,
		SubKind: string(config.Type),
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      config.ClusterName,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.CertAuthoritySpecV2{
			Type:        config.Type,
			ClusterName: config.ClusterName,
			ActiveKeys: types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PublicKey:  ssh.MarshalAuthorizedKey(signer.PublicKey()),
					PrivateKey: keyBytes,
				}},
				TLS: []*types.TLSKeyPair{{Cert: cert, Key: keyBytes}},
				JWT: []*types.JWTKeyPair{{
					PublicKey:  publicKey,
					PrivateKey: privateKey,
				}},
			},
		},
	}
	if err := services.SyncCertAuthorityKeys(ca); err != nil {
		panic(err)
	}
	return ca
}

// ServicesTestSuite is an acceptance test suite
// for services. It is used for local implementations and implementations
// using GRPC to guarantee consistency between local and remote services
type ServicesTestSuite struct {
	Access        services.Access
	CAS           services.Trust
	PresenceS     services.Presence
	ProvisioningS services.Provisioner
	WebS          services.Identity
	ConfigS       services.ClusterConfiguration
	EventsS       types.Events
	UsersS        services.UsersService
	RestrictionsS services.Restrictions
	ChangesC      chan interface{}
	Clock         clockwork.FakeClock
}

func (s *ServicesTestSuite) Users() services.UsersService {
	if s.WebS != nil {
		return s.WebS
	}
	return s.UsersS
}

func userSlicesEqual(c *check.C, a []types.User, b []types.User) {
	comment := check.Commentf("a: %#v b: %#v", a, b)
	c.Assert(len(a), check.Equals, len(b), comment)
	sort.Sort(services.Users(a))
	sort.Sort(services.Users(b))
	for i := range a {
		usersEqual(c, a[i], b[i])
	}
}

func usersEqual(c *check.C, a types.User, b types.User) {
	comment := check.Commentf("a: %#v b: %#v", a, b)
	c.Assert(services.UsersEquals(a, b), check.Equals, true, comment)
}

func newUser(name string, roles []string) types.User {
	return &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.UserSpecV2{
			Roles: roles,
		},
	}
}

func (s *ServicesTestSuite) UsersCRUD(c *check.C) {
	ctx := context.Background()
	u, err := s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	c.Assert(len(u), check.Equals, 0)

	c.Assert(s.WebS.UpsertPasswordHash("user1", []byte("hash")), check.IsNil)
	c.Assert(s.WebS.UpsertPasswordHash("user2", []byte("hash2")), check.IsNil)

	u, err = s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	userSlicesEqual(c, u, []types.User{newUser("user1", nil), newUser("user2", nil)})

	out, err := s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, u[0])

	user := newUser("user1", []string{"admin", "user"})
	c.Assert(s.WebS.UpsertUser(user), check.IsNil)

	out, err = s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, user)

	out, err = s.WebS.GetUser("user1", false)
	c.Assert(err, check.IsNil)
	usersEqual(c, out, user)

	c.Assert(s.WebS.DeleteUser(ctx, "user1"), check.IsNil)

	u, err = s.WebS.GetUsers(false)
	c.Assert(err, check.IsNil)
	userSlicesEqual(c, u, []types.User{newUser("user2", nil)})

	err = s.WebS.DeleteUser(ctx, "user1")
	fixtures.ExpectNotFound(c, err)

	// bad username
	err = s.WebS.UpsertUser(newUser("", nil))
	fixtures.ExpectBadParameter(c, err)
}

func (s *ServicesTestSuite) UsersExpiry(c *check.C) {
	expiresAt := s.Clock.Now().Add(1 * time.Minute)

	err := s.WebS.UpsertUser(&types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "foo",
			Namespace: apidefaults.Namespace,
			Expires:   &expiresAt,
		},
		Spec: types.UserSpecV2{},
	})
	c.Assert(err, check.IsNil)

	// Make sure the user exists.
	u, err := s.WebS.GetUser("foo", false)
	c.Assert(err, check.IsNil)
	c.Assert(u.GetName(), check.Equals, "foo")

	s.Clock.Advance(2 * time.Minute)

	// Make sure the user is now gone.
	_, err = s.WebS.GetUser("foo", false)
	c.Assert(err, check.NotNil)
}

func (s *ServicesTestSuite) LoginAttempts(c *check.C) {
	user := newUser("user1", []string{"admin", "user"})
	c.Assert(s.WebS.UpsertUser(user), check.IsNil)

	attempts, err := s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(len(attempts), check.Equals, 0)

	clock := clockwork.NewFakeClock()
	attempt1 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt1, defaults.AttemptTTL)
	c.Assert(err, check.IsNil)

	attempt2 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt2, defaults.AttemptTTL)
	c.Assert(err, check.IsNil)

	attempts, err = s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(attempts, check.DeepEquals, []services.LoginAttempt{attempt1, attempt2})
	c.Assert(services.LastFailed(3, attempts), check.Equals, false)
	c.Assert(services.LastFailed(2, attempts), check.Equals, true)
}

func (s *ServicesTestSuite) CertAuthCRUD(c *check.C) {
	ca := NewTestCA(types.UserCA, "example.com")
	c.Assert(s.CAS.UpsertCertAuthority(ca), check.IsNil)

	out, err := s.CAS.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, check.IsNil)
	ca.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, out, ca)

	cas, err := s.CAS.GetCertAuthorities(types.UserCA, false)
	c.Assert(err, check.IsNil)
	ca2 := ca.Clone().(*types.CertAuthorityV2)
	ca2.Spec.ActiveKeys.SSH[0].PrivateKey = nil
	ca2.Spec.SigningKeys = nil
	ca2.Spec.ActiveKeys.TLS[0].Key = nil
	ca2.Spec.TLSKeyPairs[0].Key = nil
	ca2.Spec.ActiveKeys.JWT[0].PrivateKey = nil
	ca2.Spec.JWTKeyPairs[0].PrivateKey = nil
	fixtures.DeepCompare(c, cas[0], ca2)

	cas, err = s.CAS.GetCertAuthorities(types.UserCA, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, cas[0], ca)

	cas, err = s.CAS.GetCertAuthorities(types.UserCA, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, cas[0], ca)

	err = s.CAS.DeleteCertAuthority(*ca.ID())
	c.Assert(err, check.IsNil)

	// test compare and swap
	ca = NewTestCA(types.UserCA, "example.com")
	c.Assert(s.CAS.CreateCertAuthority(ca), check.IsNil)

	clock := clockwork.NewFakeClock()
	newCA := *ca
	rotation := types.Rotation{
		State:       types.RotationStateInProgress,
		CurrentID:   "id1",
		GracePeriod: types.NewDuration(time.Hour),
		Started:     clock.Now(),
	}
	newCA.SetRotation(rotation)

	err = s.CAS.CompareAndSwapCertAuthority(&newCA, ca)
	c.Assert(err, check.IsNil)

	out, err = s.CAS.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, check.IsNil)
	newCA.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, &newCA, out)
}

// NewServer creates a new server resource
func NewServer(kind, name, addr, namespace string) *types.ServerV2 {
	return &types.ServerV2{
		Kind:    kind,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: types.ServerSpecV2{
			Addr:       addr,
			PublicAddr: addr,
		},
	}
}

func (s *ServicesTestSuite) ServerCRUD(c *check.C) {
	ctx := context.Background()
	// SSH service.
	out, err := s.PresenceS.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	srv := NewServer(types.KindNode, "srv1", "127.0.0.1:2022", apidefaults.Namespace)
	_, err = s.PresenceS.UpsertNode(ctx, srv)
	c.Assert(err, check.IsNil)

	node, err := s.PresenceS.GetNode(ctx, srv.Metadata.Namespace, srv.GetName())
	c.Assert(err, check.IsNil)
	srv.SetResourceID(node.GetResourceID())
	fixtures.DeepCompare(c, node, srv)

	out, err = s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []types.Server{srv})

	err = s.PresenceS.DeleteNode(ctx, srv.Metadata.Namespace, srv.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)

	// Proxy service.
	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	proxy := NewServer(types.KindProxy, "proxy1", "127.0.0.1:2023", apidefaults.Namespace)
	c.Assert(s.PresenceS.UpsertProxy(proxy), check.IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	proxy.SetResourceID(out[0].GetResourceID())
	c.Assert(out, check.DeepEquals, []types.Server{proxy})

	err = s.PresenceS.DeleteProxy(proxy.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)

	// Auth service.
	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	auth := NewServer(types.KindAuthServer, "auth1", "127.0.0.1:2025", apidefaults.Namespace)
	c.Assert(s.PresenceS.UpsertAuthServer(auth), check.IsNil)

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	auth.SetResourceID(out[0].GetResourceID())
	c.Assert(out, check.DeepEquals, []types.Server{auth})

	// Kubernetes service.
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	kube1 := NewServer(types.KindKubeService, "kube1", "10.0.0.1:3026", apidefaults.Namespace)
	c.Assert(s.PresenceS.UpsertKubeService(ctx, kube1), check.IsNil)
	kube2 := NewServer(types.KindKubeService, "kube2", "10.0.0.2:3026", apidefaults.Namespace)
	c.Assert(s.PresenceS.UpsertKubeService(ctx, kube2), check.IsNil)

	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 2)
	kube1.SetResourceID(out[0].GetResourceID())
	kube2.SetResourceID(out[1].GetResourceID())
	c.Assert(out, check.DeepEquals, []types.Server{kube1, kube2})

	c.Assert(s.PresenceS.DeleteKubeService(ctx, kube1.GetName()), check.IsNil)
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	c.Assert(out, check.DeepEquals, []types.Server{kube2})

	c.Assert(s.PresenceS.DeleteAllKubeServices(ctx), check.IsNil)
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// NewAppServer creates a new application server resource.
func NewAppServer(name string, internalAddr string, publicAddr string) *types.ServerV2 {
	return &types.ServerV2{
		Kind:    types.KindAppServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      uuid.New(),
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ServerSpecV2{
			Apps: []*types.App{
				{
					Name:       name,
					URI:        internalAddr,
					PublicAddr: publicAddr,
				},
			},
		},
	}
}

// AppServerCRUD tests CRUD functionality for services.Server.
func (s *ServicesTestSuite) AppServerCRUD(c *check.C) {
	ctx := context.Background()

	// Create application.
	server := NewAppServer("foo", "http://127.0.0.1:8080", "foo.example.com")

	// Expect not to be returned any applications and trace.NotFound.
	out, err := s.PresenceS.GetAppServers(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	// Upsert application.
	_, err = s.PresenceS.UpsertAppServer(ctx, server)
	c.Assert(err, check.IsNil)

	// Check again, expect a single application to be found.
	out, err = s.PresenceS.GetAppServers(ctx, server.GetNamespace())
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	server.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []types.Server{server})

	// Remove the application.
	err = s.PresenceS.DeleteAppServer(ctx, server.Metadata.Namespace, server.GetName())
	c.Assert(err, check.IsNil)

	// Now expect no applications to be returned.
	out, err = s.PresenceS.GetAppServers(ctx, server.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

func newReverseTunnel(clusterName string, dialAddrs []string) *types.ReverseTunnelV2 {
	return &types.ReverseTunnelV2{
		Kind:    types.KindReverseTunnel,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      clusterName,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
}

func (s *ServicesTestSuite) ReverseTunnelsCRUD(c *check.C) {
	out, err := s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel), check.IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	tunnel.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []types.ReverseTunnel{tunnel})

	err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("", []string{"127.0.0.1:1234"}))
	fixtures.ExpectBadParameter(c, err)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{""}))
	fixtures.ExpectBadParameter(c, err)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{}))
	fixtures.ExpectBadParameter(c, err)
}

func (s *ServicesTestSuite) PasswordHashCRUD(c *check.C) {
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

func (s *ServicesTestSuite) WebSessionCRUD(c *check.C) {
	ctx := context.Background()
	req := types.GetWebSessionRequest{User: "user1", SessionID: "sid1"}
	_, err := s.WebS.WebSessions().Get(ctx, req)
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("%#v", err))

	dt := s.Clock.Now().Add(1 * time.Minute)
	ws, err := types.NewWebSession("sid1", types.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub123"),
			Priv:    []byte("priv123"),
			Expires: dt,
		})
	c.Assert(err, check.IsNil)

	err = s.WebS.WebSessions().Upsert(ctx, ws)
	c.Assert(err, check.IsNil)

	out, err := s.WebS.WebSessions().Get(ctx, req)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, ws)

	ws1, err := types.NewWebSession("sid1", types.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub321"),
			Priv:    []byte("priv321"),
			Expires: dt,
		})
	c.Assert(err, check.IsNil)

	err = s.WebS.WebSessions().Upsert(ctx, ws1)
	c.Assert(err, check.IsNil)

	out2, err := s.WebS.WebSessions().Get(ctx, req)
	c.Assert(err, check.IsNil)
	c.Assert(out2, check.DeepEquals, ws1)

	c.Assert(s.WebS.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
		User:      req.User,
		SessionID: req.SessionID,
	}), check.IsNil)

	_, err = s.WebS.WebSessions().Get(ctx, req)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) TokenCRUD(c *check.C) {
	ctx := context.Background()
	_, err := s.ProvisioningS.GetToken(ctx, "token")
	fixtures.ExpectNotFound(c, err)

	t, err := types.NewProvisionToken("token", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
	c.Assert(err, check.IsNil)

	c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

	token, err := s.ProvisioningS.GetToken(ctx, "token")
	c.Assert(err, check.IsNil)
	c.Assert(token.GetRoles().Include(types.RoleAuth), check.Equals, true)
	c.Assert(token.GetRoles().Include(types.RoleNode), check.Equals, true)
	c.Assert(token.GetRoles().Include(types.RoleProxy), check.Equals, false)
	diff := s.Clock.Now().UTC().Add(defaults.ProvisioningTokenTTL).Second() - token.Expiry().Second()
	if diff > 1 {
		c.Fatalf("expected diff to be within one second, got %v instead", diff)
	}

	c.Assert(s.ProvisioningS.DeleteToken(ctx, "token"), check.IsNil)

	_, err = s.ProvisioningS.GetToken(ctx, "token")
	fixtures.ExpectNotFound(c, err)

	// check tokens backwards compatibility and marshal/unmarshal
	expiry := time.Now().UTC().Add(time.Hour)
	v1 := &types.ProvisionTokenV1{
		Token:   "old",
		Roles:   types.SystemRoles{types.RoleNode, types.RoleProxy},
		Expires: expiry,
	}
	v2, err := types.NewProvisionToken(v1.Token, v1.Roles, expiry)
	c.Assert(err, check.IsNil)

	// Tokens in different version formats are backwards and forwards
	// compatible
	fixtures.DeepCompare(c, v1.V2(), v2)
	fixtures.DeepCompare(c, v2.V1(), v1)

	// Marshal V1, unmarshal V2
	data, err := services.MarshalProvisionToken(v2, services.WithVersion(types.V1))
	c.Assert(err, check.IsNil)

	out, err := services.UnmarshalProvisionToken(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, out, v2)

	// Test delete all tokens
	t, err = types.NewProvisionToken("token1", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
	c.Assert(err, check.IsNil)
	c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

	t, err = types.NewProvisionToken("token2", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
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

func (s *ServicesTestSuite) RolesCRUD(c *check.C) {
	ctx := context.Background()

	out, err := s.Access.GetRoles(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	role := types.RoleV4{
		Kind:    types.KindRole,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:      "role1",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				MaxSessionTTL:     types.Duration(time.Hour),
				PortForwarding:    types.NewBoolOption(true),
				CertificateFormat: constants.CertificateFormatStandard,
				BPF:               apidefaults.EnhancedEvents(),
			},
			Allow: types.RoleConditions{
				Logins:           []string{"root", "bob"},
				NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				Namespaces:       []string{apidefaults.Namespace},
				Rules: []types.Rule{
					types.NewRule(types.KindRole, services.RO()),
				},
			},
			Deny: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
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

func (s *ServicesTestSuite) NamespacesCRUD(c *check.C) {
	out, err := s.PresenceS.GetNamespaces()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	ns := types.Namespace{
		Kind:    types.KindNamespace,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      apidefaults.Namespace,
			Namespace: apidefaults.Namespace,
		},
	}
	err = s.PresenceS.UpsertNamespace(ns)
	c.Assert(err, check.IsNil)
	nsout, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
	c.Assert(err, check.IsNil)
	c.Assert(nsout, check.DeepEquals, &ns)

	err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
	c.Assert(err, check.IsNil)

	_, err = s.PresenceS.GetNamespace(ns.Metadata.Name)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) U2FCRUD(c *check.C) {
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
	dev, err := u2f.NewDevice("u2f", &registration, s.Clock.Now())
	c.Assert(err, check.IsNil)
	ctx := context.Background()
	err = s.WebS.UpsertMFADevice(ctx, user1, dev)
	c.Assert(err, check.IsNil)

	devs, err := s.WebS.GetMFADevices(ctx, user1, true)
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
	devs, err = s.WebS.GetMFADevices(ctx, user1, false)
	c.Assert(err, check.IsNil)
	c.Assert(devs, check.HasLen, 2)
}

func (s *ServicesTestSuite) SAMLCRUD(c *check.C) {
	ctx := context.Background()
	connector := &types.SAMLConnectorV2{
		Kind:    types.KindSAML,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "saml1",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.SAMLConnectorSpecV2{
			Issuer:                   "http://example.com",
			SSO:                      "https://example.com/saml/sso",
			AssertionConsumerService: "https://localhost/acs",
			Audience:                 "https://localhost/aud",
			ServiceProviderIssuer:    "https://localhost/iss",
			AttributesToRoles: []types.AttributeMapping{
				{Name: "groups", Value: "admin", Roles: []string{"admin"}},
			},
			Cert: fixtures.TLSCACertPEM,
			SigningKeyPair: &types.AsymmetricKeyPair{
				PrivateKey: fixtures.TLSCAKeyPEM,
				Cert:       fixtures.TLSCACertPEM,
			},
		},
	}
	err := services.ValidateSAMLConnector(connector)
	c.Assert(err, check.IsNil)
	err = s.WebS.UpsertSAMLConnector(ctx, connector)
	c.Assert(err, check.IsNil)
	out, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, out, connector)

	connectors, err := s.WebS.GetSAMLConnectors(ctx, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []types.SAMLConnector{connector}, connectors)

	out2, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), false)
	c.Assert(err, check.IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.SigningKeyPair.PrivateKey = ""
	fixtures.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.WebS.GetSAMLConnectors(ctx, false)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []types.SAMLConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	c.Assert(err, check.IsNil)

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))

	_, err = s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) TunnelConnectionsCRUD(c *check.C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	dt := s.Clock.Now()
	conn, err := types.NewTunnelConnection("conn1", types.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: dt,
	})
	c.Assert(err, check.IsNil)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	out, err = s.PresenceS.GetAllTunnelConnections()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	dt = dt.Add(time.Hour)
	conn.SetLastHeartbeat(dt)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, check.IsNil)

	// test delete individual connection
	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteTunnelConnection(clusterName, conn.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)
}

func (s *ServicesTestSuite) GithubConnectorCRUD(c *check.C) {
	ctx := context.Background()
	connector := &types.GithubConnectorV3{
		Kind:    types.KindGithubConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:      "github",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.GithubConnectorSpecV3{
			ClientID:     "aaa",
			ClientSecret: "bbb",
			RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
			Display:      "Github",
			TeamsToLogins: []types.TeamMapping{
				{
					Organization: "gravitational",
					Team:         "admins",
					Logins:       []string{"admin"},
					KubeGroups:   []string{"system:masters"},
				},
			},
		},
	}
	err := connector.CheckAndSetDefaults()
	c.Assert(err, check.IsNil)
	err = s.WebS.UpsertGithubConnector(ctx, connector)
	c.Assert(err, check.IsNil)
	out, err := s.WebS.GetGithubConnector(ctx, connector.GetName(), true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, out, connector)

	connectors, err := s.WebS.GetGithubConnectors(ctx, true)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []types.GithubConnector{connector}, connectors)

	out2, err := s.WebS.GetGithubConnector(ctx, connector.GetName(), false)
	c.Assert(err, check.IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.ClientSecret = ""
	fixtures.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.WebS.GetGithubConnectors(ctx, false)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, []types.GithubConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.WebS.DeleteGithubConnector(ctx, connector.GetName())
	c.Assert(err, check.IsNil)

	err = s.WebS.DeleteGithubConnector(ctx, connector.GetName())
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))

	_, err = s.WebS.GetGithubConnector(ctx, connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) RemoteClustersCRUD(c *check.C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	rc, err := types.NewRemoteCluster(clusterName)
	c.Assert(err, check.IsNil)

	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)

	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, check.IsNil)

	err = s.PresenceS.CreateRemoteCluster(rc)
	fixtures.ExpectAlreadyExists(c, err)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	rc.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteAllRemoteClusters()
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	// test delete individual connection
	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	c.Assert(err, check.IsNil)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	fixtures.ExpectNotFound(c, err)
}

// AuthPreference tests authentication preference service
func (s *ServicesTestSuite) AuthPreference(c *check.C) {
	ctx := context.Background()
	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:                  "local",
		SecondFactor:          "otp",
		DisconnectExpiredCert: types.NewBoolOption(true),
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetAuthPreference(ctx, ap)
	c.Assert(err, check.IsNil)

	gotAP, err := s.ConfigS.GetAuthPreference(ctx)
	c.Assert(err, check.IsNil)

	c.Assert(gotAP.GetType(), check.Equals, "local")
	c.Assert(gotAP.GetSecondFactor(), check.Equals, constants.SecondFactorOTP)
	c.Assert(gotAP.GetDisconnectExpiredCert(), check.Equals, true)
}

// SessionRecordingConfig tests session recording configuration.
func (s *ServicesTestSuite) SessionRecordingConfig(c *check.C) {
	ctx := context.Background()
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetSessionRecordingConfig(ctx, recConfig)
	c.Assert(err, check.IsNil)

	gotrecConfig, err := s.ConfigS.GetSessionRecordingConfig(ctx)
	c.Assert(err, check.IsNil)

	c.Assert(gotrecConfig.GetMode(), check.Equals, types.RecordAtProxy)
}

func (s *ServicesTestSuite) StaticTokens(c *check.C) {
	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Token:   "tok1",
				Roles:   types.SystemRoles{types.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
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

// Options provides functional arguments
// to turn certain parts of the test suite off
type Options struct {
	// SkipDelete turns off deletes in tests
	SkipDelete bool
}

// Option is a functional suite option
type Option func(s *Options)

// SkipDelete instructs tests to skip testing delete features
func SkipDelete() Option {
	return func(s *Options) {
		s.SkipDelete = true
	}
}

// CollectOptions collects suite options
func CollectOptions(opts ...Option) Options {
	var suiteOpts Options
	for _, o := range opts {
		o(&suiteOpts)
	}
	return suiteOpts
}

// ClusterName tests cluster name.
func (s *ServicesTestSuite) ClusterName(c *check.C, opts ...Option) {
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
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

// ClusterNetworkingConfig tests cluster networking configuration.
func (s *ServicesTestSuite) ClusterNetworkingConfig(c *check.C) {
	ctx := context.Background()
	netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout: types.NewDuration(17 * time.Second),
		KeepAliveCountMax: 3000,
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetClusterNetworkingConfig(ctx, netConfig)
	c.Assert(err, check.IsNil)

	gotNetConfig, err := s.ConfigS.GetClusterNetworkingConfig(ctx)
	c.Assert(err, check.IsNil)

	c.Assert(gotNetConfig.GetClientIdleTimeout(), check.Equals, 17*time.Second)
	c.Assert(gotNetConfig.GetKeepAliveCountMax(), check.Equals, int64(3000))
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

func (s *ServicesTestSuite) SemaphoreFlakiness(c *check.C) {
	ctx := context.Background()
	const renewals = 3
	// wrap our services.Semaphores instance to cause two out of three lease
	// keepalive attempts fail with a meaningless error.  Locks should make
	// at *least* three attempts before their expiry, so locks should not
	// fail under these conditions.
	keepAlives := new(uint64)
	wrapper := &semWrapper{
		Semaphores: s.PresenceS,
		keepAlive: func(ctx context.Context, lease types.SemaphoreLease) error {
			kn := atomic.AddUint64(keepAlives, 1)
			if kn%3 == 0 {
				return s.PresenceS.KeepAliveSemaphoreLease(ctx, lease)
			}
			return trace.Errorf("uh-oh!")
		},
	}

	cfg := services.SemaphoreLockConfig{
		Service:  wrapper,
		Expiry:   time.Second,
		TickRate: time.Millisecond * 50,
		Params: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     1,
		},
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lock, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(cancelCtx)

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
func (s *ServicesTestSuite) SemaphoreContention(c *check.C) {
	ctx := context.Background()
	const locks int64 = 50
	const iters = 5
	for i := 0; i < iters; i++ {
		cfg := services.SemaphoreLockConfig{
			Service: s.PresenceS,
			Expiry:  time.Hour,
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindConnection,
				SemaphoreName: "alice",
				MaxLeases:     locks,
			},
		}
		// we leak lock handles in the spawned goroutines, so
		// context-based cancellation is needed to cleanup the
		// background keepalive activity.
		cancelCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		for i := int64(0); i < locks; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lock, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
				c.Assert(err, check.IsNil)
				go lock.KeepAlive(cancelCtx)
			}()
		}
		wg.Wait()
		cancel()
		c.Assert(s.PresenceS.DeleteSemaphore(ctx, types.SemaphoreFilter{
			SemaphoreKind: cfg.Params.SemaphoreKind,
			SemaphoreName: cfg.Params.SemaphoreName,
		}), check.IsNil)
	}
}

// SemaphoreConcurrency verifies that a large number of concurrent
// acquisitions result in the correct number of successful acquisitions.
func (s *ServicesTestSuite) SemaphoreConcurrency(c *check.C) {
	ctx := context.Background()
	const maxLeases int64 = 20
	const attempts int64 = 200
	cfg := services.SemaphoreLockConfig{
		Service: s.PresenceS,
		Expiry:  time.Hour,
		Params: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     maxLeases,
		},
	}
	// we leak lock handles in the spawned goroutines, so
	// context-based cancellation is needed to cleanup the
	// background keepalive activity.
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var success int64
	var failure int64
	var wg sync.WaitGroup
	for i := int64(0); i < attempts; i++ {
		wg.Add(1)
		go func() {
			lock, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
			if err == nil {
				go lock.KeepAlive(cancelCtx)
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
func (s *ServicesTestSuite) SemaphoreLock(c *check.C) {
	ctx := context.Background()
	cfg := services.SemaphoreLockConfig{
		Service: s.PresenceS,
		Expiry:  time.Hour,
		Params: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindConnection,
			SemaphoreName: "alice",
			MaxLeases:     1,
		},
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	lock, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(cancelCtx)

	// MaxLeases is 1, so second acquire op fails.
	_, err = services.AcquireSemaphoreLock(cancelCtx, cfg)
	fixtures.ExpectLimitExceeded(c, err)

	// Lock is successfully released.
	lock.Stop()
	c.Assert(lock.Wait(), check.IsNil)

	// Acquire new lock with short expiry
	// and high tick rate to verify renewals.
	cfg.Expiry = time.Second
	cfg.TickRate = time.Millisecond * 50
	lock, err = services.AcquireSemaphoreLock(cancelCtx, cfg)
	c.Assert(err, check.IsNil)
	go lock.KeepAlive(cancelCtx)

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
	c.Assert(s.PresenceS.DeleteSemaphore(ctx, types.SemaphoreFilter{
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

// Events tests various events variations
func (s *ServicesTestSuite) Events(c *check.C) {
	ctx := context.Background()
	testCases := []eventTest{
		{
			name: "Cert authority with secrets",
			kind: types.WatchKind{
				Kind:        types.KindCertAuthority,
				LoadSecrets: true,
			},
			crud: func(context.Context) types.Resource {
				ca := NewTestCA(types.UserCA, "example.com")
				c.Assert(s.CAS.UpsertCertAuthority(ca), check.IsNil)

				out, err := s.CAS.GetCertAuthority(*ca.ID(), true)
				c.Assert(err, check.IsNil)

				c.Assert(s.CAS.DeleteCertAuthority(*ca.ID()), check.IsNil)
				return out
			},
		},
	}
	s.runEventsTests(c, testCases)

	testCases = []eventTest{
		{
			name: "Cert authority without secrets",
			kind: types.WatchKind{
				Kind:        types.KindCertAuthority,
				LoadSecrets: false,
			},
			crud: func(context.Context) types.Resource {
				ca := NewTestCA(types.UserCA, "example.com")
				c.Assert(s.CAS.UpsertCertAuthority(ca), check.IsNil)

				out, err := s.CAS.GetCertAuthority(*ca.ID(), false)
				c.Assert(err, check.IsNil)

				c.Assert(s.CAS.DeleteCertAuthority(*ca.ID()), check.IsNil)
				return out
			},
		},
	}
	s.runEventsTests(c, testCases)

	testCases = []eventTest{
		{
			name: "Token",
			kind: types.WatchKind{
				Kind: types.KindToken,
			},
			crud: func(context.Context) types.Resource {
				expires := time.Now().UTC().Add(time.Hour)
				t, err := types.NewProvisionToken("token",
					types.SystemRoles{types.RoleAuth, types.RoleNode}, expires)
				c.Assert(err, check.IsNil)

				c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

				token, err := s.ProvisioningS.GetToken(ctx, "token")
				c.Assert(err, check.IsNil)

				c.Assert(s.ProvisioningS.DeleteToken(ctx, "token"), check.IsNil)
				return token
			},
		},
		{
			name: "Namespace",
			kind: types.WatchKind{
				Kind: types.KindNamespace,
			},
			crud: func(context.Context) types.Resource {
				ns := types.Namespace{
					Kind:    types.KindNamespace,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      "testnamespace",
						Namespace: apidefaults.Namespace,
					},
				}
				err := s.PresenceS.UpsertNamespace(ns)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			name: "Static tokens",
			kind: types.WatchKind{
				Kind: types.KindStaticTokens,
			},
			crud: func(context.Context) types.Resource {
				staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
					StaticTokens: []types.ProvisionTokenV1{
						{
							Token:   "tok1",
							Roles:   types.SystemRoles{types.RoleNode},
							Expires: time.Now().UTC().Add(time.Hour),
						},
					},
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetStaticTokens(staticTokens)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetStaticTokens()
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteStaticTokens()
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			name: "Role",
			kind: types.WatchKind{
				Kind: types.KindRole,
			},
			crud: func(context.Context) types.Resource {
				role, err := types.NewRole("role1", types.RoleSpecV4{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(time.Hour),
					},
					Allow: types.RoleConditions{
						Logins:     []string{"root", "bob"},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Deny: types.RoleConditions{},
				})
				c.Assert(err, check.IsNil)

				err = s.Access.UpsertRole(ctx, role)
				c.Assert(err, check.IsNil)

				out, err := s.Access.GetRole(ctx, role.GetName())
				c.Assert(err, check.IsNil)

				err = s.Access.DeleteRole(ctx, role.GetName())
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			name: "User",
			kind: types.WatchKind{
				Kind: types.KindUser,
			},
			crud: func(context.Context) types.Resource {
				user := newUser("user1", []string{"admin"})
				err := s.Users().UpsertUser(user)
				c.Assert(err, check.IsNil)

				out, err := s.Users().GetUser(user.GetName(), false)
				c.Assert(err, check.IsNil)

				c.Assert(s.Users().DeleteUser(ctx, user.GetName()), check.IsNil)
				return out
			},
		},
		{
			name: "Node",
			kind: types.WatchKind{
				Kind: types.KindNode,
			},
			crud: func(context.Context) types.Resource {
				srv := NewServer(types.KindNode, "srv1", "127.0.0.1:2022", apidefaults.Namespace)

				_, err := s.PresenceS.UpsertNode(ctx, srv)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllNodes(ctx, srv.Metadata.Namespace)
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			name: "Proxy",
			kind: types.WatchKind{
				Kind: types.KindProxy,
			},
			crud: func(context.Context) types.Resource {
				srv := NewServer(types.KindProxy, "srv1", "127.0.0.1:2022", apidefaults.Namespace)

				err := s.PresenceS.UpsertProxy(srv)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetProxies()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllProxies()
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			name: "Tunnel connection",
			kind: types.WatchKind{
				Kind: types.KindTunnelConnection,
			},
			crud: func(context.Context) types.Resource {
				conn, err := types.NewTunnelConnection("conn1", types.TunnelConnectionSpecV2{
					ClusterName:   "example.com",
					ProxyName:     "p1",
					LastHeartbeat: time.Now().UTC(),
				})
				c.Assert(err, check.IsNil)

				err = s.PresenceS.UpsertTunnelConnection(conn)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetTunnelConnections("example.com")
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllTunnelConnections()
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			name: "Reverse tunnel",
			kind: types.WatchKind{
				Kind: types.KindReverseTunnel,
			},
			crud: func(context.Context) types.Resource {
				tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
				c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel), check.IsNil)

				out, err := s.PresenceS.GetReverseTunnels()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			name: "Remote cluster",
			kind: types.WatchKind{
				Kind: types.KindRemoteCluster,
			},
			crud: func(context.Context) types.Resource {
				rc, err := types.NewRemoteCluster("example.com")
				rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
				c.Assert(err, check.IsNil)
				c.Assert(s.PresenceS.CreateRemoteCluster(rc), check.IsNil)

				out, err := s.PresenceS.GetRemoteClusters()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteRemoteCluster(rc.GetName())
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
	}
	s.runEventsTests(c, testCases)

	// Namespace with a name
	testCases = []eventTest{
		{
			name: "Namespace with a name",
			kind: types.WatchKind{
				Kind: types.KindNamespace,
				Name: "shmest",
			},
			crud: func(context.Context) types.Resource {
				ns := types.Namespace{
					Kind:    types.KindNamespace,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      "shmest",
						Namespace: apidefaults.Namespace,
					},
				}
				err := s.PresenceS.UpsertNamespace(ns)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				return out
			},
		},
	}
	s.runEventsTests(c, testCases)
}

// EventsClusterConfig tests cluster config resource events
func (s *ServicesTestSuite) EventsClusterConfig(c *check.C) {
	testCases := []eventTest{
		{
			name: "Cluster name",
			kind: types.WatchKind{
				Kind: types.KindClusterName,
			},
			crud: func(context.Context) types.Resource {
				clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
					ClusterName: "example.com",
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.UpsertClusterName(clusterName)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetClusterName()
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteClusterName()
				c.Assert(err, check.IsNil)
				return out
			},
		},
		{
			name: "Cluster audit configuration",
			kind: types.WatchKind{
				Kind: types.KindClusterAuditConfig,
			},
			crud: func(ctx context.Context) types.Resource {
				auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
					Region:           "us-west-1",
					Type:             "dynamodb",
					AuditSessionsURI: "file:///home/log",
					AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/test/log"},
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetClusterAuditConfig(ctx, auditConfig)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetClusterAuditConfig(ctx)
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteClusterAuditConfig(ctx)
				c.Assert(err, check.IsNil)
				return out
			},
		},
		{
			name: "Cluster networking configuration",
			kind: types.WatchKind{
				Kind: types.KindClusterNetworkingConfig,
			},
			crud: func(ctx context.Context) types.Resource {
				netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
					ClientIdleTimeout: types.Duration(5 * time.Second),
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetClusterNetworkingConfig(ctx, netConfig)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetClusterNetworkingConfig(ctx)
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteClusterNetworkingConfig(ctx)
				c.Assert(err, check.IsNil)
				return out
			},
		},
		{
			name: "Session recording configuration",
			kind: types.WatchKind{
				Kind: types.KindSessionRecordingConfig,
			},
			crud: func(ctx context.Context) types.Resource {
				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode: types.RecordAtProxySync,
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetSessionRecordingConfig(ctx, recConfig)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetSessionRecordingConfig(ctx)
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteSessionRecordingConfig(ctx)
				c.Assert(err, check.IsNil)
				return out
			},
		},
	}
	s.runEventsTests(c, testCases)
}

// NetworkRestrictions tests network restrictions.
func (s *ServicesTestSuite) NetworkRestrictions(c *check.C, opts ...Option) {
	ctx := context.Background()

	// blank slate, should be get/delete should fail
	_, err := s.RestrictionsS.GetNetworkRestrictions(ctx)
	fixtures.ExpectNotFound(c, err)

	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	fixtures.ExpectNotFound(c, err)

	allow := []types.AddressCondition{
		{CIDR: "10.0.1.0/24"},
		{CIDR: "10.0.2.2"},
	}
	deny := []types.AddressCondition{
		{CIDR: "10.1.0.0/16"},
		{CIDR: "8.8.8.8"},
	}

	expected := types.NewNetworkRestrictions()
	expected.SetAllow(allow)
	expected.SetDeny(deny)

	// set and make sure we get it back
	err = s.RestrictionsS.SetNetworkRestrictions(ctx, expected)
	c.Assert(err, check.IsNil)

	actual, err := s.RestrictionsS.GetNetworkRestrictions(ctx)
	c.Assert(err, check.IsNil)

	fixtures.DeepCompare(c, expected.GetAllow(), actual.GetAllow())
	fixtures.DeepCompare(c, expected.GetDeny(), actual.GetDeny())

	// now delete should work ok and get should fail again
	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	c.Assert(err, check.IsNil)

	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) runEventsTests(c *check.C, testCases []eventTest) {
	ctx := context.Background()
	w, err := s.EventsS.NewWatcher(ctx, types.Watch{
		Kinds: eventsTestKinds(testCases),
	})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, types.OpInit)
	case <-w.Done():
		c.Fatalf("Watcher exited with error %v", w.Error())
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	}

	// filter out all events that could have been inserted
	// by the initialization routines
skiploop:
	for {
		select {
		case event := <-w.Events():
			log.Debugf("Skipping pre-test event: %v", event)
			continue skiploop
		default:
			break skiploop
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		}
	}

	for _, tc := range testCases {
		c.Logf("test case %q", tc.name)
		resource := tc.crud(ctx)

		ExpectResource(c, w, 3*time.Second, resource)

		meta := resource.GetMetadata()
		header := &types.ResourceHeader{
			Kind:    resource.GetKind(),
			SubKind: resource.GetSubKind(),
			Version: resource.GetVersion(),
			Metadata: types.Metadata{
				Name:      meta.Name,
				Namespace: meta.Namespace,
			},
		}
		// delete events don't have IDs yet
		header.SetResourceID(0)
		ExpectDeleteResource(c, w, 3*time.Second, header)
	}
}

type eventTest struct {
	name string
	kind types.WatchKind
	crud func(context.Context) types.Resource
}

func eventsTestKinds(tests []eventTest) []types.WatchKind {
	out := make([]types.WatchKind, len(tests))
	for i, tc := range tests {
		out[i] = tc.kind
	}
	return out
}

// ExpectResource expects a Put event of a certain resource
func ExpectResource(c *check.C, w types.Watcher, timeout time.Duration, resource types.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			c.Fatalf("Timeout waiting for event")
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != types.OpPut {
				log.Debugf("Skipping event %+v", event)
				continue
			}
			if resource.GetResourceID() > event.Resource.GetResourceID() {
				log.Debugf("Skipping stale event %v %v %v %v, latest object version is %v", event.Type, event.Resource.GetKind(), event.Resource.GetName(), event.Resource.GetResourceID(), resource.GetResourceID())
				continue waitLoop
			}
			if resource.GetName() != event.Resource.GetName() || resource.GetKind() != event.Resource.GetKind() || resource.GetSubKind() != event.Resource.GetSubKind() {
				log.Debugf("Skipping event %v resource %v, expecting %v", event.Type, event.Resource.GetMetadata(), event.Resource.GetMetadata())
				continue waitLoop
			}
			fixtures.DeepCompare(c, resource, event.Resource)
			break waitLoop
		}
	}
}

// ExpectDeleteResource expects a delete event of a certain kind
func ExpectDeleteResource(c *check.C, w types.Watcher, timeout time.Duration, resource types.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			c.Fatalf("Timeout waiting for delete resource %v", resource)
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != types.OpDelete {
				log.Debugf("Skipping stale event %v %v", event.Type, event.Resource.GetName())
				continue
			}
			fixtures.DeepCompare(c, resource, event.Resource)
			break waitLoop
		}
	}
}
