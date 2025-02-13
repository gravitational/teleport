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
	"crypto/rsa"
	"crypto/x509/pkix"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
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
	PrivateKeys [][]byte
	Clock       clockwork.Clock
	ClusterName string
	// the below string fields default to ClusterName if left empty
	ResourceName        string
	SubjectOrganization string
}

// NewTestCAWithConfig generates a new certificate authority with the specified
// configuration
// Keep this function in-sync with lib/auth/auth.go:newKeySet().
// TODO(jakule): reuse keystore.KeyStore interface to match newKeySet().
func NewTestCAWithConfig(config TestCAConfig) *types.CertAuthorityV2 {
	if config.ResourceName == "" {
		config.ResourceName = config.ClusterName
	}
	if config.SubjectOrganization == "" {
		config.SubjectOrganization = config.ClusterName
	}

	// privateKeys is to specify another RSA private key
	if len(config.PrivateKeys) == 0 {
		// db client CA gets its own private key to distinguish its pub key
		// from the other CAs. Snowflake uses public key to verify JWT signer,
		// so if we don't do this then tests verifying that the correct
		// signer was used are pointless.
		if config.Type == types.DatabaseClientCA {
			config.PrivateKeys = [][]byte{fixtures.PEMBytes["rsa-db-client"]}
		} else {
			config.PrivateKeys = [][]byte{fixtures.PEMBytes["rsa"]}
		}
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
			Organization: []string{config.SubjectOrganization},
		},
		TTL:   defaults.CATTL,
		Clock: config.Clock,
	})
	if err != nil {
		panic(err)
	}

	ca := &types.CertAuthorityV2{
		Kind:    types.KindCertAuthority,
		SubKind: string(config.Type),
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      config.ResourceName,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.CertAuthoritySpecV2{
			Type:        config.Type,
			ClusterName: config.ClusterName,
		},
	}

	// Match the key set to lib/auth/auth.go:newKeySet().
	switch config.Type {
	case types.DatabaseCA, types.DatabaseClientCA, types.SAMLIDPCA:
		ca.Spec.ActiveKeys.TLS = []*types.TLSKeyPair{{Cert: cert, Key: keyBytes}}
	case types.KindJWT, types.OIDCIdPCA:
		// Generating keys is CPU intensive operation. Generate JWT keys only
		// when needed.
		publicKey, privateKey, err := testauthority.New().GenerateJWT()
		if err != nil {
			panic(err)
		}
		ca.Spec.ActiveKeys.JWT = []*types.JWTKeyPair{{
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		}}
	case types.UserCA, types.HostCA:
		ca.Spec.ActiveKeys = types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PublicKey:  ssh.MarshalAuthorizedKey(signer.PublicKey()),
				PrivateKey: keyBytes,
			}},
			TLS: []*types.TLSKeyPair{{Cert: cert, Key: keyBytes}},
		}
	case types.OpenSSHCA:
		ca.Spec.ActiveKeys = types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PublicKey:  ssh.MarshalAuthorizedKey(signer.PublicKey()),
				PrivateKey: keyBytes,
			}},
		}
	default:
		panic("unknown CA type")
	}

	return ca
}

// ServicesTestSuite is an acceptance test suite
// for services. It is used for local implementations and implementations
// using gRPC to guarantee consistency between local and remote services
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

func userSlicesEqual(t *testing.T, a []types.User, b []types.User) {
	require.EqualValuesf(t, len(a), len(b), "a: %#v b: %#v", a, b)

	sort.Sort(services.Users(a))
	sort.Sort(services.Users(b))

	for i := range a {
		usersEqual(t, a[i], b[i])
	}
}

