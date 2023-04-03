/*
Copyright 2020 Gravitational, Inc.

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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMiddlewareGetUser(t *testing.T) {
	t.Parallel()
	const (
		localClusterName  = "local"
		remoteClusterName = "remote"
	)
	s := newTestServices(t)
	// Set up local cluster name in the backend.
	cn, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: localClusterName,
	})
	require.NoError(t, err)
	require.NoError(t, s.UpsertClusterName(cn))

	now := time.Date(2020, time.November, 5, 0, 0, 0, 0, time.UTC)

	var (
		localUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		localUserIdentityNoTeleportCluster = tlsca.Identity{
			Username: "foo",
			Groups:   []string{"devs"},
			Expires:  now,
		}
		localSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(types.RoleNode)},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		remoteUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
		remoteUserIdentityNoTeleportCluster = tlsca.Identity{
			Username: "foo",
			Groups:   []string{"devs"},
			Expires:  now,
		}
		remoteSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(types.RoleNode)},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
	)

	tests := []struct {
		desc      string
		peers     []*x509.Certificate
		wantID    authz.IdentityGetter
		assertErr require.ErrorAssertionFunc
	}{
		{
			desc: "no client cert",
			wantID: authz.BuiltinRole{
				Role:        types.RoleNop,
				Username:    string(types.RoleNop),
				ClusterName: localClusterName,
				Identity:    tlsca.Identity{},
			},
			assertErr: require.NoError,
		},
		{
			desc: "local user",
			peers: []*x509.Certificate{{
				Subject:  subject(t, localUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: authz.LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "local user no teleport cluster in cert subject",
			peers: []*x509.Certificate{{
				Subject:  subject(t, localUserIdentityNoTeleportCluster),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: authz.LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "local system role",
			peers: []*x509.Certificate{{
				Subject:  subject(t, localSystemRole),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: authz.BuiltinRole{
				Username:    localSystemRole.Username,
				Role:        types.RoleNode,
				ClusterName: localClusterName,
				Identity:    localSystemRole,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote user",
			peers: []*x509.Certificate{{
				Subject:  subject(t, remoteUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: authz.RemoteUser{
				ClusterName: remoteClusterName,
				Username:    remoteUserIdentity.Username,
				RemoteRoles: remoteUserIdentity.Groups,
				Identity:    remoteUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote user no teleport cluster in cert subject",
			peers: []*x509.Certificate{{
				Subject:  subject(t, remoteUserIdentityNoTeleportCluster),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: authz.RemoteUser{
				ClusterName: remoteClusterName,
				Username:    remoteUserIdentity.Username,
				RemoteRoles: remoteUserIdentity.Groups,
				Identity:    remoteUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote system role",
			peers: []*x509.Certificate{{
				Subject:  subject(t, remoteSystemRole),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: authz.RemoteBuiltinRole{
				Username:    remoteSystemRole.Username,
				Role:        types.RoleNode,
				ClusterName: remoteClusterName,
				Identity:    remoteSystemRole,
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {

			m := &Middleware{
				AccessPoint: s,
			}

			id, err := m.GetUser(tls.ConnectionState{PeerCertificates: tt.peers})
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(id, tt.wantID, cmpopts.EquateEmpty()))
		})
	}
}

func TestCheckIPPinning(t *testing.T) {
	testCases := []struct {
		desc       string
		clientAddr string
		pinnedIP   string
		pinIP      bool
		wantErr    string
	}{
		{
			desc:       "no IP pinning",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "",
			pinIP:      false,
		},
		{
			desc:       "IP pinning, no pinned IP",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "",
			pinIP:      true,
			wantErr:    "pinned IP is required for the user, but is not present on identity",
		},
		{
			desc:       "Pinned IP doesn't match",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      true,
			wantErr:    "pinned IP doesn't match observed client IP",
		},
		{
			desc:       "Role doesn't require IP pinning now, but old certificate still pinned",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.2",
			pinIP:      false,
			wantErr:    "pinned IP doesn't match observed client IP",
		},
		{
			desc:     "IP pinning enabled, missing client IP",
			pinnedIP: "127.0.0.1",
			pinIP:    true,
			wantErr:  "missing observed client IP while checking IP pinning",
		},
		{
			desc:       "correct IP pinning",
			clientAddr: "127.0.0.1:444",
			pinnedIP:   "127.0.0.1",
			pinIP:      true,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		if tt.clientAddr != "" {
			ctx = authz.ContextWithClientAddr(ctx, utils.MustParseAddr(tt.clientAddr))
		}
		identity := tlsca.Identity{PinnedIP: tt.pinnedIP}

		err := CheckIPPinning(ctx, identity, tt.pinIP)

		if tt.wantErr != "" {
			require.ErrorContains(t, err, tt.wantErr)
		} else {
			require.NoError(t, err)
		}

	}
}

// testConn is a connection that implements utils.TLSConn for testing WrapContextWithUser.
type testConn struct {
	tls.Conn

	state           tls.ConnectionState
	handshakeCalled bool
	remoteAddr      net.Addr
}

func (t *testConn) ConnectionState() tls.ConnectionState   { return t.state }
func (t *testConn) Handshake() error                       { t.handshakeCalled = true; return nil }
func (t *testConn) HandshakeContext(context.Context) error { return t.Handshake() }
func (t *testConn) RemoteAddr() net.Addr                   { return t.remoteAddr }

func TestWrapContextWithUser(t *testing.T) {
	localClusterName := "local"
	s := newTestServices(t)

	// Set up local cluster name in the backend.
	cn, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: localClusterName,
	})
	require.NoError(t, err)
	require.NoError(t, s.UpsertClusterName(cn))

	now := time.Date(2020, time.November, 5, 0, 0, 0, 0, time.UTC)
	localUserIdentity := tlsca.Identity{
		Username:        "foo",
		Groups:          []string{"devs"},
		TeleportCluster: localClusterName,
		Expires:         now,
	}

	tests := []struct {
		desc           string
		peers          []*x509.Certificate
		wantID         authz.IdentityGetter
		needsHandshake bool
	}{
		{
			desc: "local user doesn't need handshake",
			peers: []*x509.Certificate{{
				Subject:  subject(t, localUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: authz.LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			needsHandshake: false,
		},
		{
			desc: "local user needs handshake",
			peers: []*x509.Certificate{{
				Subject:  subject(t, localUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: authz.LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			needsHandshake: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := &Middleware{
				AccessPoint: s,
			}

			conn := &testConn{
				state: tls.ConnectionState{PeerCertificates: tt.peers,
					HandshakeComplete: !tt.needsHandshake},
				remoteAddr: utils.MustParseAddr("127.0.0.1:4242"),
			}

			parentCtx := context.Background()
			ctx, err := m.WrapContextWithUser(parentCtx, conn)
			require.NoError(t, err)
			require.Equal(t, tt.needsHandshake, conn.handshakeCalled)

			cert, err := authz.UserCertificateFromContext(ctx)
			require.NoError(t, err)
			user, err := authz.UserFromContext(ctx)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(cert, tt.peers[0], cmpopts.EquateEmpty()))
			require.Empty(t, cmp.Diff(user, tt.wantID, cmpopts.EquateEmpty()))
		})
	}
}

// Helper func for generating fake certs.
func subject(t *testing.T, id tlsca.Identity) pkix.Name {
	s, err := id.Subject()
	require.NoError(t, err)
	// ExtraNames get moved to Names when generating a real x509 cert.
	// Since we're just mimicking certs in memory, move manually.
	s.Names = s.ExtraNames
	return s
}
