/*
Copyright 2015-2022 Gravitational, Inc.

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
	"errors"
	"fmt"
	mathrand "math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/google/go-cmp/cmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/coreos/go-oidc/jose"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type testPack struct {
	bk          backend.Backend
	clusterName types.ClusterName
	a           *Server
	mockEmitter *eventstest.MockEmitter
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
		Authority:              testauthority.New(),
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

	if err := p.a.UpsertCertAuthority(suite.NewTestCA(types.UserCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.UpsertCertAuthority(suite.NewTestCA(types.HostCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}

	if err := p.a.UpsertNamespace(types.DefaultNamespace()); err != nil {
		return p, trace.Wrap(err)
	}

	p.mockEmitter = &eventstest.MockEmitter{}
	p.a.emitter = p.mockEmitter
	return p, nil
}

func newAuthSuite(t *testing.T) *testPack {
	s, err := newTestPack(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		if s.bk != nil {
			s.bk.Close()
		}
	})

	return &s
}

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSessions(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()

	user := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	require.Error(t, err)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	require.NoError(t, err)

	err = s.a.UpsertPassword(user, pass)
	require.NoError(t, err)

	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	require.NoError(t, err)
	require.NotNil(t, ws)

	out, err := s.a.GetWebSessionInfo(ctx, user, ws.GetName())
	require.NoError(t, err)
	ws.SetPriv(nil)
	require.Equal(t, ws, out)

	err = s.a.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
		User:      user,
		SessionID: ws.GetName(),
	})
	require.NoError(t, err)

	_, err = s.a.GetWebSession(ctx, types.GetWebSessionRequest{
		User:      user,
		SessionID: ws.GetName(),
	})
	require.True(t, trace.IsNotFound(err), "%#v", err)
}

func TestAuthenticateSSHUser(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()

	// Register the leaf cluster.
	leaf, err := types.NewRemoteCluster("leaf.localhost")
	require.NoError(t, err)
	require.NoError(t, s.a.CreateRemoteCluster(leaf))

	user := "user1"
	pass := []byte("abc123")

	// Try to login as an unknown user.
	_, err = s.a.AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: pass},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Create the user.
	_, role, err := CreateUserAndRole(s.a, user, []string{user})
	require.NoError(t, err)
	err = s.a.UpsertPassword(user, pass)
	require.NoError(t, err)
	// Give the role some k8s principals too.
	role.SetKubeUsers(types.Allow, []string{user})
	role.SetKubeGroups(types.Allow, []string{"system:masters"})
	err = s.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	kg := testauthority.New()
	_, pub, err := kg.GetNewKeyPairFromPool()
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	// Verify the public key and principals in SSH cert.
	inSSHPub, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)
	gotSSHCert, err := sshutils.ParseCertificate(resp.Cert)
	require.NoError(t, err)
	require.Equal(t, gotSSHCert.Key, inSSHPub)
	require.Equal(t, gotSSHCert.ValidPrincipals, []string{user, teleport.SSHSessionJoinPrincipal})
	// Verify the public key and Subject in TLS cert.
	inCryptoPub := inSSHPub.(ssh.CryptoPublicKey).CryptoPublicKey()
	gotTLSCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	require.Equal(t, gotTLSCert.PublicKey, inCryptoPub)
	wantID := tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		Expires:          gotTLSCert.NotAfter,
		RouteToCluster:   s.clusterName.GetClusterName(),
		TeleportCluster:  s.clusterName.GetClusterName(),
	}
	gotID, err := tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

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
	require.NoError(t, err)
	// Verify the TLS cert has the correct RouteToCluster set.
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
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
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

	// Register a kubernetes cluster to verify the defaulting logic in TLS cert
	// generation.
	_, err = s.a.UpsertKubeServiceV2(ctx, &types.ServerV2{
		Metadata: types.Metadata{Name: "kube-service"},
		Kind:     types.KindKubeService,
		Version:  types.V2,
		Spec: types.ServerSpecV2{
			KubernetesClusters: []*types.KubernetesCluster{{Name: "root-kube-cluster"}},
		},
	})
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

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
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

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
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

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
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, *gotID, wantID)

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
	require.Error(t, err)
}

func TestUserLock(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	username := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	require.Error(t, err)

	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	require.NoError(t, err)

	err = s.a.UpsertPassword(username, pass)
	require.NoError(t, err)

	// successful log in
	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	require.NoError(t, err)
	require.NotNil(t, ws)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	for i := 0; i <= defaults.MaxLoginAttempts; i++ {
		_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
			Username: username,
			Pass:     &PassCreds{Password: []byte("wrong pass")},
		})
		require.Error(t, err)
	}

	user, err := s.a.Identity.GetUser(username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: username,
		Pass:     &PassCreds{Password: pass},
	})
	require.NoError(t, err)
}

func TestTokensCRUD(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()

	// before we do anything, we should have 0 tokens
	btokens, err := s.a.GetTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, btokens, 0)

	// generate persistent token
	tokenName, err := s.a.GenerateToken(ctx, &proto.GenerateTokenRequest{Roles: types.SystemRoles{types.RoleNode}})
	require.NoError(t, err)
	require.Len(t, tokenName, 2*TokenLenBytes)
	tokens, err := s.a.GetTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, tokens[0].GetName(), tokenName)

	tokenResource, err := s.a.ValidateToken(ctx, tokenName)
	require.NoError(t, err)
	roles := tokenResource.GetRoles()
	require.True(t, roles.Include(types.RoleNode))
	require.False(t, roles.Include(types.RoleProxy))

	// generate predefined token
	customToken := "custom-token"
	tokenName, err = s.a.GenerateToken(ctx, &proto.GenerateTokenRequest{Roles: types.SystemRoles{types.RoleNode}, Token: customToken})
	require.NoError(t, err)
	require.Equal(t, tokenName, customToken)

	tokenResource, err = s.a.ValidateToken(ctx, tokenName)
	require.NoError(t, err)
	roles = tokenResource.GetRoles()
	require.True(t, roles.Include(types.RoleNode))
	require.False(t, roles.Include(types.RoleProxy))

	err = s.a.DeleteToken(ctx, customToken)
	require.NoError(t, err)

	// lets use static tokens now
	roles = types.SystemRoles{types.RoleProxy}
	st, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token:   "static-token-value",
			Roles:   roles,
			Expires: time.Unix(0, 0).UTC(),
		}},
	})
	require.NoError(t, err)

	err = s.a.SetStaticTokens(st)
	require.NoError(t, err)

	tokenResource, err = s.a.ValidateToken(ctx, "static-token-value")
	require.NoError(t, err)
	fetchesRoles := tokenResource.GetRoles()
	require.Equal(t, fetchesRoles, roles)

	// List tokens (should see 2: one static, one regular)
	tokens, err = s.a.GetTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 2)
}

func TestBadTokens(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// empty
	_, err := s.a.ValidateToken(ctx, "")
	require.Error(t, err)

	// garbage
	_, err = s.a.ValidateToken(ctx, "bla bla")
	require.Error(t, err)

	// tampered
	tok, err := s.a.GenerateToken(ctx, &proto.GenerateTokenRequest{Roles: types.SystemRoles{types.RoleAuth}})
	require.NoError(t, err)

	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(ctx, tampered)
	require.Error(t, err)
}

// TestLocalControlStream verifies that local control stream behaves as expected.
func TestLocalControlStream(t *testing.T) {
	const serverID = "test-server"

	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newAuthSuite(t)

	stream := s.a.MakeLocalInventoryControlStream()
	defer stream.Close()

	err := stream.Send(ctx, proto.UpstreamInventoryHello{
		ServerID: serverID,
	})
	require.NoError(t, err)

	select {
	case msg := <-stream.Recv():
		_, ok := msg.(proto.DownstreamInventoryHello)
		require.True(t, ok)
	case <-stream.Done():
		t.Fatalf("stream closed unexpectedly: %v", stream.Error())
	case <-time.After(time.Second * 10):
		t.Fatal("timeout waiting for downstream hello")
	}

	// wait for control stream to get inserted into the controller (happens after
	// hello exchange is finished).
	require.Eventually(t, func() bool {
		_, ok := s.a.inventory.GetControlStream(serverID)
		return ok
	}, time.Second*5, time.Millisecond*200)

	// try performing a normal operation against the control stream to double-check that it is healthy
	go s.a.PingInventory(ctx, proto.InventoryPingRequest{
		ServerID: serverID,
	})

	select {
	case msg := <-stream.Recv():
		_, ok := msg.(proto.DownstreamInventoryPing)
		require.True(t, ok)
	case <-stream.Done():
		t.Fatalf("stream closed unexpectedly: %v", stream.Error())
	case <-time.After(time.Second * 10):
		t.Fatal("timeout waiting for downstream hello")
	}
}

func TestGenerateTokenEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// test trusted cluster token emit
	_, err := s.a.GenerateToken(ctx, &proto.GenerateTokenRequest{Roles: types.SystemRoles{types.RoleTrustedCluster}})
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.TrustedClusterTokenCreateEvent)
	s.mockEmitter.Reset()

	// test emit with multiple roles
	_, err = s.a.GenerateToken(ctx, &proto.GenerateTokenRequest{Roles: types.SystemRoles{
		types.RoleNode,
		types.RoleTrustedCluster,
		types.RoleAuth,
	}})
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.TrustedClusterTokenCreateEvent)
}

func TestValidateACRValues(t *testing.T) {
	s := newAuthSuite(t)

	tests := []struct {
		comment       string
		inIDToken     string
		inACRValue    string
		inACRProvider string
		outIsValid    require.ErrorAssertionFunc
	}{
		{
			"0 - default, acr values match",
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo",
			"",
			require.NoError,
		},
		{
			"1 - default, acr values do not match",
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"bar",
			"",
			require.Error,
		},
		{
			"2 - netiq, acr values match",
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
			require.NoError,
		},
		{
			"3 - netiq, invalid format",
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
			require.Error,
		},
		{
			"4 - netiq, invalid value",
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
			require.Error,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.comment, func(t *testing.T) {
			t.Parallel()
			var claims jose.Claims
			err := json.Unmarshal([]byte(tt.inIDToken), &claims)
			require.NoError(t, err)

			err = s.a.validateACRValues(tt.inACRValue, tt.inACRProvider, claims)
			tt.outIsValid(t, err)
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	cn, err := s.a.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, cn.GetClusterName(), s.clusterName.GetClusterName())
	st, err := s.a.GetStaticTokens()
	require.NoError(t, err)
	require.Equal(t, st.GetStaticTokens(), []types.ProvisionToken{})

	// try and set cluster name, this should fail because you can only set the
	// cluster name once
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "foo.localhost",
	})
	require.NoError(t, err)
	// use same backend but start a new auth server with different config.
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              testauthority.New(),
		SkipPeriodicOperations: true,
	}
	authServer, err := NewServer(authConfig)
	require.NoError(t, err)

	err = authServer.SetClusterName(clusterName)
	require.Error(t, err)
	// try and set static tokens, this should be successful because the last
	// one to upsert tokens wins
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token: "bar",
			Roles: types.SystemRoles{types.SystemRole("baz")},
		}},
	})
	require.NoError(t, err)
	err = authServer.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	// check first auth server and make sure it returns the correct values
	// (original cluster name, new static tokens)
	cn, err = s.a.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, cn.GetClusterName(), s.clusterName.GetClusterName())
	st, err = s.a.GetStaticTokens()
	require.NoError(t, err)
	require.Equal(t, st.GetStaticTokens(), types.ProvisionTokensFromV1([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))

	// check second auth server and make sure it also has the correct values
	// new static tokens
	st, err = authServer.GetStaticTokens()
	require.NoError(t, err)
	require.Equal(t, st.GetStaticTokens(), types.ProvisionTokensFromV1([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))
}

func TestCreateAndUpdateUserEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	user, err := types.NewUser("some-user")
	require.NoError(t, err)

	ctx := context.Background()

	// test create user, happy path
	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})
	err = s.a.CreateUser(ctx, user)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.UserCreateEvent)
	require.Equal(t, s.mockEmitter.LastEvent().(*apievents.UserCreate).User, "some-auth-user")
	s.mockEmitter.Reset()

	// test create user with existing user
	err = s.a.CreateUser(ctx, user)
	require.True(t, trace.IsAlreadyExists(err))
	require.Nil(t, s.mockEmitter.LastEvent())

	// test createdBy gets set to default
	user2, err := types.NewUser("some-other-user")
	require.NoError(t, err)
	err = s.a.CreateUser(ctx, user2)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().(*apievents.UserCreate).User, teleport.UserSystem)
	s.mockEmitter.Reset()

	// test update on non-existent user
	user3, err := types.NewUser("non-existent-user")
	require.NoError(t, err)
	err = s.a.UpdateUser(ctx, user3)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, s.mockEmitter.LastEvent())

	// test update user
	err = s.a.UpdateUser(ctx, user)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.UserUpdatedEvent)
	require.Equal(t, s.mockEmitter.LastEvent().(*apievents.UserCreate).User, teleport.UserSystem)
}

func TestTrustedClusterCRUDEventEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	s.a.emitter = s.mockEmitter

	// set up existing cluster to bypass switch cases that
	// makes a network request when creating new clusters
	tc, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"a"},
		ReverseTunnelAddress: "b",
	})
	require.NoError(t, err)
	_, err = s.a.Presence.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)

	require.NoError(t, s.a.UpsertCertAuthority(suite.NewTestCA(types.UserCA, "test")))
	require.NoError(t, s.a.UpsertCertAuthority(suite.NewTestCA(types.HostCA, "test")))

	err = s.a.createReverseTunnel(tc)
	require.NoError(t, err)

	// test create event for switch case: when tc exists but enabled is false
	tc.SetEnabled(false)

	_, err = s.a.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.TrustedClusterCreateEvent)
	s.mockEmitter.Reset()

	// test create event for switch case: when tc exists but enabled is true
	tc.SetEnabled(true)

	_, err = s.a.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.TrustedClusterCreateEvent)
	s.mockEmitter.Reset()

	// test delete event
	err = s.a.DeleteTrustedCluster(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.TrustedClusterDeleteEvent)
}

func TestGithubConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// test github create event
	github, err := types.NewGithubConnector("test", types.GithubConnectorSpecV3{
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "octocats",
				Team:         "dummy",
				Logins:       []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	err = s.a.upsertGithubConnector(ctx, github)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.GithubConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test github update event
	err = s.a.upsertGithubConnector(ctx, github)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.GithubConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test github delete event
	err = s.a.deleteGithubConnector(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.GithubConnectorDeletedEvent)
}

func TestOIDCConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// test oidc create event
	oidc, err := types.NewOIDCConnector("test", types.OIDCConnectorSpecV3{
		ClientID: "a",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "dummy",
				Value: "dummy",
				Roles: []string{"dummy"},
			},
		},
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)
	err = s.a.UpsertOIDCConnector(ctx, oidc)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.OIDCConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test oidc update event
	err = s.a.UpsertOIDCConnector(ctx, oidc)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.OIDCConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test oidc delete event
	err = s.a.DeleteOIDCConnector(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.OIDCConnectorDeletedEvent)
}

func TestSAMLConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// generate a certificate that makes ParseCertificatePEM happy, copied from ca_test.go
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	testClock := clockwork.NewFakeClock()
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     testClock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  testClock.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	// test saml create
	saml, err := types.NewSAMLConnector("test", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "a",
		Issuer:                   "b",
		SSO:                      "c",
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "dummy",
				Value: "dummy",
				Roles: []string{"dummy"},
			},
		},
		Cert: string(certBytes),
	})
	require.NoError(t, err)

	err = s.a.UpsertSAMLConnector(ctx, saml)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.SAMLConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test saml update event
	err = s.a.UpsertSAMLConnector(ctx, saml)
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.SAMLConnectorCreatedEvent)
	s.mockEmitter.Reset()

	// test saml delete event
	err = s.a.DeleteSAMLConnector(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, s.mockEmitter.LastEvent().GetType(), events.SAMLConnectorDeletedEvent)
}

func TestEmitSSOLoginFailureEvent(t *testing.T) {
	mockE := &eventstest.MockEmitter{}

	emitSSOLoginFailureEvent(context.Background(), mockE, "test", trace.BadParameter("some error"), false)

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

	emitSSOLoginFailureEvent(context.Background(), mockE, "test", trace.BadParameter("some error"), true)

	require.Equal(t, mockE.LastEvent(), &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserSSOTestFlowLoginFailureCode,
		},
		Method: "test",
		Status: apievents.Status{
			Success:     false,
			Error:       "some error",
			UserMessage: "some error",
		},
	})
}

func TestGenerateUserCertWithCertExtension(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, role, err := CreateUserAndRole(p.a, "test-user", []string{})
	require.NoError(t, err)

	extension := types.CertExtension{
		Name:  "abc",
		Value: "cde",
		Type:  types.CertExtensionType_SSH,
		Mode:  types.CertExtensionMode_EXTENSION,
	}
	options := role.GetOptions()
	options.CertExtensions = []*types.CertExtension{&extension}
	role.SetOptions(options)
	err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	accessInfo, err := services.AccessInfoFromUser(user, p.a)
	require.NoError(t, err)

	keygen := testauthority.New()
	_, pub, err := keygen.GetNewKeyPairFromPool()
	require.NoError(t, err)
	certReq := certRequest{
		user:      user,
		checker:   services.NewAccessChecker(accessInfo, p.clusterName.GetClusterName()),
		publicKey: pub,
	}
	certs, err := p.a.generateUserCert(certReq)
	require.NoError(t, err)

	key, err := sshutils.ParseCertificate(certs.SSH)
	require.NoError(t, err)

	val, ok := key.Extensions[extension.Name]
	require.True(t, ok)
	require.Equal(t, extension.Value, val)
}

func TestGenerateUserCertWithLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(p.a, "test-user", []string{})
	require.NoError(t, err)
	accessInfo, err := services.AccessInfoFromUser(user, p.a)
	require.NoError(t, err)
	mfaID := "test-mfa-id"
	requestID := "test-access-request"
	keygen := testauthority.New()
	_, pub, err := keygen.GetNewKeyPairFromPool()
	require.NoError(t, err)
	certReq := certRequest{
		user:           user,
		checker:        services.NewAccessChecker(accessInfo, p.clusterName.GetClusterName()),
		mfaVerified:    mfaID,
		publicKey:      pub,
		activeRequests: services.RequestIDs{AccessRequests: []string{requestID}},
	}
	_, err = p.a.generateUserCert(certReq)
	require.NoError(t, err)

	testTargets := append(
		[]types.LockTarget{{User: user.GetName()}, {MFADevice: mfaID}, {AccessRequest: requestID}},
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

	hostID := uuid.New().String()
	keygen := testauthority.New()
	_, pub, err := keygen.GetNewKeyPairFromPool()
	require.NoError(t, err)
	_, err = p.a.GenerateHostCert(pub, hostID, "test-node", []string{},
		p.clusterName.GetClusterName(), types.RoleNode, time.Minute)
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
	_, err = p.a.GenerateHostCert(pub, hostID, "test-node", []string{}, p.clusterName.GetClusterName(), types.RoleNode, time.Minute)
	require.Error(t, err)
	require.EqualError(t, err, services.LockInForceAccessDenied(lock).Error())

	// Locks targeting nodes should not apply to other system roles.
	_, err = p.a.GenerateHostCert(pub, hostID, "test-proxy", []string{}, p.clusterName.GetClusterName(), types.RoleProxy, time.Minute)
	require.NoError(t, err)
}

func TestNewWebSession(t *testing.T) {
	t.Parallel()
	p, err := newTestPack(context.Background(), t.TempDir())
	require.NoError(t, err)

	// Set a web idle timeout.
	duration := time.Duration(5) * time.Minute
	cfg := types.DefaultClusterNetworkingConfig()
	cfg.SetWebIdleTimeout(duration)
	err = p.a.SetClusterNetworkingConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Create a user.
	user, _, err := CreateUserAndRole(p.a, "test-user", []string{"test-role"})
	require.NoError(t, err)

	// Create a new web session.
	req := types.NewWebSessionRequest{
		User:       user.GetName(),
		Roles:      user.GetRoles(),
		Traits:     user.GetTraits(),
		LoginTime:  p.a.clock.Now().UTC(),
		SessionTTL: apidefaults.CertDuration,
	}
	bearerTokenTTL := utils.MinTTL(req.SessionTTL, BearerTokenTTL)

	ws, err := p.a.NewWebSession(req)
	require.NoError(t, err)
	require.Equal(t, user.GetName(), ws.GetUser())
	require.Equal(t, duration, ws.GetIdleTimeout())
	require.Equal(t, req.LoginTime, ws.GetLoginTime())
	require.Equal(t, req.LoginTime.UTC().Add(req.SessionTTL), ws.GetExpiryTime())
	require.Equal(t, req.LoginTime.UTC().Add(bearerTokenTTL), ws.GetBearerTokenExpiryTime())
	require.NotEmpty(t, ws.GetBearerToken())
	require.NotEmpty(t, ws.GetPriv())
	require.NotEmpty(t, ws.GetPub())
	require.NotEmpty(t, ws.GetTLSCert())
}

func TestDeleteMFADeviceSync(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &eventstest.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	username := "llama@goteleport.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)

	// Insert dummy devices.
	webDev1, err := RegisterTestDevice(ctx, clt, "web-1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)
	webDev2, err := RegisterTestDevice(ctx, clt, "web-2", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, webDev1)
	require.NoError(t, err)
	totpDev1, err := RegisterTestDevice(ctx, clt, "otp-1", proto.DeviceType_DEVICE_TYPE_TOTP, webDev1, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)
	totpDev2, err := RegisterTestDevice(ctx, clt, "otp-2", proto.DeviceType_DEVICE_TYPE_TOTP, webDev1, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	tests := []struct {
		name           string
		deviceToDelete string
		tokenReq       CreateUserTokenRequest
	}{
		{
			name:           "recovery approved token",
			deviceToDelete: webDev1.MFA.GetName(),
			tokenReq: CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
		},
		{
			name:           "privilege token",
			deviceToDelete: totpDev1.MFA.GetName(),
			tokenReq: CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypePrivilege,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := srv.Auth().newUserToken(tc.tokenReq)
			require.NoError(t, err)
			_, err = srv.Auth().Identity.CreateUserToken(ctx, token)
			require.NoError(t, err)

			// Delete the TOTP device.
			err = srv.Auth().DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
				TokenID:    token.GetName(),
				DeviceName: tc.deviceToDelete,
			})
			require.NoError(t, err)
		})
	}

	// Check it's been deleted.
	devs, err := srv.Auth().Identity.GetMFADevices(ctx, username, false)
	require.NoError(t, err)
	compareDevices(t, false /* ignoreUpdateAndCounter */, devs, webDev2.MFA, totpDev2.MFA)

	// Test last events emitted.
	event := mockEmitter.LastEvent()
	require.Equal(t, events.MFADeviceDeleteEvent, event.GetType())
	require.Equal(t, events.MFADeviceDeleteEventCode, event.GetCode())
	require.Equal(t, event.(*apievents.MFADeviceDelete).UserMetadata.User, username)
}