func usersEqual(t *testing.T, a types.User, b types.User) {
	require.True(t, services.UsersEquals(a, b), cmp.Diff(a, b))
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

func (s *ServicesTestSuite) UsersCRUD(t *testing.T) {
	ctx := context.Background()

	u, err := s.WebS.GetUsers(false)
	require.NoError(t, err)
	require.Equal(t, len(u), 0)

	require.NoError(t, s.WebS.UpsertPasswordHash("user1", []byte("hash")))
	require.NoError(t, s.WebS.UpsertPasswordHash("user2", []byte("hash2")))

	u, err = s.WebS.GetUsers(false)
	require.NoError(t, err)
	userSlicesEqual(t, u, []types.User{newUser("user1", nil), newUser("user2", nil)})

	out, err := s.WebS.GetUser("user1", false)
	require.NoError(t, err)
	usersEqual(t, out, u[0])

	user := newUser("user1", []string{"admin", "user"})
	require.NoError(t, s.WebS.UpsertUser(user))

	out, err = s.WebS.GetUser("user1", false)
	require.NoError(t, err)
	usersEqual(t, out, user)

	out, err = s.WebS.GetUser("user1", false)
	require.NoError(t, err)
	usersEqual(t, out, user)

	require.NoError(t, s.WebS.DeleteUser(ctx, "user1"))

	u, err = s.WebS.GetUsers(false)
	require.NoError(t, err)
	userSlicesEqual(t, u, []types.User{newUser("user2", nil)})

	err = s.WebS.DeleteUser(ctx, "user1")
	require.True(t, trace.IsNotFound(err))

	// bad username
	err = s.WebS.UpsertUser(newUser("", nil))
	require.True(t, trace.IsBadParameter(err))
}

func (s *ServicesTestSuite) UsersExpiry(t *testing.T) {
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
	require.NoError(t, err)

	// Make sure the user exists.
	u, err := s.WebS.GetUser("foo", false)
	require.NoError(t, err)
	require.Equal(t, u.GetName(), "foo")

	s.Clock.Advance(2 * time.Minute)

	// Make sure the user is now gone.
	_, err = s.WebS.GetUser("foo", false)
	require.Error(t, err)
}

func (s *ServicesTestSuite) LoginAttempts(t *testing.T) {
	user1 := uuid.NewString()

	user := newUser(user1, []string{"admin", "user"})
	require.NoError(t, s.WebS.UpsertUser(user))

	attempts, err := s.WebS.GetUserLoginAttempts(user.GetName())
	require.NoError(t, err)
	require.Equal(t, len(attempts), 0)

	clock := clockwork.NewFakeClock()
	attempt1 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt1, defaults.AttemptTTL)
	require.NoError(t, err)

	attempt2 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt2, defaults.AttemptTTL)
	require.NoError(t, err)

	attempts, err = s.WebS.GetUserLoginAttempts(user.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(attempts, []services.LoginAttempt{attempt1, attempt2}))
	require.Equal(t, services.LastFailed(3, attempts), false)
	require.Equal(t, services.LastFailed(2, attempts), true)
}

