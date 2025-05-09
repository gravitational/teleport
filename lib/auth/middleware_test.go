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

package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
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
				ClusterName: localClusterName,
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
	ctx := context.Background()

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

	clusterName, err := s.GetClusterName(ctx)
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := &Middleware{
				ClusterName: clusterName.GetClusterName(),
			}

			conn := &testConn{
				state: tls.ConnectionState{
					PeerCertificates:  tt.peers,
					HandshakeComplete: !tt.needsHandshake,
				},
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

func TestMiddleware_ServeHTTP(t *testing.T) {
	t.Parallel()
	localClusterName := "local"
	remoteClusterName := "remote"
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
		Usage:           []string{},
		Principals:      []string{},
	}

	remoteUserIdentity := tlsca.Identity{
		Username:        "foo",
		Groups:          []string{"devs"},
		TeleportCluster: remoteClusterName,
		Expires:         now,
		Usage:           []string{},
		Principals:      []string{},
	}

	proxyIdentity := tlsca.Identity{
		Username:        "proxy...",
		Groups:          []string{string(types.RoleProxy)},
		TeleportCluster: localClusterName,
		Expires:         now,
		Usage:           []string{},
		Principals:      []string{},
	}

	remoteProxyIdentity := tlsca.Identity{
		Username:        "proxy...",
		Groups:          []string{string(types.RoleProxy)},
		TeleportCluster: remoteClusterName,
		Expires:         now,
		Usage:           []string{},
		Principals:      []string{},
	}

	dbIdentity := tlsca.Identity{
		Username:        "db...",
		Groups:          []string{string(types.RoleDatabase)},
		TeleportCluster: localClusterName,
		Expires:         now,
		Usage:           []string{},
		Principals:      []string{},
	}

	type args struct {
		impersonateIdentity *tlsca.Identity
		peers               []*x509.Certificate
		sourceIPAddr        string
		impersonatedIPAddr  string
	}
	type want struct {
		user       authz.IdentityGetter
		userIPAddr string
	}
	tests := []struct {
		name                                  string
		args                                  args
		want                                  want
		credentialsForwardingDennied          bool
		enableCredentialsForwarding           bool
		impersonateLocalUserViaRemoteProxyErr bool
	}{
		{
			name: "local user without impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, localUserIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				sourceIPAddr: "127.0.0.1:6514",
			},
			want: want{
				user: authz.LocalUser{
					Username: localUserIdentity.Username,
					Identity: localUserIdentity,
				},
				userIPAddr: "127.0.0.1:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "remote user without impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, remoteUserIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
				}},
				sourceIPAddr: "127.0.0.1:6514",
			},
			want: want{
				user: authz.RemoteUser{
					Username:    remoteUserIdentity.Username,
					Identity:    remoteUserIdentity,
					RemoteRoles: remoteUserIdentity.Groups,
					ClusterName: remoteClusterName,
					Principals:  []string{},
				},
				userIPAddr: "127.0.0.1:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "proxy without impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, proxyIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				sourceIPAddr: "127.0.0.1:6514",
			},
			want: want{
				user: authz.BuiltinRole{
					Username:    proxyIdentity.Username,
					Identity:    proxyIdentity,
					Role:        types.RoleProxy,
					ClusterName: localClusterName,
				},
				userIPAddr: "127.0.0.1:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "db without impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, dbIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				sourceIPAddr: "127.0.0.1:6514",
			},
			want: want{
				user: authz.BuiltinRole{
					Username:    dbIdentity.Username,
					Identity:    dbIdentity,
					Role:        types.RoleDatabase,
					ClusterName: localClusterName,
				},
				userIPAddr: "127.0.0.1:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "proxy with impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, proxyIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				impersonateIdentity: &localUserIdentity,
				sourceIPAddr:        "127.0.0.1:6514",
				impersonatedIPAddr:  "127.0.0.2:6514",
			},
			want: want{
				user: authz.LocalUser{
					Username: localUserIdentity.Username,
					Identity: localUserIdentity,
				},
				userIPAddr: "127.0.0.2:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "proxy with remote user impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, proxyIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				impersonateIdentity: &remoteUserIdentity,
				sourceIPAddr:        "127.0.0.1:6514",
				impersonatedIPAddr:  "127.0.0.2:6514",
			},
			want: want{
				user: authz.RemoteUser{
					Username:    remoteUserIdentity.Username,
					Identity:    remoteUserIdentity,
					RemoteRoles: remoteUserIdentity.Groups,
					ClusterName: remoteClusterName,
					Principals:  []string{},
				},
				userIPAddr: "127.0.0.2:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  true,
		},
		{
			name: "db with impersonation but disabled forwarding",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, dbIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				impersonateIdentity: &localUserIdentity,
			},
			credentialsForwardingDennied: true,
			enableCredentialsForwarding:  true,
		},
		{
			name: "proxy with remote user impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, proxyIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{localClusterName}},
				}},
				impersonateIdentity: &remoteUserIdentity,
				sourceIPAddr:        "127.0.0.1:6514",
				impersonatedIPAddr:  "127.0.0.2:6514",
			},
			credentialsForwardingDennied: false,
			enableCredentialsForwarding:  false,
		},
		{
			name: "remote proxy with local user impersonation",
			args: args{
				peers: []*x509.Certificate{{
					Subject:  subject(t, remoteProxyIdentity),
					NotAfter: now,
					Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
				}},
				impersonateIdentity: &localUserIdentity,
				sourceIPAddr:        "127.0.0.1:6514",
				impersonatedIPAddr:  "127.0.0.2:6514",
			},
			enableCredentialsForwarding:           true,
			impersonateLocalUserViaRemoteProxyErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Middleware{
				ClusterName: localClusterName,
				Handler: &fakeHTTPHandler{
					t:                 t,
					expectedUser:      tt.want.user,
					mustPanicIfCalled: tt.credentialsForwardingDennied,
					userIP:            tt.want.userIPAddr,
				},
				EnableCredentialsForwarding: tt.enableCredentialsForwarding,
			}
			r := &http.Request{
				Header: make(http.Header),
				TLS: &tls.ConnectionState{
					PeerCertificates: tt.args.peers,
				},
				RemoteAddr: tt.args.sourceIPAddr,
			}
			if tt.args.impersonateIdentity != nil {
				data, err := json.Marshal(tt.args.impersonateIdentity)
				require.NoError(t, err)
				r.Header.Set(TeleportImpersonateUserHeader, string(data))
				r.Header.Set(TeleportImpersonateIPHeader, tt.args.impersonatedIPAddr)
			}
			rsp := httptest.NewRecorder()
			a.ServeHTTP(rsp, r)
			if tt.credentialsForwardingDennied {
				require.True(t,
					bytes.Contains(
						rsp.Body.Bytes(),
						[]byte("Credentials forwarding is only permitted for Proxy"),
					),
				)
			}
			if !tt.enableCredentialsForwarding {
				require.True(t,
					bytes.Contains(
						rsp.Body.Bytes(),
						[]byte("Credentials forwarding is not permitted by this service"),
					),
				)
			}
			if tt.impersonateLocalUserViaRemoteProxyErr {
				require.True(t,
					bytes.Contains(
						rsp.Body.Bytes(),
						[]byte("can not impersonate users via a different cluster proxy"),
					),
				)
			}
		})
	}
}

