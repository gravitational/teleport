/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"context"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/sshca"
)

type mockCAandAuthPrefGetter struct {
	AccessPoint

	authPref types.AuthPreference
	cas      map[types.CertAuthType][]types.CertAuthority
}

func (m mockCAandAuthPrefGetter) GetAuthPreference(s_12345678 context.Context) (types.AuthPreference, error) {
	return m.authPref, nil
}

func (m mockCAandAuthPrefGetter) GetCertAuthorities(_ context.Context, caType types.CertAuthType, _ bool) ([]types.CertAuthority, error) {
	cas, ok := m.cas[caType]
	if !ok {
		return nil, trace.NotFound("CA not found")
	}

	return cas, nil
}

type mockLoginChecker struct {
	rbacChecked bool
}

func (m *mockLoginChecker) evaluateSSHAccess(_ *sshca.Identity, _ types.CertAuthority, _ string, _ types.Server, _ string) (*decisionpb.SSHAccessPermit, error) {
	m.rbacChecked = true
	return nil, nil
}

type mockConnMetadata struct{}

func (m mockConnMetadata) User() string {
	return "testuser"
}

func (m mockConnMetadata) SessionID() []byte {
	return nil
}

func (m mockConnMetadata) ClientVersion() []byte {
	return nil
}

func (m mockConnMetadata) ServerVersion() []byte {
	return nil
}

func (m mockConnMetadata) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 22,
	}
}

func (m mockConnMetadata) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 9001,
	}
}

func TestRBAC(t *testing.T) {
	t.Parallel()

	node, err := types.NewNode("testie_node", types.SubKindTeleportNode, types.ServerSpecV2{
		Addr:     "1.2.3.4:22",
		Hostname: "testie",
	}, nil)
	require.NoError(t, err)

	openSSHNode, err := types.NewNode("openssh", types.SubKindOpenSSHNode, types.ServerSpecV2{
		Addr:     "1.2.3.4:22",
		Hostname: "openssh",
	}, nil)
	require.NoError(t, err)

	gitServer, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  "org",
		Organization: "org",
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		component       string
		targetServer    types.Server
		assertRBACCheck require.BoolAssertionFunc
	}{
		{
			name:            "teleport node, regular server",
			component:       teleport.ComponentNode,
			targetServer:    node,
			assertRBACCheck: require.True,
		},
		{
			name:            "teleport node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			targetServer:    node,
			assertRBACCheck: require.False,
		},
		{
			name:            "registered openssh node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			targetServer:    openSSHNode,
			assertRBACCheck: require.True,
		},
		{
			name:            "unregistered openssh node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			targetServer:    nil,
			assertRBACCheck: require.False,
		},
		{
			name:            "forwarding git",
			component:       teleport.ComponentForwardingGit,
			targetServer:    gitServer,
			assertRBACCheck: require.False,
		},
	}

	// create User CA
	userCAPriv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: "localhost",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PublicKey:      userCAPriv.MarshalSSHPublicKey(),
					PrivateKey:     userCAPriv.PrivateKeyPEM(),
					PrivateKeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	})
	require.NoError(t, err)

	// create mock SSH server and add a cluster name
	server := newMockServer(t)
	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "localhost",
		ClusterID:   "cluster_id",
	})
	require.NoError(t, err)
	err = server.auth.SetClusterName(clusterName)
	require.NoError(t, err)

	_, err = server.auth.CreateClusterNetworkingConfig(context.Background(), types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)

	accessPoint := mockCAandAuthPrefGetter{
		AccessPoint: server.auth,
		authPref:    types.DefaultAuthPreference(),
		cas: map[types.CertAuthType][]types.CertAuthority{
			types.UserCA: {userCA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AuthHandlerConfig{
				Server:       server,
				Component:    tt.component,
				Emitter:      &eventstest.MockRecorderEmitter{},
				AccessPoint:  accessPoint,
				TargetServer: tt.targetServer,
			}
			ah, err := NewAuthHandlers(config)
			require.NoError(t, err)

			lc := mockLoginChecker{}
			ah.loginChecker = &lc

			// create SSH certificate
			caSigner, err := ssh.NewSignerFromKey(userCAPriv)
			require.NoError(t, err)
			keygen := testauthority.New()
			privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
			require.NoError(t, err)

			c, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
				CASigner:      caSigner,
				PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
				Identity: sshca.Identity{
					Username:   "testuser",
					Principals: []string{"testuser"},
				},
			})
			require.NoError(t, err)

			cert, err := sshutils.ParseCertificate(c)
			require.NoError(t, err)

			// preform public key authentication
			_, err = ah.UserKeyAuth(&mockConnMetadata{}, cert)
			require.NoError(t, err)

			tt.assertRBACCheck(t, lc.rbacChecked)
		})
	}
}