func (s *ServicesTestSuite) CertAuthCRUD(t *testing.T) {
	ctx := context.Background()
	ca := NewTestCA(types.UserCA, "example.com")
	require.NoError(t, s.CAS.UpsertCertAuthority(ctx, ca))

	out, err := s.CAS.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, ca, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	cas, err := s.CAS.GetCertAuthorities(ctx, types.UserCA, false)
	require.NoError(t, err)
	ca2 := ca.Clone().(*types.CertAuthorityV2)
	ca2.Spec.ActiveKeys.SSH[0].PrivateKey = nil
	ca2.Spec.ActiveKeys.TLS[0].Key = nil
	require.Empty(t, cmp.Diff(cas[0], ca2, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	cas, err = s.CAS.GetCertAuthorities(ctx, types.UserCA, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cas[0], ca, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	cas, err = s.CAS.GetCertAuthorities(ctx, types.UserCA, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cas[0], ca, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.CAS.DeleteCertAuthority(ctx, *ca.ID())
	require.NoError(t, err)

	// test compare and swap
	ca = NewTestCA(types.UserCA, "example.com")
	require.NoError(t, s.CAS.CreateCertAuthority(ctx, ca))

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
	require.NoError(t, err)

	out, err = s.CAS.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&newCA, out, cmpopts.EquateApproxTime(time.Second), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
			Addr: addr,
		},
	}
}

func (s *ServicesTestSuite) ServerCRUD(t *testing.T) {
	ctx := context.Background()

	// SSH service.
	out, err := s.PresenceS.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	srv := NewServer(types.KindNode, "srv1", "127.0.0.1:2022", apidefaults.Namespace)
	_, err = s.PresenceS.UpsertNode(ctx, srv)
	require.NoError(t, err)

	node, err := s.PresenceS.GetNode(ctx, srv.Metadata.Namespace, srv.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(node, srv, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	out, err = s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(out, []types.Server{srv}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteNode(ctx, srv.Metadata.Namespace, srv.GetName())
	require.NoError(t, err)

	out, err = s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 0)

	// Proxy service.
	out, err = s.PresenceS.GetProxies()
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	proxy := NewServer(types.KindProxy, "proxy1", "127.0.0.1:2023", apidefaults.Namespace)
	require.NoError(t, s.PresenceS.UpsertProxy(ctx, proxy))

	out, err = s.PresenceS.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(out, []types.Server{proxy}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteProxy(ctx, proxy.GetName())
	require.NoError(t, err)

	out, err = s.PresenceS.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 0)

	// Auth service.
	out, err = s.PresenceS.GetAuthServers()
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	auth := NewServer(types.KindAuthServer, "auth1", "127.0.0.1:2025", apidefaults.Namespace)
	require.NoError(t, s.PresenceS.UpsertAuthServer(ctx, auth))

	out, err = s.PresenceS.GetAuthServers()
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(out, []types.Server{auth}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
}

// AppServerCRUD tests CRUD functionality for services.Server.
func (s *ServicesTestSuite) AppServerCRUD(t *testing.T) {
	ctx := context.Background()

	// Expect not to be returned any applications and trace.NotFound.
	out, err := s.PresenceS.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	// Make an app and an app server.
	app, err := types.NewAppV3(types.Metadata{Name: "foo"},
		types.AppSpecV3{URI: "http://127.0.0.1:8080", PublicAddr: "foo.example.com"})
	require.NoError(t, err)
	server, err := types.NewAppServerV3(types.Metadata{
		Name:      app.GetName(),
		Namespace: apidefaults.Namespace,
	}, types.AppServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		App:      app,
	})
	require.NoError(t, err)

	// Upsert application.
	_, err = s.PresenceS.UpsertApplicationServer(ctx, server)
	require.NoError(t, err)

	// Check again, expect a single application to be found.
	out, err = s.PresenceS.GetApplicationServers(ctx, server.GetNamespace())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff([]types.AppServer{server}, out, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	// Remove the application.
	err = s.PresenceS.DeleteApplicationServer(ctx, server.Metadata.Namespace, server.GetHostID(), server.GetName())
	require.NoError(t, err)

	// Now expect no applications to be returned.
	out, err = s.PresenceS.GetApplicationServers(ctx, server.Metadata.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 0)
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

func (s *ServicesTestSuite) ReverseTunnelsCRUD(t *testing.T) {
	out, err := s.PresenceS.GetReverseTunnels(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
	require.NoError(t, s.PresenceS.UpsertReverseTunnel(tunnel))

	out, err = s.PresenceS.GetReverseTunnels(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(out, []types.ReverseTunnel{tunnel}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
	require.NoError(t, err)

	out, err = s.PresenceS.GetReverseTunnels(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("", []string{"127.0.0.1:1234"}))
	require.True(t, trace.IsBadParameter(err))

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{""}))
	require.True(t, trace.IsBadParameter(err))

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{}))
	require.True(t, trace.IsBadParameter(err))
}

func (s *ServicesTestSuite) PasswordHashCRUD(t *testing.T) {
	_, err := s.WebS.GetPasswordHash("user1")
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("%#v", err))

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello123"))
	require.NoError(t, err)

	hash, err := s.WebS.GetPasswordHash("user1")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(hash, []byte("hello123")))

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello321"))
	require.NoError(t, err)

	hash, err = s.WebS.GetPasswordHash("user1")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(hash, []byte("hello321")))
}

func (s *ServicesTestSuite) WebSessionCRUD(t *testing.T) {
	ctx := context.Background()
	req := types.GetWebSessionRequest{User: "user1", SessionID: "sid1"}
	_, err := s.WebS.WebSessions().Get(ctx, req)
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("%#v", err))

	dt := s.Clock.Now().Add(1 * time.Minute)
	ws, err := types.NewWebSession("sid1", types.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub123"),
			Priv:    []byte("priv123"),
			Expires: dt,
		})
	require.NoError(t, err)

	err = s.WebS.WebSessions().Upsert(ctx, ws)
	require.NoError(t, err)

	out, err := s.WebS.WebSessions().Get(ctx, req)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, ws, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	ws1, err := types.NewWebSession("sid1", types.KindWebSession,
		types.WebSessionSpecV2{
			User:    "user1",
			Pub:     []byte("pub321"),
			Priv:    []byte("priv321"),
			Expires: dt,
		})
	require.NoError(t, err)

	err = s.WebS.WebSessions().Upsert(ctx, ws1)
	require.NoError(t, err)

	out2, err := s.WebS.WebSessions().Get(ctx, req)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out2, ws1, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	require.NoError(t, s.WebS.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
		User:      req.User,
		SessionID: req.SessionID,
	}))

	_, err = s.WebS.WebSessions().Get(ctx, req)
	require.True(t, trace.IsNotFound(err))
}

func (s *ServicesTestSuite) TokenCRUD(t *testing.T) {
	ctx := context.Background()
	_, err := s.ProvisioningS.GetToken(ctx, "token")
	require.True(t, trace.IsNotFound(err))

	tok, err := types.NewProvisionToken("token", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
	require.NoError(t, err)

	require.NoError(t, s.ProvisioningS.UpsertToken(ctx, tok))

	token, err := s.ProvisioningS.GetToken(ctx, "token")
	require.NoError(t, err)
	require.Equal(t, token.GetRoles().Include(types.RoleAuth), true)
	require.Equal(t, token.GetRoles().Include(types.RoleNode), true)
	require.Equal(t, token.GetRoles().Include(types.RoleProxy), false)
	require.Equal(t, time.Time{}, token.Expiry())

	require.NoError(t, s.ProvisioningS.DeleteToken(ctx, "token"))

	_, err = s.ProvisioningS.GetToken(ctx, "token")
	require.True(t, trace.IsNotFound(err))

	// check tokens backwards compatibility and marshal/unmarshal
	expiry := time.Now().UTC().Add(time.Hour)
	v1 := &types.ProvisionTokenV1{
		Token:   "old",
		Roles:   types.SystemRoles{types.RoleNode, types.RoleProxy},
		Expires: expiry,
	}
	v2, err := types.NewProvisionToken(v1.Token, v1.Roles, expiry)
	require.NoError(t, err)

	// Tokens in different version formats are backwards and forwards
	// compatible
	require.Empty(t, cmp.Diff(v1.V2(), v2))
	require.Empty(t, cmp.Diff(v2.V1(), v1))

	// Marshal V1, unmarshal V2
	data, err := services.MarshalProvisionToken(v2, services.WithVersion(types.V1))
	require.NoError(t, err)

	out, err := services.UnmarshalProvisionToken(data)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, v2))

	// Test delete all tokens
	tok, err = types.NewProvisionToken("token1", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
	require.NoError(t, err)
	require.NoError(t, s.ProvisioningS.UpsertToken(ctx, tok))

	tok, err = types.NewProvisionToken("token2", types.SystemRoles{types.RoleAuth, types.RoleNode}, time.Time{})
	require.NoError(t, err)
	require.NoError(t, s.ProvisioningS.UpsertToken(ctx, tok))

	tokens, err := s.ProvisioningS.GetTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 2)

	err = s.ProvisioningS.DeleteAllTokens()
	require.NoError(t, err)

	tokens, err = s.ProvisioningS.GetTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 0)
}

