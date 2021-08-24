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

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/google/go-cmp/cmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/coreos/go-oidc/jose"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	. "gopkg.in/check.v1"
)

type testPack struct {
	bk          backend.Backend
	clusterName types.ClusterName
	a           *Server
	mockEmitter *events.MockEmitter
}

func newTestPack(ctx context.Context, dataDir string) (testPack, error) {
	var (
		p   testPack
		err error
	)
	p.bk, err = lite.NewWithConfig(ctx, lite.Config{Path: dataDir})
	if err != nil {
		return p, trace.Wrap(err)
	}
	p.clusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "test.localhost",
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	authConfig := &InitConfig{
		Backend:                p.bk,
		ClusterName:            p.clusterName,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	p.a, err = NewServer(authConfig)
	if err != nil {
		return p, trace.Wrap(err)
	}

	// set lock watcher
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    p.a,
		},
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	p.a.SetLockWatcher(lockWatcher)

	// set cluster name
	err = p.a.SetClusterName(p.clusterName)
	if err != nil {
		return p, trace.Wrap(err)
	}

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	err = p.a.SetStaticTokens(staticTokens)
	if err != nil {
		return p, trace.Wrap(err)
	}

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetAuthPreference(ctx, authPreference); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetClusterConfig(types.DefaultClusterConfig()); err != nil {
		return p, trace.Wrap(err)
	}

	if err := p.a.UpsertCertAuthority(suite.NewTestCA(types.UserCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.UpsertCertAuthority(suite.NewTestCA(types.HostCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}

	p.mockEmitter = &events.MockEmitter{}
	p.a.emitter = p.mockEmitter
	return p, nil
}

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestAPI(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	testPack
}

var _ = Suite(&AuthSuite{})

func (s *AuthSuite) SetUpTest(c *C) {
	p, err := newTestPack(context.Background(), c.MkDir())
	c.Assert(err, IsNil)
	s.testPack = p
}

func (s *AuthSuite) TearDownTest(c *C) {
	if s.bk != nil {
		s.bk.Close()
	}
}

func (s *AuthSuite) TestSessions(c *C) {
	ctx := context.Background()

	user := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSessionInfo(ctx, user, ws.GetName())
	c.Assert(err, IsNil)
	ws.SetPriv(nil)
	fixtures.DeepCompare(c, ws, out)

	err = s.a.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
		User:      user,
		SessionID: ws.GetName(),
	})
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(ctx, types.GetWebSessionRequest{
		User:      user,
		SessionID: ws.GetName(),
	})
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *AuthSuite) TestAuthenticateSSHUser(c *C) {
	ctx := context.Background()

	// Register the leaf cluster.
	leaf, err := types.NewRemoteCluster("leaf.localhost")
	c.Assert(err, IsNil)
	c.Assert(s.a.CreateRemoteCluster(leaf), IsNil)

	user := "user1"
	pass := []byte("abc123")

	// Try to login as an unknown user.
	_, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
	})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)

	// Create the user.
	_, role, err := CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)
	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)
	// Give the role some k8s principals too.
	role.SetKubeUsers(services.Allow, []string{user})
	role.SetKubeGroups(services.Allow, []string{"system:masters"})
	err = s.a.UpsertRole(ctx, role)
	c.Assert(err, IsNil)

	kg := testauthority.New()
	_, pub, err := kg.GetNewKeyPairFromPool()
	c.Assert(err, IsNil)

	// Login to the root cluster.
	resp, err := s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:      pub,
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
	})
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, user)
	// Verify the public key and principals in SSH cert.
	inSSHPub, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
	gotSSHCert, err := sshutils.ParseCertificate(resp.Cert)
	c.Assert(err, IsNil)
	c.Assert(gotSSHCert.Key, DeepEquals, inSSHPub)
	c.Assert(gotSSHCert.ValidPrincipals, DeepEquals, []string{user})
	// Verify the public key and Subject in TLS cert.
	inCryptoPub := inSSHPub.(ssh.CryptoPublicKey).CryptoPublicKey()
	gotTLSCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	c.Assert(gotTLSCert.PublicKey, DeepEquals, inCryptoPub)
	wantID := tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		Expires:          gotTLSCert.NotAfter,
		RouteToCluster:   s.clusterName.GetClusterName(),
		TeleportCluster:  s.clusterName.GetClusterName(),
	}
	gotID, err := tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Login to the leaf cluster.
	resp, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:         pub,
		TTL:               time.Hour,
		RouteToCluster:    "leaf.localhost",
		KubernetesCluster: "leaf-kube-cluster",
	})
	c.Assert(err, IsNil)
	// Verify the TLS cert has the correct RouteToCluster set.
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	wantID = tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		// It's OK to use a non-existent kube cluster for leaf teleport
		// clusters. The leaf is responsible for validating those.
		KubernetesCluster: "leaf-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    "leaf.localhost",
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Register a kubernetes cluster to verify the defaulting logic in TLS cert
	// generation.
	err = s.a.UpsertKubeService(ctx, &types.ServerV2{
		Metadata: types.Metadata{Name: "kube-service"},
		Kind:     types.KindKubeService,
		Version:  types.V2,
		Spec: types.ServerSpecV2{
			KubernetesClusters: []*types.KubernetesCluster{{Name: "root-kube-cluster"}},
		},
	})
	c.Assert(err, IsNil)

	// Login specifying a valid kube cluster. It should appear in the TLS cert.
	resp, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:         pub,
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "root-kube-cluster",
	})
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Login without specifying kube cluster. A registered one should be picked
	// automatically.
	resp, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:      pub,
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
		// Intentionally empty, auth server should default to a registered
		// kubernetes cluster.
		KubernetesCluster: "",
	})
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Register a kubernetes cluster to verify the defaulting logic in TLS cert
	// generation.
	err = s.a.UpsertKubeService(ctx, &types.ServerV2{
		Metadata: types.Metadata{Name: "kube-service"},
		Kind:     types.KindKubeService,
		Version:  types.V2,
		Spec: types.ServerSpecV2{
			KubernetesClusters: []*types.KubernetesCluster{{Name: "root-kube-cluster"}},
		},
	})
	c.Assert(err, IsNil)

	// Login specifying a valid kube cluster. It should appear in the TLS cert.
	resp, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:         pub,
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "root-kube-cluster",
	})
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Login without specifying kube cluster. A registered one should be picked
	// automatically.
	resp, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:      pub,
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
		// Intentionally empty, auth server should default to a registered
		// kubernetes cluster.
		KubernetesCluster: "",
	})
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	c.Assert(err, IsNil)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	c.Assert(err, IsNil)
	c.Assert(*gotID, DeepEquals, wantID)

	// Login specifying an invalid kube cluster. This should fail.
	_, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
		PublicKey:         pub,
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "invalid-kube-cluster",
	})
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestUserLock(c *C) {
	username := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(username, pass)
	c.Assert(err, IsNil)

	// successful log in
	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	for i := 0; i <= defaults.MaxLoginAttempts; i++ {
		_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
			Username: username,
			Pass:     &PassCreds{Password: []byte("wrong pass")},
		})
		c.Assert(err, NotNil)
	}

	user, err := s.a.Identity.GetUser(username, false)
	c.Assert(err, IsNil)
	c.Assert(user.GetStatus().IsLocked, Equals, true)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	ctx := context.Background()

	// before we do anything, we should have 0 tokens
	btokens, err := s.a.GetTokens(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(btokens), Equals, 0)

	// generate persistent token
	tok, err := s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleNode}})
	c.Assert(err, IsNil)
	c.Assert(len(tok), Equals, 2*TokenLenBytes)
	tokens, err := s.a.GetTokens(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].GetName(), Equals, tok)

	roles, _, err := s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(types.RoleNode), Equals, true)
	c.Assert(roles.Include(types.RoleProxy), Equals, false)

	// unsuccessful registration (wrong role)
	keys, err := s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    tok,
		HostID:   "bad-host-id",
		NodeName: "bad-node-name",
		Role:     types.RoleProxy,
	})
	c.Assert(keys, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, `node "bad-node-name" \[bad-host-id\] can not join the cluster, the token does not allow "Proxy" role`)

	// generate predefined token
	customToken := "custom-token"
	tok, err = s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleNode}, Token: customToken})
	c.Assert(err, IsNil)
	c.Assert(tok, Equals, customToken)

	roles, _, err = s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(types.RoleNode), Equals, true)
	c.Assert(roles.Include(types.RoleProxy), Equals, false)

	err = s.a.DeleteToken(ctx, customToken)
	c.Assert(err, IsNil)

	// generate multi-use token with long TTL:
	multiUseToken, err := s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleProxy}, TTL: time.Hour})
	c.Assert(err, IsNil)
	_, _, err = s.a.ValidateToken(multiUseToken)
	c.Assert(err, IsNil)

	// use it twice:
	keys, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:                multiUseToken,
		HostID:               "once",
		NodeName:             "node-name",
		Role:                 types.RoleProxy,
		AdditionalPrincipals: []string{"example.com"},
	})
	c.Assert(err, IsNil)

	// along the way, make sure that additional principals work
	hostCert, err := sshutils.ParseCertificate(keys.Cert)
	c.Assert(err, IsNil)
	comment := Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(apiutils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), Equals, true, comment)

	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "twice",
		NodeName: "node-name",
		Role:     types.RoleProxy,
	})
	c.Assert(err, IsNil)

	// try to use after TTL:
	s.a.SetClock(clockwork.NewFakeClockAt(time.Now().UTC().Add(time.Hour + 1)))
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "late.bird",
		NodeName: "node-name",
		Role:     types.RoleProxy,
	})
	c.Assert(err, ErrorMatches, `"node-name" \[late.bird\] can not join the cluster with role Proxy, the token is not valid`)

	// expired token should be gone now
	err = s.a.DeleteToken(ctx, multiUseToken)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	// lets use static tokens now
	roles = types.SystemRoles{types.RoleProxy}
	st, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token:   "static-token-value",
			Roles:   roles,
			Expires: time.Unix(0, 0).UTC(),
		}},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(st)
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "static.host",
		NodeName: "node-name",
		Role:     types.RoleProxy,
	})
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "wrong.role",
		NodeName: "node-name",
		Role:     types.RoleAuth,
	})
	c.Assert(err, NotNil)
	r, _, err := s.a.ValidateToken("static-token-value")
	c.Assert(err, IsNil)
	c.Assert(r, DeepEquals, roles)

	// List tokens (should see 2: one static, one regular)
	tokens, err = s.a.GetTokens(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 2)
}