// TestForwardingGitLocalOnly verifies that remote identities are categorically rejected
// by UserKeyAuth when the auth handler is running as a ForwardingGit component.
func TestForwardingGitLocalOnly(t *testing.T) {
	t.Parallel()

	gitServer, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  "org",
		Organization: "org",
	})
	require.NoError(t, err)

	// create local User CA
	localCAPriv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	localCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: "localhost",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PublicKey:      localCAPriv.MarshalSSHPublicKey(),
					PrivateKey:     localCAPriv.PrivateKeyPEM(),
					PrivateKeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	})
	require.NoError(t, err)

	// create remote User CA
	remoteCAPriv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	remoteCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: "remotehost",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PublicKey:      remoteCAPriv.MarshalSSHPublicKey(),
					PrivateKey:     remoteCAPriv.PrivateKeyPEM(),
					PrivateKeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	})
	require.NoError(t, err)

	// create mock SSH server and add a cluster name
	server := newMockServer(t)
	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "localhost",
		ClusterID:   "cluster_id",
	})
	require.NoError(t, err)
	err = server.auth.SetClusterName(clusterName)
	require.NoError(t, err)

	accessPoint := mockCAandAuthPrefGetter{
		AccessPoint: server.auth,
		authPref:    types.DefaultAuthPreference(),
		cas: map[types.CertAuthType][]types.CertAuthority{
			types.UserCA: {remoteCA, localCA},
		},
	}

	config := &AuthHandlerConfig{
		Server:       server,
		Component:    teleport.ComponentForwardingGit,
		Emitter:      &eventstest.MockRecorderEmitter{},
		AccessPoint:  accessPoint,
		TargetServer: gitServer,
	}
	ah, err := NewAuthHandlers(config)
	require.NoError(t, err)

	lc := mockLoginChecker{}
	ah.loginChecker = &lc

	privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	keygen := testauthority.New()

	// create local SSH certificate
	localCASigner, err := ssh.NewSignerFromKey(localCAPriv)
	require.NoError(t, err)

	localCertRaw, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:      localCASigner,
		PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
		Identity: sshca.Identity{
			Username:   "testuser",
			Principals: []string{"testuser"},
		},
	})
	require.NoError(t, err)

	localCert, err := sshutils.ParseCertificate(localCertRaw)
	require.NoError(t, err)

	// create remote SSH certificate
	remoteCASigner, err := ssh.NewSignerFromKey(remoteCAPriv)
	require.NoError(t, err)

	remoteCertRaw, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:      remoteCASigner,
		PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
		Identity: sshca.Identity{
			Username:   "testuser",
			Principals: []string{"testuser"},
		},
	})
	require.NoError(t, err)

	remoteCert, err := sshutils.ParseCertificate(remoteCertRaw)
	require.NoError(t, err)

	// verify that authentication succeeds for local cert but is rejected categorically for remote
	_, err = ah.UserKeyAuth(&mockConnMetadata{}, localCert)
	require.NoError(t, err)

	_, err = ah.UserKeyAuth(&mockConnMetadata{}, remoteCert)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cross-cluster git forwarding is not supported")
}