func (s *ServicesTestSuite) RolesCRUD(t *testing.T) {
	ctx := context.Background()

	out, err := s.Access.GetRoles(ctx)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	role := types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:      "role1",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
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
	require.NoError(t, err)
	rout, err := s.Access.GetRole(ctx, role.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rout, &role, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	role.Spec.Allow.Logins = []string{"bob"}
	err = s.Access.UpsertRole(ctx, &role)
	require.NoError(t, err)
	rout, err = s.Access.GetRole(ctx, role.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rout, &role, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.Access.DeleteRole(ctx, role.Metadata.Name)
	require.NoError(t, err)

	_, err = s.Access.GetRole(ctx, role.Metadata.Name)
	require.True(t, trace.IsNotFound(err))
}

func (s *ServicesTestSuite) NamespacesCRUD(t *testing.T) {
	out, err := s.PresenceS.GetNamespaces()
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	ns := types.Namespace{
		Kind:    types.KindNamespace,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      apidefaults.Namespace,
			Namespace: apidefaults.Namespace,
		},
	}
	err = s.PresenceS.UpsertNamespace(ns)
	require.NoError(t, err)
	nsout, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(nsout, &ns))

	err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
	require.NoError(t, err)

	_, err = s.PresenceS.GetNamespace(ns.Metadata.Name)
	require.True(t, trace.IsNotFound(err))
}

func (s *ServicesTestSuite) SAMLCRUD(t *testing.T) {
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
	err := services.ValidateSAMLConnector(connector, nil)
	require.NoError(t, err)
	err = s.WebS.UpsertSAMLConnector(ctx, connector)
	require.NoError(t, err)
	out, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, connector))

	connectors, err := s.WebS.GetSAMLConnectors(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.SAMLConnector{connector}, connectors))

	out2, err := s.WebS.GetSAMLConnector(ctx, connector.GetName(), false)
	require.NoError(t, err)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.SigningKeyPair.PrivateKey = ""
	require.Empty(t, cmp.Diff(out2, &connectorNoSecrets))

	connectorsNoSecrets, err := s.WebS.GetSAMLConnectors(ctx, false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.SAMLConnector{&connectorNoSecrets}, connectorsNoSecrets))

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	require.NoError(t, err)

	err = s.WebS.DeleteSAMLConnector(ctx, connector.GetName())
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("expected not found, got %T", err))

	_, err = s.WebS.GetSAMLConnector(ctx, connector.GetName(), true)
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) TunnelConnectionsCRUD(t *testing.T) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	dt := s.Clock.Now()
	conn, err := types.NewTunnelConnection("conn1", types.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: dt,
	})
	require.NoError(t, err)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	require.NoError(t, err)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], conn, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	out, err = s.PresenceS.GetAllTunnelConnections()
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], conn, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	dt = dt.Add(time.Hour)
	conn.SetLastHeartbeat(dt)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	require.NoError(t, err)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], conn, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteAllTunnelConnections()
	require.NoError(t, err)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	err = s.PresenceS.DeleteAllTunnelConnections()
	require.NoError(t, err)

	// test delete individual connection
	err = s.PresenceS.UpsertTunnelConnection(conn)
	require.NoError(t, err)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], conn, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteTunnelConnection(clusterName, conn.GetName())
	require.NoError(t, err)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Equal(t, len(out), 0)
}

