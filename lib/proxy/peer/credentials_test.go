// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

			creds := newClientCredentials(test.expectedPeerID, test.peerAddr, utils.NewLoggerForTests(), newTestCredentials(cert))

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