// TestRBACJoinMFA tests that MFA is enforced correctly when joining
// sessions depending on the cluster auth preference and roles presented.
func TestRBACJoinMFA(t *testing.T) {
	t.Parallel()

	const clusterName = "localhost"
	const username = "testuser"

	// create User CA
	userCAPriv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PublicKey:      userCAPriv.MarshalSSHPublicKey(),
					PrivateKey:     userCAPriv.PrivateKeyPEM(),
					PrivateKeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	})
	require.NoError(t, err)

	// create mock SSH server and add a cluster name
	server := newMockServer(t)
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: clusterName,
		ClusterID:   "cluster_id",
	})
	require.NoError(t, err)
	err = server.auth.SetClusterName(cn)
	require.NoError(t, err)
	ctx := context.Background()

	accessPoint := &mockCAandAuthPrefGetter{
		AccessPoint: server.auth,
		cas: map[types.CertAuthType][]types.CertAuthority{
			types.UserCA: {userCA},
		},
	}

	// create auth handler and dummy node
	config := &AuthHandlerConfig{
		Server:      server,
		Emitter:     &eventstest.MockRecorderEmitter{},
		AccessPoint: accessPoint,
	}
	ah, err := NewAuthHandlers(config)
	require.NoError(t, err)

	node, err := types.NewServer("testie_node", types.KindNode, types.ServerSpecV2{
		Addr:     "1.2.3.4:22",
		Hostname: "testie",
		Version:  types.V2,
	})
	require.NoError(t, err)

	mfaAuthPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
	})
	require.NoError(t, err)

	noMFAAuthPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_OFF,
	})
	require.NoError(t, err)

	// create roles
	joinMFARole, err := types.NewRole("joinMFA", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)
	_, err = server.auth.CreateRole(ctx, joinMFARole)
	require.NoError(t, err)

	joinRole, err := types.NewRole("join", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)
	_, err = server.auth.CreateRole(ctx, joinRole)
	require.NoError(t, err)

	tests := []struct {
		name      string
		authPref  types.AuthPreference
		role      string
		testError func(t *testing.T, err error)
	}{
		{
			name:     "MFA cluster auth, MFA role",
			authPref: mfaAuthPref,
			role:     joinMFARole.GetName(),
			testError: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:     "MFA cluster auth, no MFA role",
			authPref: mfaAuthPref,
			role:     joinRole.GetName(),
			testError: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:     "no MFA cluster auth, MFA role",
			authPref: noMFAAuthPref,
			role:     joinMFARole.GetName(),
			testError: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:     "no MFA cluster auth, no MFA role",
			authPref: noMFAAuthPref,
			role:     joinRole.GetName(),
			testError: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessPoint.authPref = tt.authPref

			// create SSH certificate
			caSigner, err := ssh.NewSignerFromSigner(userCAPriv.Signer)
			require.NoError(t, err)
			privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
			require.NoError(t, err)

			keygen := testauthority.New()
			c, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
				CASigner:          caSigner,
				PublicUserKey:     privateKey.MarshalSSHPublicKey(),
				CertificateFormat: constants.CertificateFormatStandard,
				Identity: sshca.Identity{
					Username:   username,
					Principals: []string{username},
					Traits: wrappers.Traits{
						teleport.TraitInternalPrefix: []string{""},
					},
					Roles: []string{tt.role},
				},
			})
			require.NoError(t, err)

			cert, err := sshutils.ParseCertificate(c)
			require.NoError(t, err)

			ident, err := sshca.DecodeIdentity(cert)
			require.NoError(t, err)

			_, err = ah.evaluateSSHAccess(ident, userCA, clusterName, node, teleport.SSHSessionJoinPrincipal)
			tt.testError(t, err)
		})
	}
}