func (s *ServicesTestSuite) GithubConnectorCRUD(t *testing.T) {
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
			Display:      "GitHub",
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
	require.NoError(t, err)
	err = s.WebS.UpsertGithubConnector(ctx, connector)
	require.NoError(t, err)
	out, err := s.WebS.GetGithubConnector(ctx, connector.GetName(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, connector))

	connectors, err := s.WebS.GetGithubConnectors(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.GithubConnector{connector}, connectors))

	out2, err := s.WebS.GetGithubConnector(ctx, connector.GetName(), false)
	require.NoError(t, err)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.ClientSecret = ""
	require.Empty(t, cmp.Diff(out2, &connectorNoSecrets))

	connectorsNoSecrets, err := s.WebS.GetGithubConnectors(ctx, false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.GithubConnector{&connectorNoSecrets}, connectorsNoSecrets))

	err = s.WebS.DeleteGithubConnector(ctx, connector.GetName())
	require.NoError(t, err)

	err = s.WebS.DeleteGithubConnector(ctx, connector.GetName())
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("expected not found, got %T", err))

	_, err = s.WebS.GetGithubConnector(ctx, connector.GetName(), true)
	require.Equal(t, trace.IsNotFound(err), true, fmt.Sprintf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) RemoteClustersCRUD(t *testing.T) {
	ctx := context.Background()
	clusterName := "example.com"
	out, err := s.PresenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	rc, err := types.NewRemoteCluster(clusterName)
	require.NoError(t, err)

	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)

	err = s.PresenceS.CreateRemoteCluster(rc)
	require.NoError(t, err)

	err = s.PresenceS.CreateRemoteCluster(rc)
	require.True(t, trace.IsAlreadyExists(err))

	out, err = s.PresenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], rc, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteAllRemoteClusters()
	require.NoError(t, err)

	out, err = s.PresenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Equal(t, len(out), 0)

	// test delete individual connection
	err = s.PresenceS.CreateRemoteCluster(rc)
	require.NoError(t, err)

	out, err = s.PresenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Equal(t, len(out), 1)
	require.Empty(t, cmp.Diff(out[0], rc, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.PresenceS.DeleteRemoteCluster(ctx, clusterName)
	require.NoError(t, err)

	err = s.PresenceS.DeleteRemoteCluster(ctx, clusterName)
	require.True(t, trace.IsNotFound(err))
}

// AuthPreference tests authentication preference service
func (s *ServicesTestSuite) AuthPreference(t *testing.T) {
	ctx := context.Background()
	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:                  "local",
		SecondFactor:          "otp",
		DisconnectExpiredCert: types.NewBoolOption(true),
	})
	require.NoError(t, err)

	err = s.ConfigS.SetAuthPreference(ctx, ap)
	require.NoError(t, err)

	gotAP, err := s.ConfigS.GetAuthPreference(ctx)
	require.NoError(t, err)

	require.Equal(t, gotAP.GetType(), "local")
	require.Equal(t, gotAP.GetSecondFactor(), constants.SecondFactorOTP)
	require.Equal(t, gotAP.GetDisconnectExpiredCert(), true)
}

// SessionRecordingConfig tests session recording configuration.
func (s *ServicesTestSuite) SessionRecordingConfig(t *testing.T) {
	ctx := context.Background()
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)

	err = s.ConfigS.SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	gotrecConfig, err := s.ConfigS.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)

	require.Equal(t, gotrecConfig.GetMode(), types.RecordAtProxy)
}