func TestDeleteMFADeviceSync_WithErrors(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	username := "llama@goteleport.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)

	// Insert a device.
	const devName = "otp"
	_, err = RegisterTestDevice(ctx, clt, devName, proto.DeviceType_DEVICE_TYPE_TOTP, nil /* authenticator */, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	tests := []struct {
		name          string
		deviceName    string
		tokenRequest  *CreateUserTokenRequest
		assertErrType func(error) bool
	}{
		{
			name:          "token not found",
			deviceName:    devName,
			assertErrType: trace.IsAccessDenied,
		},
		{
			name:       "invalid token type",
			deviceName: devName,
			tokenRequest: &CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: "unknown-token-type",
			},
			assertErrType: trace.IsAccessDenied,
		},
		{
			name:       "device not found",
			deviceName: "does-not-exist",
			tokenRequest: &CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
			assertErrType: trace.IsNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := "test-token-not-found"

			if tc.tokenRequest != nil {
				token, err := srv.Auth().newUserToken(*tc.tokenRequest)
				require.NoError(t, err)
				_, err = srv.Auth().Identity.CreateUserToken(context.Background(), token)
				require.NoError(t, err)

				tokenID = token.GetName()
			}

			err = srv.Auth().DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
				TokenID:    tokenID,
				DeviceName: tc.deviceName,
			})
			require.True(t, tc.assertErrType(err))
		})
	}
}

