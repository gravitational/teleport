// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gateway

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestLocalProxyMiddleware_OnNewConnection(t *testing.T) {
	cert, err := utils.GenerateSelfSignedCert([]string{"localhost"})
	require.NoError(t, err)
	tlsCert, err := keys.X509KeyPair(cert.Cert, cert.PrivateKey)
	require.NoError(t, err)
	x509cert, err := utils.TLSCertToX509(tlsCert)
	require.NoError(t, err)

	clockAfterCertExpiry := clockwork.NewFakeClockAt(x509cert.NotAfter)
	clockAfterCertExpiry.Advance(time.Hour*4 + time.Minute*20)

	clockBeforeCertExpiry := clockwork.NewFakeClockAt(x509cert.NotBefore)
	clockBeforeCertExpiry.Advance(time.Hour*4 + time.Minute*20)

	tests := []struct {
		name        string
		clock       clockwork.Clock
		expectation func(t *testing.T, hasCalledOnExpiredCert bool)
	}{
		{
			name:  "With expired cert",
			clock: clockAfterCertExpiry,
			expectation: func(t *testing.T, hasCalledOnExpiredCert bool) {
				require.True(t, hasCalledOnExpiredCert,
					"The onExpiredCert callback has not been called by the middleware")
			},
		},
		{
			name:  "With valid cert",
			clock: clockBeforeCertExpiry,
			expectation: func(t *testing.T, hasCalledOnExpiredCert bool) {
				require.False(t, hasCalledOnExpiredCert,
					"The onExpiredCert callback has been called by the middleware")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			hasCalledOnExpiredCert := false

			middleware := &localProxyMiddleware{
				onExpiredCert: func(context.Context) error {
					hasCalledOnExpiredCert = true
					return nil
				},
				log: logrus.WithField(trace.Component, "middleware"),
				dbRoute: tlsca.RouteToDatabase{
					Protocol:    defaults.ProtocolPostgres,
					ServiceName: "foo-database-server",
				},
			}

			localProxy, err := alpn.NewLocalProxy(alpn.LocalProxyConfig{
				RemoteProxyAddr: "localhost",
				Protocols:       []alpncommon.Protocol{alpncommon.ProtocolHTTP},
				ParentContext:   ctx,
				Clock:           tt.clock,
			})
			require.NoError(t, err)

			localProxy.SetCerts([]tls.Certificate{tlsCert})

			conn, _ := net.Pipe()

			err = middleware.OnNewConnection(ctx, localProxy, conn)
			require.NoError(t, err)

			tt.expectation(t, hasCalledOnExpiredCert)
		})
	}
}