func (s *ServicesTestSuite) StaticTokens(t *testing.T) {
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
	require.NoError(t, err)

	err = s.ConfigS.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	out, err := s.ConfigS.GetStaticTokens()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(staticTokens, out, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.ConfigS.DeleteStaticTokens()
	require.NoError(t, err)

	_, err = s.ConfigS.GetStaticTokens()
	require.True(t, trace.IsNotFound(err))
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
func (s *ServicesTestSuite) ClusterName(t *testing.T, opts ...Option) {
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = s.ConfigS.SetClusterName(clusterName)
	require.NoError(t, err)

	gotName, err := s.ConfigS.GetClusterName()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(clusterName, gotName, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	err = s.ConfigS.DeleteClusterName()
	require.NoError(t, err)

	_, err = s.ConfigS.GetClusterName()
	require.True(t, trace.IsNotFound(err))

	err = s.ConfigS.UpsertClusterName(clusterName)
	require.NoError(t, err)

	gotName, err = s.ConfigS.GetClusterName()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(clusterName, gotName, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
}

// ClusterNetworkingConfig tests cluster networking configuration.
func (s *ServicesTestSuite) ClusterNetworkingConfig(t *testing.T) {
	ctx := context.Background()
	netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout: types.NewDuration(17 * time.Second),
		KeepAliveCountMax: 3000,
	})
	require.NoError(t, err)

	err = s.ConfigS.SetClusterNetworkingConfig(ctx, netConfig)
	require.NoError(t, err)

	gotNetConfig, err := s.ConfigS.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err)

	require.Equal(t, gotNetConfig.GetClientIdleTimeout(), 17*time.Second)
	require.Equal(t, gotNetConfig.GetKeepAliveCountMax(), int64(3000))
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

func (s *ServicesTestSuite) SemaphoreFlakiness(t *testing.T) {
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
		Expiry:   time.Hour,
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
	require.NoError(t, err)

	for i := 0; i < renewals; i++ {
		select {
		case <-lock.Renewed():
			continue
		case <-lock.Done():
			t.Fatalf("Lost semaphore lock: %v", lock.Wait())
		case <-time.After(time.Second * 30):
			t.Fatalf("Timeout waiting for renewals")
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
func (s *ServicesTestSuite) SemaphoreContention(t *testing.T) {
	ctx := context.Background()
	const locks int64 = 50
	const iters = 5
	for i := 0; i < iters; i++ {
		cfg := services.SemaphoreLockConfig{
			Service: s.PresenceS,
			Expiry:  time.Hour,
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindConnection,
				SemaphoreName: fmt.Sprintf("sem-%d", i), // avoid overlap between iterations
				MaxLeases:     locks,
			},
		}
		// we leak lock handles in the spawned goroutines, so
		// context-based cancellation is needed to cleanup the
		// background keepalive activity.
		cancelCtx, cancel := context.WithCancel(ctx)
		acquireErrs := make(chan error, locks)
		for i := int64(0); i < locks; i++ {
			go func() {
				_, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
				acquireErrs <- err
			}()
		}
		for i := int64(0); i < locks; i++ {
			require.NoError(t, <-acquireErrs)
		}
		cancel()
		require.NoError(t, s.PresenceS.DeleteSemaphore(ctx, types.SemaphoreFilter{
			SemaphoreKind: cfg.Params.SemaphoreKind,
			SemaphoreName: cfg.Params.SemaphoreName,
		}))
	}
}

// SemaphoreConcurrency verifies that a large number of concurrent
// acquisitions result in the correct number of successful acquisitions.
func (s *ServicesTestSuite) SemaphoreConcurrency(t *testing.T) {
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
			_, err := services.AcquireSemaphoreLock(cancelCtx, cfg)
			if err == nil {
				atomic.AddInt64(&success, 1)
			} else {
				atomic.AddInt64(&failure, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, atomic.LoadInt64(&success), maxLeases)
	require.Equal(t, atomic.LoadInt64(&failure), attempts-maxLeases)
}

// SemaphoreLock verifies correct functionality of the basic
// semaphore lock scenarios.
func (s *ServicesTestSuite) SemaphoreLock(t *testing.T) {
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
	require.NoError(t, err)

	// MaxLeases is 1, so second acquire op fails.
	_, err = services.AcquireSemaphoreLock(cancelCtx, cfg)
	require.True(t, trace.IsLimitExceeded(err))

	// Lock is successfully released.
	lock.Stop()
	require.NoError(t, lock.Wait())

	// Acquire new lock with short expiry
	// and high tick rate to verify renewals.
	cfg.Expiry = time.Second
	cfg.TickRate = time.Millisecond * 50
	lock, err = services.AcquireSemaphoreLock(cancelCtx, cfg)
	require.NoError(t, err)

	timeout := time.After(time.Second)

	for i := 0; i < 3; i++ {
		select {
		case <-lock.Done():
			t.Fatalf("Unexpected lock failure: %v", lock.Wait())
		case <-timeout:
			t.Fatalf("Timeout waiting for lock renewal %d", i)
		case <-lock.Renewed():
		}
	}

	// forcibly delete the semaphore
	require.NoError(t, s.PresenceS.DeleteSemaphore(ctx, types.SemaphoreFilter{
		SemaphoreKind: cfg.Params.SemaphoreKind,
		SemaphoreName: cfg.Params.SemaphoreName,
	}))

	select {
	case <-lock.Done():
		require.True(t, trace.IsNotFound(lock.Wait()))
	case <-time.After(time.Millisecond * 1500):
		t.Errorf("timeout waiting for semaphore lock failure")
	}
}

// Events tests various events variations
func (s *ServicesTestSuite) Events(t *testing.T) {
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
				require.NoError(t, s.CAS.UpsertCertAuthority(ctx, ca))

				out, err := s.CAS.GetCertAuthority(ctx, *ca.ID(), true)
				require.NoError(t, err)

				require.NoError(t, s.CAS.DeleteCertAuthority(ctx, *ca.ID()))
				return out
			},
		},
	}
	s.runEventsTests(t, testCases, types.Watch{Kinds: eventsTestKinds(testCases)})

	testCases = []eventTest{
		{
			name: "Cert authority without secrets",
			kind: types.WatchKind{
				Kind:        types.KindCertAuthority,
				LoadSecrets: false,
			},
			crud: func(context.Context) types.Resource {
				ca := NewTestCA(types.UserCA, "example.com")
				require.NoError(t, s.CAS.UpsertCertAuthority(ctx, ca))

				out, err := s.CAS.GetCertAuthority(ctx, *ca.ID(), false)
				require.NoError(t, err)

				require.NoError(t, s.CAS.DeleteCertAuthority(ctx, *ca.ID()))
				return out
			},
		},
	}
	s.runEventsTests(t, testCases, types.Watch{Kinds: eventsTestKinds(testCases)})

	testCases = []eventTest{
		{
			name: "Token",
			kind: types.WatchKind{
				Kind: types.KindToken,
			},
			crud: func(context.Context) types.Resource {
				expires := time.Now().UTC().Add(time.Hour)
				tok, err := types.NewProvisionToken("token",
					types.SystemRoles{types.RoleAuth, types.RoleNode}, expires)
				require.NoError(t, err)

				require.NoError(t, s.ProvisioningS.UpsertToken(ctx, tok))

				token, err := s.ProvisioningS.GetToken(ctx, "token")
				require.NoError(t, err)

				require.NoError(t, s.ProvisioningS.DeleteToken(ctx, "token"))
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
				require.NoError(t, err)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				require.NoError(t, err)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = s.ConfigS.SetStaticTokens(staticTokens)
				require.NoError(t, err)

				out, err := s.ConfigS.GetStaticTokens()
				require.NoError(t, err)

				err = s.ConfigS.DeleteStaticTokens()
				require.NoError(t, err)

				return out
			},
		},
		{
			name: "Role",
			kind: types.WatchKind{
				Kind: types.KindRole,
			},
			crud: func(context.Context) types.Resource {
				role, err := types.NewRole("role1", types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(time.Hour),
					},
					Allow: types.RoleConditions{
						Logins:     []string{"root", "bob"},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Deny: types.RoleConditions{},
				})
				require.NoError(t, err)

				err = s.Access.UpsertRole(ctx, role)
				require.NoError(t, err)

				out, err := s.Access.GetRole(ctx, role.GetName())
				require.NoError(t, err)

				err = s.Access.DeleteRole(ctx, role.GetName())
				require.NoError(t, err)

				return out
			},
		},
		{
			name: "User",
			kind: types.WatchKind{
				Kind: types.KindUser,
			},
			crud: func(context.Context) types.Resource {
				user := newUser("user1", []string{constants.DefaultImplicitRole})
				err := s.Users().UpsertUser(user)
				require.NoError(t, err)

				out, err := s.Users().GetUser(user.GetName(), false)
				require.NoError(t, err)

				require.NoError(t, s.Users().DeleteUser(ctx, user.GetName()))
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
				require.NoError(t, err)

				out, err := s.PresenceS.GetNodes(ctx, srv.Metadata.Namespace)
				require.NoError(t, err)

				err = s.PresenceS.DeleteAllNodes(ctx, srv.Metadata.Namespace)
				require.NoError(t, err)

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

				err := s.PresenceS.UpsertProxy(ctx, srv)
				require.NoError(t, err)

				out, err := s.PresenceS.GetProxies()
				require.NoError(t, err)

				err = s.PresenceS.DeleteAllProxies()
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = s.PresenceS.UpsertTunnelConnection(conn)
				require.NoError(t, err)

				out, err := s.PresenceS.GetTunnelConnections("example.com")
				require.NoError(t, err)

				err = s.PresenceS.DeleteAllTunnelConnections()
				require.NoError(t, err)

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
				require.NoError(t, s.PresenceS.UpsertReverseTunnel(tunnel))

				out, err := s.PresenceS.GetReverseTunnels(context.Background())
				require.NoError(t, err)

				err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
				require.NoError(t, err)

				return out[0]
			},
		},
		{
			name: "Remote cluster",
			kind: types.WatchKind{
				Kind: types.KindRemoteCluster,
			},
			crud: func(ctx context.Context) types.Resource {
				rc, err := types.NewRemoteCluster("example.com")
				rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
				require.NoError(t, err)
				require.NoError(t, s.PresenceS.CreateRemoteCluster(rc))

				out, err := s.PresenceS.GetRemoteClusters()
				require.NoError(t, err)

				err = s.PresenceS.DeleteRemoteCluster(ctx, rc.GetName())
				require.NoError(t, err)

				return out[0]
			},
		},
	}
	// this also tests the partial success mode by requesting an unknown kind
	s.runEventsTests(t, testCases, types.Watch{
		Kinds:               append(eventsTestKinds(testCases), types.WatchKind{Kind: "unknown"}),
		AllowPartialSuccess: true,
	})

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
				require.NoError(t, err)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				require.NoError(t, err)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				require.NoError(t, err)

				return out
			},
		},
	}
	s.runEventsTests(t, testCases, types.Watch{Kinds: eventsTestKinds(testCases)})

	// tests that a watch fails given an unknown kind when the partial success mode is not enabled
	s.runUnknownEventsTest(t, types.Watch{Kinds: []types.WatchKind{
		{Kind: types.KindNamespace},
		{Kind: "unknown"},
	}})

	// tests that a watch fails if all given kinds are unknown even if the success mode is enabled
	s.runUnknownEventsTest(t, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: "unrecognized"},
			{Kind: "unidentified"},
		},
		AllowPartialSuccess: true,
	})
}