// TestDeleteMFADeviceSync_lastDevice tests for preventing deletion of last
// device when second factor is required.
func TestDeleteMFADeviceSync_lastDevice(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	webConfig := &types.Webauthn{
		RPID: "localhost",
	}

	newTOTPForUser := func(user string) *types.MFADevice {
		clt, err := srv.NewClient(TestUser(user))
		require.NoError(t, err)
		dev, err := RegisterTestDevice(ctx, clt, "otp", proto.DeviceType_DEVICE_TYPE_TOTP, nil /* authenticator */, WithTestDeviceClock(srv.Clock()))
		require.NoError(t, err)
		return dev.MFA
	}

	tests := []struct {
		name              string
		wantErr           bool
		setAuthPreference func()
		createDevice      func(user string) *types.MFADevice
	}{
		{
			name: "with second factor optional",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn:     webConfig,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			createDevice: newTOTPForUser,
		},
		{
			name:    "with second factor otp",
			wantErr: true,
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOTP,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			createDevice: newTOTPForUser,
		},
		{
			name:    "with second factor webauthn",
			wantErr: true,
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn:     webConfig,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			createDevice: func(user string) *types.MFADevice {
				clt, err := srv.NewClient(TestUser(user))
				require.NoError(t, err)
				dev, err := RegisterTestDevice(ctx, clt, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
				require.NoError(t, err)
				return dev.MFA
			},
		},
		{
			name:    "with second factor on",
			wantErr: true,
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOn,
					Webauthn:     webConfig,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			createDevice: newTOTPForUser,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a user with no MFA device.
			username := fmt.Sprintf("llama%v@goteleport.com", mathrand.Int())
			_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
			require.NoError(t, err)

			// Set auth preference.
			tc.setAuthPreference()

			// Insert a MFA device.
			dev := tc.createDevice(username)

			// Acquire an approved token.
			token, err := srv.Auth().newUserToken(CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			})
			require.NoError(t, err)
			_, err = srv.Auth().Identity.CreateUserToken(context.Background(), token)
			require.NoError(t, err)

			// Delete the device.
			err = srv.Auth().DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
				TokenID:    token.GetName(),
				DeviceName: dev.GetName(),
			})

			switch {
			case tc.wantErr:
				require.Error(t, err)
				// Check it hasn't been deleted.
				res, err := srv.Auth().GetMFADevices(ctx, &proto.GetMFADevicesRequest{
					TokenID: token.GetName(),
				})
				require.NoError(t, err)
				require.Len(t, res.GetDevices(), 1)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestAddMFADeviceSync(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &eventstest.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(u.username))
	require.NoError(t, err)

	tests := []struct {
		name       string
		deviceName string
		wantErr    bool
		getReq     func(string) *proto.AddMFADeviceSyncRequest
	}{
		{
			name:    "invalid token type",
			wantErr: true,
			getReq: func(deviceName string) *proto.AddMFADeviceSyncRequest {
				// Obtain a non privilege token.
				token, err := srv.Auth().newUserToken(CreateUserTokenRequest{
					Name: u.username,
					TTL:  5 * time.Minute,
					Type: UserTokenTypeResetPassword,
				})
				require.NoError(t, err)
				_, err = srv.Auth().Identity.CreateUserToken(ctx, token)
				require.NoError(t, err)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:       token.GetName(),
					NewDeviceName: deviceName,
				}
			},
		},
		{
			name:       "TOTP device with privilege token",
			deviceName: "new-totp",
			getReq: func(deviceName string) *proto.AddMFADeviceSyncRequest {
				// Obtain a privilege token.
				privelegeToken, err := srv.Auth().createPrivilegeToken(ctx, u.username, UserTokenTypePrivilege)
				require.NoError(t, err)

				// Create token secrets.
				res, err := srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
					TokenID:    privelegeToken.GetName(),
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				})
				require.NoError(t, err)

				_, totpRegRes, err := NewTestDeviceFromChallenge(res, WithTestDeviceClock(srv.Auth().clock))
				require.NoError(t, err)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:        privelegeToken.GetName(),
					NewDeviceName:  deviceName,
					NewMFAResponse: totpRegRes,
				}
			},
		},
		{
			name:       "Webauthn device with privilege exception token",
			deviceName: "new-webauthn",
			getReq: func(deviceName string) *proto.AddMFADeviceSyncRequest {
				privExToken, err := srv.Auth().createPrivilegeToken(ctx, u.username, UserTokenTypePrivilegeException)
				require.NoError(t, err)

				_, webauthnRes, err := getMockedWebauthnAndRegisterRes(srv.Auth(), privExToken.GetName(), proto.DeviceUsage_DEVICE_USAGE_MFA)
				require.NoError(t, err)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:        privExToken.GetName(),
					NewDeviceName:  deviceName,
					NewMFAResponse: webauthnRes,
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := clt.AddMFADeviceSync(ctx, tc.getReq(tc.deviceName))
			switch {
			case tc.wantErr:
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				require.Equal(t, tc.deviceName, res.GetDevice().GetName())

				// Test events emitted.
				event := mockEmitter.LastEvent()
				require.Equal(t, events.MFADeviceAddEvent, event.GetType())
				require.Equal(t, events.MFADeviceAddEventCode, event.GetCode())
				require.Equal(t, event.(*apievents.MFADeviceAdd).UserMetadata.User, u.username)

				// Check it's been added.
				res, err := clt.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
				require.NoError(t, err)

				found := false
				for _, mfa := range res.GetDevices() {
					if mfa.GetName() == tc.deviceName {
						found = true
						break
					}
				}
				require.True(t, found, "MFA device %q not found", tc.deviceName)
			}
		})
	}
}