type fakeHTTPHandler struct {
	t                 *testing.T
	expectedUser      authz.IdentityGetter
	mustPanicIfCalled bool
	userIP            string
}

func (h *fakeHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.mustPanicIfCalled {
		panic("handler should not be called")
	}
	user, err := authz.UserFromContext(r.Context())
	require.NoError(h.t, err)
	require.Equal(h.t, h.expectedUser, user)
	clientSrcAddr, err := authz.ClientSrcAddrFromContext(r.Context())
	require.NoError(h.t, err)
	require.Equal(h.t, h.userIP, clientSrcAddr.String())
	require.Equal(h.t, h.userIP, r.RemoteAddr)
	// Ensure that the Teleport-Impersonate-User header is not set on the request
	// after the middleware has run.
	require.Empty(h.t, r.Header.Get(TeleportImpersonateUserHeader))
	require.Empty(h.t, r.Header.Get(TeleportImpersonateIPHeader))
}

type fakeConn struct {
	net.Conn
	closed atomic.Bool
}

func (f *fakeConn) Close() error {
	f.closed.CompareAndSwap(false, true)
	return nil
}

func (f *fakeConn) RemoteAddr() net.Addr {
	return &utils.NetAddr{
		Addr:        "127.0.0.1:6514",
		AddrNetwork: "tcp",
	}
}

func TestValidateClientVersion(t *testing.T) {
	cases := []struct {
		name          string
		middleware    *Middleware
		clientVersion string
		errAssertion  func(t *testing.T, err error)
	}{
		{
			name:       "rejection disabled",
			middleware: &Middleware{},
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:       "rejection enabled and client version not specified",
			middleware: &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "client rejected",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 2}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
		{
			name:          "valid client v-1",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 1}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "valid client v-0",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "invalid client version",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: "abc123",
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
		{
			name:          "pre-release client allowed",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 1, PreRelease: "dev.abcd.123"}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:          "pre-release client rejected",
			middleware:    &Middleware{OldestSupportedVersion: teleport.MinClientSemVer()},
			clientVersion: semver.Version{Major: api.VersionMajor - 2, PreRelease: "dev.abcd.123"}.String(),
			errAssertion: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err), "got %T, expected access denied error", err)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.clientVersion != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{"version": tt.clientVersion}))
			}

			tt.errAssertion(t, tt.middleware.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I}))
		})
	}
}

