/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestHTTPSTunnelHandler(t *testing.T) {
	mockUser := authz.LocalUser{
		Username: "alice",
		Identity: tlsca.Identity{
			Username: "alice",
			RouteToApp: tlsca.RouteToApp{
				SessionID:   "test-session-id",
				PublicAddr:  "app.example.com",
				ClusterName: "test-cluster",
			},
		},
	}
	handshakeErr := errors.New("handshake failed")

	tests := []struct {
		name            string
		tlsConfig       *tls.Config
		conn            net.Conn
		auth            httpsConnAuthorizer
		next            func(context.Context, net.Conn) error
		wantErrContains string
	}{
		{
			name:            "missing TLS config",
			conn:            &mockTLSConn{},
			wantErrContains: "missing tls.Config",
		},
		{
			name:            "handshake error",
			tlsConfig:       &tls.Config{},
			conn:            &mockTLSConn{handshakeErr: handshakeErr},
			wantErrContains: handshakeErr.Error(),
		},
		{
			name:            "auth error",
			tlsConfig:       &tls.Config{},
			conn:            &mockTLSConn{},
			auth:            &mockHTTPSConnAuthorizer{err: trace.AccessDenied("access denied")},
			wantErrContains: "access denied",
		},
		{
			name:      "success",
			tlsConfig: &tls.Config{},
			conn:      &mockTLSConn{},
			auth:      &mockHTTPSConnAuthorizer{user: mockUser},
			next: func(_ context.Context, c net.Conn) error {
				defer c.Close()
				r := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
				r = r.WithContext(authz.ContextWithConn(r.Context(), c))
				if !IsHTTPSTunnelConn(r) {
					return trace.BadParameter("not a https tunnel connection")
				}
				user, err := getIdentityFromHTTPSTunnelRequest(r)
				if err != nil {
					return trace.Wrap(err)
				}
				if user.Username != mockUser.Username {
					return trace.AccessDenied("wrong username %q vs %q", user.Username, mockUser.Username)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHTTPSTunnelHandler(tt.next, "cluster-name")
			h.SetTLSConfig(tt.tlsConfig)
			h.auth = cmp.Or(tt.auth, h.auth)

			err := h.HandleConnection(context.Background(), tt.conn)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

// mockTLSConn implements utils.TLSConn for testing.
type mockTLSConn struct {
	net.Conn
	handshakeErr error
}

func (m *mockTLSConn) ConnectionState() tls.ConnectionState     { return tls.ConnectionState{} }
func (m *mockTLSConn) Handshake() error                         { return m.handshakeErr }
func (m *mockTLSConn) HandshakeContext(_ context.Context) error { return m.handshakeErr }
func (m *mockTLSConn) Close() error                             { return nil }

func (m *mockTLSConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 443}
}

func (m *mockTLSConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}
}

type mockHTTPSConnAuthorizer struct {
	user authz.IdentityGetter
	err  error
}

func (m *mockHTTPSConnAuthorizer) GetUser(_ context.Context, _ tls.ConnectionState) (authz.IdentityGetter, error) {
	return m.user, m.err
}