func TestGetMFADevices_WithToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	username := "llama@goteleport.com"
	_, _, err = CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)
	webDev, err := RegisterTestDevice(ctx, clt, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)
	totpDev, err := RegisterTestDevice(ctx, clt, "otp", proto.DeviceType_DEVICE_TYPE_TOTP, webDev, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	tests := []struct {
		name         string
		wantErr      bool
		tokenRequest *CreateUserTokenRequest
	}{
		{
			name:    "token not found",
			wantErr: true,
		},
		{
			name:    "invalid token type",
			wantErr: true,
			tokenRequest: &CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypeResetPassword,
			},
		},
		{
			name: "valid token",
			tokenRequest: &CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokenID := "test-token-not-found"

			if tc.tokenRequest != nil {
				token, err := srv.Auth().newUserToken(*tc.tokenRequest)
				require.NoError(t, err)
				_, err = srv.Auth().Identity.CreateUserToken(context.Background(), token)
				require.NoError(t, err)

				tokenID = token.GetName()
			}

			res, err := srv.Auth().GetMFADevices(ctx, &proto.GetMFADevicesRequest{
				TokenID: tokenID,
			})

			switch {
			case tc.wantErr:
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				compareDevices(t, true /* ignoreUpdateAndCounter */, res.GetDevices(), webDev.MFA, totpDev.MFA)
			}
		})
	}
}