func TestRejectedClientClusterAlertContents(t *testing.T) {
	var alerts []types.ClusterAlert
	mw := Middleware{
		OldestSupportedVersion: teleport.MinClientSemVer(),
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			alerts = append(alerts, a)
			return nil
		},
	}

	alertVersion := semver.Version{
		Major: mw.OldestSupportedVersion.Major,
		Minor: mw.OldestSupportedVersion.Minor,
		Patch: mw.OldestSupportedVersion.Patch,
	}.String()

	version := semver.Version{Major: api.VersionMajor - 5}.String()

	tests := []struct {
		name      string
		userAgent string
		identity  authz.IdentityGetter
		expected  string
	}{
		{
			name:     "invalid node",
			identity: TestServerID(types.RoleNode, "1-2-3-4").I,
			expected: fmt.Sprintf("Connection from Node 1-2-3-4 at 127.0.0.1:6514, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", version, alertVersion),
		},
		{
			name:      "invalid tsh",
			userAgent: "tsh/" + teleport.Version,
			identity:  TestUser("llama").I,
			expected:  fmt.Sprintf("Connection from tsh v%s by llama was rejected. Connections will be allowed after upgrading tsh to v%s or newer", version, alertVersion),
		},
		{
			name: "invalid remote node",
			identity: authz.RemoteBuiltinRole{
				Role:        types.RoleNode,
				Username:    string(types.RoleNode),
				ClusterName: "leaf",
				Identity: tlsca.Identity{
					Username: "1-2-3-4",
				},
			},
			expected: fmt.Sprintf("Connection from Node 1-2-3-4 at 127.0.0.1:6514 in cluster leaf, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", version, alertVersion),
		},

		{
			name:     "invalid tool",
			identity: TestUser("llama").I,
			expected: fmt.Sprintf("Connection from tsh, tctl, tbot, or a plugin running v%s by llama was rejected. Connections will be allowed after upgrading to v%s or newer", version, alertVersion),
		},
	}

	// Trigger alerts from a variety of identities and validate the content of emitted alerts.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"version":    version,
				"user-agent": test.userAgent,
			}))

			err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: test.identity})
			assert.Error(t, err)

			// Assert that only an alert was created and the content matches expectations.
			require.Len(t, alerts, 1)
			require.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
			require.Equal(t, test.expected, alerts[0].Spec.Message)

			// Reset the test alerts.
			alerts = nil

			// Reset the last alert time to a time beyond the rate limit, allowing the next
			// rejection to trigger another alert.
			mw.lastRejectedAlertTime.Store(time.Now().Add(-25 * time.Hour).UnixNano())
		})
	}
}

func TestRejectedClientClusterAlert(t *testing.T) {
	var alerts []types.ClusterAlert
	mw := Middleware{
		OldestSupportedVersion: teleport.MinClientSemVer(),
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			alerts = append(alerts, a)
			return nil
		},
	}

	// Validate an unsupported client, which should trigger an alert
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		"version": semver.Version{Major: api.VersionMajor - 20}.String(),
	}))
	err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
	assert.Error(t, err)

	// Validate a client with an unknown version, which should trigger an alert, however,
	// due to rate limiting of 1 alert per 24h no alert should be created.
	ctx = metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		"version": "abcd",
	}))
	err = mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
	assert.Error(t, err)

	// Assert that only a single alert was created based on the above rejections.
	require.Len(t, alerts, 1)
	require.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
	// Assert that the version in the message does not contain any prereleases
	require.NotContains(t, alerts[0].Spec.Message, "-aa")

	for _, tool := range []string{"tsh", "tctl", "tbot"} {
		t.Run(tool, func(t *testing.T) {
			// Reset the test alerts.
			alerts = nil

			// Reset the last alert time to a time beyond the rate limit, allowing the next
			// rejection to trigger another alert.
			mw.lastRejectedAlertTime.Store(time.Now().Add(-25 * time.Hour).UnixNano())

			// Create a new context with the user-agent set to a client tool. This should alter the
			// text in the alert to indicate the connection was from a client tool and not an agent.
			ctx = metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"version":    semver.Version{Major: api.VersionMajor - 20}.String(),
				"user-agent": tool + "/" + teleport.Version,
			}))

			// Validate two unsupported clients in parallel to verify that concurrent attempts
			// to create an alert are prevented.
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := mw.ValidateClientVersion(ctx, IdentityInfo{Conn: &fakeConn{}, IdentityGetter: TestBuiltin(types.RoleNode).I})
					assert.Error(t, err)
				}()
			}

			wg.Wait()

			// Assert that only a single additional alert was created and that
			// it was created for clients and not agents.
			require.Len(t, alerts, 1)
			assert.Equal(t, "rejected-unsupported-connection", alerts[0].GetName())
			require.Contains(t, alerts[0].Spec.Message, tool)
			// Assert that the version in the message does not contain any prereleases
			require.NotContains(t, alerts[0].Spec.Message, "-aa")
		})
	}

}