func (s *AuthSuite) TestBadTokens(c *C) {
	ctx := context.Background()
	// empty
	_, _, err := s.a.ValidateToken("")
	c.Assert(err, NotNil)

	// garbage
	_, _, err = s.a.ValidateToken("bla bla")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleAuth}})
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	_, _, err = s.a.ValidateToken(tampered)
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestGenerateTokenEventsEmitted(c *C) {
	ctx := context.Background()
	// test trusted cluster token emit
	_, err := s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleTrustedCluster}})
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.TrustedClusterTokenCreateEvent)
	s.mockEmitter.Reset()

	// test emit with multiple roles
	_, err = s.a.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{
		types.RoleNode,
		types.RoleTrustedCluster,
		types.RoleAuth,
	}})
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.TrustedClusterTokenCreateEvent)
}

func (s *AuthSuite) TestValidateACRValues(c *C) {
	tests := []struct {
		inIDToken     string
		inACRValue    string
		inACRProvider string
		outIsValid    bool
	}{
		// 0 - default, acr values match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo",
			"",
			true,
		},
		// 1 - default, acr values do not match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"bar",
			"",
			false,
		},
		// 2 - netiq, acr values match
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			true,
		},
		// 3 - netiq, invalid format
		{
			`
{
    "acr": {
        "values": "foo/bar/baz"
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
		// 4 - netiq, invalid value
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz/qux"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
	}

	for i, tt := range tests {
		comment := Commentf("Test %v", i)

		var claims jose.Claims
		err := json.Unmarshal([]byte(tt.inIDToken), &claims)
		c.Assert(err, IsNil, comment)

		err = s.a.validateACRValues(tt.inACRValue, tt.inACRProvider, claims)
		if tt.outIsValid {
			c.Assert(err, IsNil, comment)
		} else {
			c.Assert(err, NotNil, comment)
		}
	}
}

func (s *AuthSuite) TestUpdateConfig(c *C) {
	cn, err := s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, s.clusterName.GetClusterName())
	st, err := s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, []types.ProvisionToken{})

	// try and set cluster name, this should fail because you can only set the
	// cluster name once
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "foo.localhost",
	})
	c.Assert(err, IsNil)
	// use same backend but start a new auth server with different config.
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	authServer, err := NewServer(authConfig)
	c.Assert(err, IsNil)

	err = authServer.SetClusterName(clusterName)
	c.Assert(err, NotNil)
	// try and set static tokens, this should be successful because the last
	// one to upsert tokens wins
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token: "bar",
			Roles: types.SystemRoles{types.SystemRole("baz")},
		}},
	})
	c.Assert(err, IsNil)
	err = authServer.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	// check first auth server and make sure it returns the correct values
	// (original cluster name, new static tokens)
	cn, err = s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, s.clusterName.GetClusterName())
	st, err = s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, types.ProvisionTokensFromV1([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))

	// check second auth server and make sure it also has the correct values
	// new static tokens
	st, err = authServer.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, types.ProvisionTokensFromV1([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))
}

func (s *AuthSuite) TestCreateAndUpdateUserEventsEmitted(c *C) {
	user, err := types.NewUser("some-user")
	c.Assert(err, IsNil)

	ctx := context.Background()

	// test create uesr, happy path
	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})
	err = s.a.CreateUser(ctx, user)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.UserCreateEvent)
	c.Assert(s.mockEmitter.LastEvent().(*apievents.UserCreate).User, Equals, "some-auth-user")
	s.mockEmitter.Reset()

	// test create user with existing user
	err = s.a.CreateUser(ctx, user)
	c.Assert(trace.IsAlreadyExists(err), Equals, true)
	c.Assert(s.mockEmitter.LastEvent(), IsNil)

	// test createdBy gets set to default
	user2, err := types.NewUser("some-other-user")
	c.Assert(err, IsNil)
	err = s.a.CreateUser(ctx, user2)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().(*apievents.UserCreate).User, Equals, teleport.UserSystem)
	s.mockEmitter.Reset()

	// test update on non-existent user
	user3, err := types.NewUser("non-existent-user")
	c.Assert(err, IsNil)
	err = s.a.UpdateUser(ctx, user3)
	c.Assert(trace.IsNotFound(err), Equals, true)
	c.Assert(s.mockEmitter.LastEvent(), IsNil)

	// test update user
	err = s.a.UpdateUser(ctx, user)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.UserUpdatedEvent)
	c.Assert(s.mockEmitter.LastEvent().(*apievents.UserCreate).User, Equals, teleport.UserSystem)
}

func (s *AuthSuite) TestTrustedClusterCRUDEventEmitted(c *C) {
	ctx := context.Background()
	s.a.emitter = s.mockEmitter

	// set up existing cluster to bypass switch cases that
	// makes a network request when creating new clusters
	tc, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"a"},
		ReverseTunnelAddress: "b",
	})
	c.Assert(err, IsNil)
	_, err = s.a.Presence.UpsertTrustedCluster(ctx, tc)
	c.Assert(err, IsNil)

	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(types.UserCA, "test")), IsNil)
	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(types.HostCA, "test")), IsNil)

	err = s.a.createReverseTunnel(tc)
	c.Assert(err, IsNil)

	// test create event for switch case: when tc exists but enabled is false
	tc.SetEnabled(false)

	_, err = s.a.UpsertTrustedCluster(ctx, tc)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.TrustedClusterCreateEvent)
	s.mockEmitter.Reset()

	// test create event for switch case: when tc exists but enabled is true
	tc.SetEnabled(true)

	_, err = s.a.UpsertTrustedCluster(ctx, tc)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.TrustedClusterCreateEvent)
	s.mockEmitter.Reset()

	// test delete event
	err = s.a.DeleteTrustedCluster(ctx, "test")
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.TrustedClusterDeleteEvent)
}

func (s *AuthSuite) TestGithubConnectorCRUDEventsEmitted(c *C) {
	ctx := context.Background()
	// test github create event
	github, err := types.NewGithubConnector("test", types.GithubConnectorSpecV3{})
	c.Assert(err, IsNil)
	err = s.a.upsertGithubConnector(ctx, github)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.GithubConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test github update event
	err = s.a.upsertGithubConnector(ctx, github)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.GithubConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test github delete event
	err = s.a.deleteGithubConnector(ctx, "test")
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.GithubConnectorDeletedEvent)
}

func (s *AuthSuite) TestOIDCConnectorCRUDEventsEmitted(c *C) {
	ctx := context.Background()
	// test oidc create event
	oidc, err := types.NewOIDCConnector("test", types.OIDCConnectorSpecV2{ClientID: "a"})
	c.Assert(err, IsNil)
	err = s.a.UpsertOIDCConnector(ctx, oidc)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.OIDCConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test oidc update event
	err = s.a.UpsertOIDCConnector(ctx, oidc)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.OIDCConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test oidc delete event
	err = s.a.DeleteOIDCConnector(ctx, "test")
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.OIDCConnectorDeletedEvent)
}

func (s *AuthSuite) TestSAMLConnectorCRUDEventsEmitted(c *C) {
	ctx := context.Background()
	// generate a certificate that makes ParseCertificatePEM happy, copied from ca_test.go
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	c.Assert(err, IsNil)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	c.Assert(err, IsNil)

	testClock := clockwork.NewFakeClock()
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     testClock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  testClock.Now().Add(time.Hour),
	})
	c.Assert(err, IsNil)

	// test saml create
	saml, err := types.NewSAMLConnector("test", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "a",
		Issuer:                   "b",
		SSO:                      "c",
		Cert:                     string(certBytes),
	})
	c.Assert(err, IsNil)

	err = s.a.UpsertSAMLConnector(ctx, saml)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.SAMLConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test saml update event
	err = s.a.UpsertSAMLConnector(ctx, saml)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.SAMLConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test saml delete event
	err = s.a.DeleteSAMLConnector(ctx, "test")
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.SAMLConnectorDeletedEvent)
}

func TestU2FSignChallengeCompat(t *testing.T) {
	// Test that the new U2F challenge encoding format is backwards-compatible
	// with older clients and servers.
	//
	// New format is U2FAuthenticateChallenge as JSON.
	// Old format was u2f.AuthenticateChallenge as JSON.
	t.Run("old client, new server", func(t *testing.T) {
		newChallenge := &MFAAuthenticateChallenge{
			AuthenticateChallenge: &u2f.AuthenticateChallenge{
				Challenge: "c1",
			},
			U2FChallenges: []u2f.AuthenticateChallenge{
				{Challenge: "c1"},
				{Challenge: "c2"},
				{Challenge: "c3"},
			},
		}
		wire, err := json.Marshal(newChallenge)
		require.NoError(t, err)

		var oldChallenge u2f.AuthenticateChallenge
		err = json.Unmarshal(wire, &oldChallenge)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(oldChallenge, *newChallenge.AuthenticateChallenge))
	})
	t.Run("new client, old server", func(t *testing.T) {
		oldChallenge := &u2f.AuthenticateChallenge{
			Challenge: "c1",
		}
		wire, err := json.Marshal(oldChallenge)
		require.NoError(t, err)

		var newChallenge MFAAuthenticateChallenge
		err = json.Unmarshal(wire, &newChallenge)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(newChallenge, MFAAuthenticateChallenge{AuthenticateChallenge: oldChallenge}))
	})
}

func TestEmitSSOLoginFailureEvent(t *testing.T) {
	mockE := &events.MockEmitter{}

	emitSSOLoginFailureEvent(context.Background(), mockE, "test", trace.BadParameter("some error"))

	require.Equal(t, mockE.LastEvent(), &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserSSOLoginFailureCode,
		},
		Method: "test",
		Status: apievents.Status{
			Success:     false,
			Error:       "some error",
			UserMessage: "some error",
		},
	})
}

func (s *AuthSuite) TestNewWebSession(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(types.UserCA, "me.localhost")), IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(types.HostCA, "me.localhost")), IsNil)

	// Set a web idle timeout.
	duration := time.Duration(5) * time.Minute
	cfg := types.DefaultClusterNetworkingConfig()
	cfg.SetWebIdleTimeout(duration)
	s.a.SetClusterNetworkingConfig(context.Background(), cfg)

	// Create a user.
	user, _, err := CreateUserAndRole(s.a, "test-user", []string{"test-role"})
	c.Assert(err, IsNil)

	// Create a new web session.
	req := types.NewWebSessionRequest{
		User:       user.GetName(),
		Roles:      user.GetRoles(),
		Traits:     user.GetTraits(),
		LoginTime:  s.a.clock.Now().UTC(),
		SessionTTL: apidefaults.CertDuration,
	}
	bearerTokenTTL := utils.MinTTL(req.SessionTTL, BearerTokenTTL)

	ws, err := s.a.NewWebSession(req)
	c.Assert(err, IsNil)
	c.Assert(user.GetName(), Equals, ws.GetUser())
	c.Assert(duration, Equals, ws.GetIdleTimeout())
	c.Assert(req.LoginTime, Equals, ws.GetLoginTime())
	c.Assert(req.LoginTime.UTC().Add(req.SessionTTL), Equals, ws.GetExpiryTime())
	c.Assert(req.LoginTime.UTC().Add(bearerTokenTTL), Equals, ws.GetBearerTokenExpiryTime())
	c.Assert(ws.GetBearerToken(), Not(Equals), "")
	c.Assert(ws.GetPriv(), NotNil)
	c.Assert(ws.GetPub(), NotNil)
	c.Assert(ws.GetTLSCert(), NotNil)
}

func TestGenerateUserCertWithLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, role, err := CreateUserAndRole(p.a, "test-user", []string{})
	require.NoError(t, err)
	mfaID := uuid.New()
	keygen := testauthority.New()
	_, pub, err := keygen.GetNewKeyPairFromPool()
	require.NoError(t, err)
	certReq := certRequest{
		user:        user,
		checker:     services.NewRoleSet(role),
		mfaVerified: mfaID,
		publicKey:   pub,
	}
	_, err = p.a.generateUserCert(certReq)
	require.NoError(t, err)

	testTargets := append(
		[]types.LockTarget{{User: user.GetName()}, {MFADevice: mfaID}},
		services.RolesToLockTargets(user.GetRoles())...,
	)
	for _, target := range testTargets {
		t.Run(fmt.Sprintf("lock targeting %v", target), func(t *testing.T) {
			lockWatch, err := p.a.lockWatcher.Subscribe(ctx, target)
			require.NoError(t, err)
			defer lockWatch.Close()
			lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: target})
			require.NoError(t, err)

			require.NoError(t, p.a.UpsertLock(ctx, lock))
			select {
			case event := <-lockWatch.Events():
				require.Equal(t, types.OpPut, event.Type)
				require.Empty(t, resourceDiff(event.Resource, lock))
			case <-lockWatch.Done():
				t.Fatal("Watcher has unexpectedly exited.")
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for lock update.")
			}
			_, err = p.a.generateUserCert(certReq)
			require.Error(t, err)
			require.EqualError(t, err, services.LockInForceAccessDenied(lock).Error())
		})
	}
}

func TestGenerateHostCertWithLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	hostID := uuid.New()
	keygen := testauthority.New()
	_, pub, err := keygen.GetNewKeyPairFromPool()
	require.NoError(t, err)
	_, err = p.a.GenerateHostCert(pub, hostID, "test-node", []string{}, p.clusterName.GetClusterName(), types.SystemRoles{types.RoleNode}, time.Minute)
	require.NoError(t, err)

	target := types.LockTarget{Node: hostID}
	lockWatch, err := p.a.lockWatcher.Subscribe(ctx, target)
	require.NoError(t, err)
	defer lockWatch.Close()
	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: target})
	require.NoError(t, err)

	require.NoError(t, p.a.UpsertLock(ctx, lock))
	select {
	case event := <-lockWatch.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, resourceDiff(event.Resource, lock))
	case <-lockWatch.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for lock update.")
	}
	_, err = p.a.GenerateHostCert(pub, hostID, "test-node", []string{}, p.clusterName.GetClusterName(), types.SystemRoles{types.RoleNode}, time.Minute)
	require.Error(t, err)
	require.EqualError(t, err, services.LockInForceAccessDenied(lock).Error())

	// Locks targeting nodes should not apply to other system roles.
	_, err = p.a.GenerateHostCert(pub, hostID, "test-proxy", []string{}, p.clusterName.GetClusterName(), types.SystemRoles{types.RoleProxy}, time.Minute)
	require.NoError(t, err)
}

func newTestServices(t *testing.T) Services {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	configService, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)

	return Services{
		Trust:                local.NewCAService(bk),
		Presence:             local.NewPresenceService(bk),
		Provisioner:          local.NewProvisioningService(bk),
		Identity:             local.NewIdentityService(bk),
		Access:               local.NewAccessService(bk),
		DynamicAccessExt:     local.NewDynamicAccessService(bk),
		ClusterConfiguration: configService,
		Events:               local.NewEventsService(bk, configService.GetClusterConfig),
		IAuditLog:            events.NewDiscardAuditLog(),
	}
}