func TestGetMFADevices_WithAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	username := "llama@goteleport.com"
	_, _, err = CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)
	webDev, err := RegisterTestDevice(ctx, clt, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)
	totpDev, err := RegisterTestDevice(ctx, clt, "otp", proto.DeviceType_DEVICE_TYPE_TOTP, webDev, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	res, err := clt.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	compareDevices(t, true /* ignoreUpdateAndCounter */, res.GetDevices(), webDev.MFA, totpDev.MFA)
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
		Events:               local.NewEventsService(bk),
		IAuditLog:            events.NewDiscardAuditLog(),
	}
}

func compareDevices(t *testing.T, ignoreUpdateAndCounter bool, got []*types.MFADevice, want ...*types.MFADevice) {
	sort.Slice(got, func(i, j int) bool { return got[i].GetName() < got[j].GetName() })
	sort.Slice(want, func(i, j int) bool { return want[i].GetName() < want[j].GetName() })

	// Remove TOTP keys before comparison.
	for _, w := range want {
		totp := w.GetTotp()
		if totp == nil {
			continue
		}
		if totp.Key == "" {
			continue
		}
		key := totp.Key
		// defer in loop on purpose, we want this to run at the end of the function.
		defer func() {
			totp.Key = key
		}()
		totp.Key = ""
	}

	// Ignore LastUsed and SignatureCounter?
	var opts []cmp.Option
	if ignoreUpdateAndCounter {
		opts = append(opts, cmp.FilterPath(func(path cmp.Path) bool {
			p := path.String()
			return p == "LastUsed" || p == "Device.Webauthn.SignatureCounter"
		}, cmp.Ignore()))
	}

	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("compareDevices mismatch (-want +got):\n%s", diff)
	}
}

