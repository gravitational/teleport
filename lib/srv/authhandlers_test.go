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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

type mockCAandAuthPrefGetter struct {
	AccessPoint

	authPref types.AuthPreference
	cas      map[types.CertAuthType]types.CertAuthority
}

func (m mockCAandAuthPrefGetter) GetAuthPreference(s_12345678 context.Context) (types.AuthPreference, error) {
	return m.authPref, nil
}

func (m mockCAandAuthPrefGetter) GetCertAuthorities(_ context.Context, caType types.CertAuthType, _ bool) ([]types.CertAuthority, error) {
	ca, ok := m.cas[caType]
	if !ok {
		return nil, trace.NotFound("CA not found")
	}

	return []types.CertAuthority{ca}, nil
}

type mockLoginChecker struct {
	rbacChecked bool
}

func (m *mockLoginChecker) canLoginWithRBAC(_ *ssh.Certificate, _ types.CertAuthority, _ string, _ types.Server, _, _ string) error {
	m.rbacChecked = true
	return nil
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

	tests := []struct {
		name            string
		component       string
		nodeExists      bool
		openSSHNode     bool
		assertRBACCheck require.BoolAssertionFunc
	}{
		{
			name:            "teleport node, regular server",
			component:       teleport.ComponentNode,
			nodeExists:      true,
			openSSHNode:     false,
			assertRBACCheck: require.True,
		},
		{
			name:            "teleport node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			nodeExists:      true,
			openSSHNode:     false,
			assertRBACCheck: require.False,
		},
		{
			name:            "registered openssh node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			nodeExists:      true,
			openSSHNode:     true,
			assertRBACCheck: require.True,
		},
		{
			name:            "unregistered openssh node, forwarding server",
			component:       teleport.ComponentForwardingNode,
			nodeExists:      false,
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

	accessPoint := mockCAandAuthPrefGetter{
		AccessPoint: server.auth,
		authPref:    types.DefaultAuthPreference(),
		cas: map[types.CertAuthType]types.CertAuthority{
			types.UserCA: userCA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create node resource
			var target types.Server
			if tt.nodeExists {
				n, err := types.NewServer("testie_node", types.KindNode, types.ServerSpecV2{
					Addr:     "1.2.3.4:22",
					Hostname: "testie",
					Version:  types.V2,
				})
				require.NoError(t, err)
				server, ok := n.(*types.ServerV2)
				require.True(t, ok)
				if tt.openSSHNode {
					server.SubKind = types.SubKindOpenSSHNode
				}
				target = server
			}

			config := &AuthHandlerConfig{
				Server:       server,
				Component:    tt.component,
				Emitter:      &eventstest.MockRecorderEmitter{},
				AccessPoint:  accessPoint,
				TargetServer: target,
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

			c, err := keygen.GenerateUserCert(services.UserCertParams{
				CASigner:      caSigner,
				PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
				Username:      "testuser",
				AllowedLogins: []string{"testuser"},
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
		cas: map[types.CertAuthType]types.CertAuthority{
			types.UserCA: userCA,
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
			c, err := keygen.GenerateUserCert(services.UserCertParams{
				CASigner:      caSigner,
				PublicUserKey: privateKey.MarshalSSHPublicKey(),
				Username:      username,
				AllowedLogins: []string{username},
				Traits: wrappers.Traits{
					teleport.TraitInternalPrefix: []string{""},
				},
				Roles:             []string{tt.role},
				CertificateFormat: constants.CertificateFormatStandard,
			})
			require.NoError(t, err)

			cert, err := sshutils.ParseCertificate(c)
			require.NoError(t, err)

			err = ah.canLoginWithRBAC(cert, userCA, clusterName, node, username, teleport.SSHSessionJoinPrincipal)
			tt.testError(t, err)
		})
	}
}