// EventsClusterConfig tests cluster config resource events
func (s *ServicesTestSuite) EventsClusterConfig(t *testing.T) {
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
				require.NoError(t, err)

				err = s.ConfigS.UpsertClusterName(clusterName)
				require.NoError(t, err)

				out, err := s.ConfigS.GetClusterName()
				require.NoError(t, err)

				err = s.ConfigS.DeleteClusterName()
				require.NoError(t, err)
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
				require.NoError(t, err)

				err = s.ConfigS.SetClusterAuditConfig(ctx, auditConfig)
				require.NoError(t, err)

				out, err := s.ConfigS.GetClusterAuditConfig(ctx)
				require.NoError(t, err)

				err = s.ConfigS.DeleteClusterAuditConfig(ctx)
				require.NoError(t, err)
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
				require.NoError(t, err)

				err = s.ConfigS.SetClusterNetworkingConfig(ctx, netConfig)
				require.NoError(t, err)

				out, err := s.ConfigS.GetClusterNetworkingConfig(ctx)
				require.NoError(t, err)

				err = s.ConfigS.DeleteClusterNetworkingConfig(ctx)
				require.NoError(t, err)
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
				require.NoError(t, err)

				err = s.ConfigS.SetSessionRecordingConfig(ctx, recConfig)
				require.NoError(t, err)

				out, err := s.ConfigS.GetSessionRecordingConfig(ctx)
				require.NoError(t, err)

				err = s.ConfigS.DeleteSessionRecordingConfig(ctx)
				require.NoError(t, err)
				return out
			},
		},
	}
	s.runEventsTests(t, testCases, types.Watch{Kinds: eventsTestKinds(testCases)})
}