type mockCache struct {
	Cache

	resources      []types.ResourceWithLabels
	resourcesError error
}

func (m mockCache) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if m.resourcesError != nil {
		return nil, m.resourcesError
	}

	if req.StartKey != "" {
		return nil, nil
	}

	return &types.ListResourcesResponse{Resources: m.resources}, nil
}

func TestFilterResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fail := errors.New("fail")

	const resourceCount = 100
	nodes := make([]types.ResourceWithLabels, 0, resourceCount)

	for i := 0; i < resourceCount; i++ {
		s, err := types.NewServer(uuid.NewString(), types.KindNode, types.ServerSpecV2{})
		require.NoError(t, err)
		nodes = append(nodes, s)
	}

	cases := []struct {
		name           string
		limit          int32
		filterFn       func(labels types.ResourceWithLabels) error
		errorAssertion require.ErrorAssertionFunc
		cache          mockCache
	}{
		{
			name:  "ListResources fails",
			cache: mockCache{resourcesError: fail},
			errorAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err, i...)
				require.ErrorIs(t, err, fail)
			},
		},
		{
			name:           "Done returns no errors",
			cache:          mockCache{resources: nodes},
			errorAssertion: require.NoError,
			filterFn: func(labels types.ResourceWithLabels) error {
				return ErrDone
			},
		},
		{
			name:  "fatal errors are propagated",
			cache: mockCache{resources: nodes},
			errorAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err, i...)
				require.ErrorIs(t, err, fail)
			},
			filterFn: func(labels types.ResourceWithLabels) error {
				return fail
			},
		},
		{
			name:           "no errors iterates the entire resource set",
			cache:          mockCache{resources: nodes},
			errorAssertion: require.NoError,
			filterFn: func(labels types.ResourceWithLabels) error {
				return nil
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := &Server{cache: tt.cache}

			err := srv.IterateResources(ctx, proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Namespace:    apidefaults.Namespace,
				Limit:        tt.limit,
			}, tt.filterFn)
			tt.errorAssertion(t, err)
		})
	}
}

