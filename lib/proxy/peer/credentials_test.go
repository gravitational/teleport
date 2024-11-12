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

package peer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type testCredentials struct {
	credentials.TransportCredentials
	info credentials.TLSInfo
}

func newTestCredentials(cert *x509.Certificate) testCredentials {
	return testCredentials{
		info: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}},
	}
}

func (t testCredentials) ClientHandshake(ctx context.Context, addr string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, t.info, nil
}

type fakeConn struct {
	closed bool
	net.Conn
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

func TestClientCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		expectedPeerID string
		actualPeerID   string
		peerAddr       string
		role           string
		expectedErr    error
	}{
		{
			name:           "invalid role",
			expectedPeerID: "test",
			actualPeerID:   "test",
			peerAddr:       "test",
			role:           string(types.RoleNode),
			expectedErr:    trace.AccessDenied("proxy system role required"),
		},
		{
			name:           "invalid peer id",
			expectedPeerID: "peer1",
			actualPeerID:   "peer2",
			peerAddr:       "peer-addr",
			role:           string(types.RoleProxy),
			expectedErr:    trace.AccessDenied("connected to unexpected proxy"),
		},
		{
			name:           "invalid identity",
			expectedPeerID: "",
			actualPeerID:   "",
			peerAddr:       "peer-addr",
			role:           string(types.RoleProxy),
			expectedErr:    trace.BadParameter("missing identity username"),
		},
		{
			name:           "valid connection",
			expectedPeerID: "peer1",
			actualPeerID:   "peer1",
			peerAddr:       "peer-addr",
			role:           string(types.RoleProxy),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cert := &x509.Certificate{
				Subject: pkix.Name{
					CommonName:   test.actualPeerID,
					Organization: []string{test.role},
				},
			}

			creds := newClientCredentials(test.expectedPeerID, test.peerAddr, utils.NewSlogLoggerForTests(), newTestCredentials(cert))

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			conn := &fakeConn{}
			_, _, err := creds.ClientHandshake(ctx, "127.0.0.1", conn)
			if test.expectedErr == nil {
				require.NoError(t, err)
				require.False(t, conn.closed)
				return
			}

			require.ErrorIs(t, err, test.expectedErr)
			require.True(t, conn.closed)
		})
	}
}