// NetworkRestrictions tests network restrictions.
func (s *ServicesTestSuite) NetworkRestrictions(t *testing.T, opts ...Option) {
	ctx := context.Background()

	// blank slate, should be get/delete should fail
	_, err := s.RestrictionsS.GetNetworkRestrictions(ctx)
	require.True(t, trace.IsNotFound(err))

	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	require.True(t, trace.IsNotFound(err))

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
	require.NoError(t, err)

	actual, err := s.RestrictionsS.GetNetworkRestrictions(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(expected.GetAllow(), actual.GetAllow()))
	require.Empty(t, cmp.Diff(expected.GetDeny(), actual.GetDeny()))

	// now delete should work ok and get should fail again
	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	require.NoError(t, err)

	err = s.RestrictionsS.DeleteNetworkRestrictions(ctx)
	require.True(t, trace.IsNotFound(err))
}

func (s *ServicesTestSuite) runEventsTests(t *testing.T, testCases []eventTest, watch types.Watch) {
	ctx := context.Background()
	w, err := s.EventsS.NewWatcher(ctx, watch)
	require.NoError(t, err)
	defer w.Close()

	select {
	case event := <-w.Events():
		require.Equal(t, event.Type, types.OpInit)
		watchStatus, ok := event.Resource.(types.WatchStatus)
		require.True(t, ok)
		expectedKinds := eventsTestKinds(testCases)
		require.Equal(t, expectedKinds, watchStatus.GetKinds())
	case <-w.Done():
		t.Fatalf("Watcher exited with error %v", w.Error())
	case <-time.After(2 * time.Second):
		t.Fatalf("Timeout waiting for init event")
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
			t.Fatalf("Watcher exited with error %v", w.Error())
		}
	}

	for _, tc := range testCases {
		t.Logf("test case %q", tc.name)
		resource := tc.crud(ctx)

		ExpectResource(t, w, 3*time.Second, resource)

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
		ExpectDeleteResource(t, w, 3*time.Second, header)
	}
}

func (s *ServicesTestSuite) runUnknownEventsTest(t *testing.T, watch types.Watch) {
	ctx := context.Background()
	w, err := s.EventsS.NewWatcher(ctx, watch)
	if err != nil {
		// depending on the implementation of EventsS, it might fail here immediately
		// or later before returning the first event from the watcher.
		return
	}
	defer w.Close()

	select {
	case <-w.Events():
		t.Fatal("unexpected event from watcher that is supposed to fail")
	case <-w.Done():
		require.Error(t, w.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error from watcher")
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
func ExpectResource(t *testing.T, w types.Watcher, timeout time.Duration, resource types.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			t.Fatalf("Timeout waiting for event")
		case <-w.Done():
			t.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != types.OpPut {
				log.Debugf("Skipping event %+v", event)
				continue
			}
			if resource.GetName() != event.Resource.GetName() || resource.GetKind() != event.Resource.GetKind() || resource.GetSubKind() != event.Resource.GetSubKind() {
				log.Debugf("Skipping event %v resource %v, expecting %v", event.Type, event.Resource.GetMetadata(), event.Resource.GetMetadata())
				continue waitLoop
			}
			require.Empty(t, cmp.Diff(resource, event.Resource))
			break waitLoop
		}
	}
}

// ExpectDeleteResource expects a delete event of a certain kind
func ExpectDeleteResource(t *testing.T, w types.Watcher, timeout time.Duration, resource types.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			t.Fatalf("Timeout waiting for delete resource %v", resource)
		case <-w.Done():
			t.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != types.OpDelete {
				log.Debugf("Skipping stale event %v %v", event.Type, event.Resource.GetName())
				continue
			}

			// Server resources may have subkind set, but the backend
			// generating this delete event doesn't know the subkind.
			// Set it to prevent the check below from failing.
			if event.Resource.GetKind() == types.KindNode {
				event.Resource.SetSubKind(resource.GetSubKind())
			}

			require.Empty(t, cmp.Diff(resource, event.Resource))
			break waitLoop
		}
	}
}