func TestCAGeneration(t *testing.T) {
	const (
		clusterName = "cluster1"
		HostUUID    = "0000-000-000-0000"
	)
	native.PrecomputeKeys()
	// Cache key for better performance as we don't care about the value being unique.
	privKey, pubKey, err := native.GenerateKeyPair()
	require.NoError(t, err)

	ksConfig := keystore.Config{
		RSAKeyPairSource: func() (priv []byte, pub []byte, err error) {
			return privKey, pubKey, nil
		},
		HostUUID: HostUUID,
	}
	keyStore, err := keystore.NewKeyStore(ksConfig)
	require.NoError(t, err)

	for _, caType := range types.CertAuthTypes {
		t.Run(string(caType), func(t *testing.T) {
			testKeySet := suite.NewTestCA(caType, clusterName, privKey).Spec.ActiveKeys
			keySet, err := newKeySet(keyStore, types.CertAuthID{Type: caType, DomainName: clusterName})
			require.NoError(t, err)

			// Don't compare values as those are different. Only check if the key is set/not set in both cases.
			require.Equal(t, len(testKeySet.SSH) > 0, len(keySet.SSH) > 0,
				"test CA and production CA have different SSH keys for type %v", caType)
			require.Equal(t, len(testKeySet.TLS) > 0, len(keySet.TLS) > 0,
				"test CA and production CA have different TLS keys for type %v", caType)
			require.Equal(t, len(testKeySet.JWT) > 0, len(keySet.JWT) > 0,
				"test CA and production CA have different JWT keys for type %v", caType)
		})
	}
}
