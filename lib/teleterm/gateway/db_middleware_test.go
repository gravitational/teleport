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
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestDBMiddleware_OnNewConnection(t *testing.T) {
	testCert, err := cert.GenerateSelfSignedCert([]string{"localhost"}, []string{})
	require.NoError(t, err)
	tlsCert, err := keys.X509KeyPair(testCert.Cert, testCert.PrivateKey)
	require.NoError(t, err)
	x509cert, err := utils.TLSCertLeaf(tlsCert)
	require.NoError(t, err)

	clockAfterCertExpiry := clockwork.NewFakeClockAt(x509cert.NotAfter)
	clockAfterCertExpiry.Advance(time.Hour*4 + time.Minute*20)

	clockBeforeCertExpiry := clockwork.NewFakeClockAt(x509cert.NotBefore)
	clockBeforeCertExpiry.Advance(time.Hour*4 + time.Minute*20)

	validDbRoute := tlsca.RouteToDatabase{
		Protocol:    defaults.ProtocolPostgres,
		ServiceName: "foo-database-server",
	}

	tests := []struct {
		name        string
		clock       clockwork.Clock
		dbRoute     tlsca.RouteToDatabase
		expectation func(t *testing.T, onNewConnectionErr error, hasCalledOnExpiredCert bool)
	}{
		{
			name:    "With expired cert",
			clock:   clockAfterCertExpiry,
			dbRoute: validDbRoute,
			expectation: func(t *testing.T, onNewConnectionErr error, hasCalledOnExpiredCert bool) {
				require.NoError(t, onNewConnectionErr)
				require.True(t, hasCalledOnExpiredCert,
					"Expected the onExpiredCert callback to be called by the middleware")
			},
		},
		{
			name:  "With active cert with subject not matching dbRoute",
			clock: clockBeforeCertExpiry,
			dbRoute: tlsca.RouteToDatabase{
				Protocol:    defaults.ProtocolPostgres,
				ServiceName: "foo-database-server",
				Username:    "bar",
				Database:    "quux",
			},
			expectation: func(t *testing.T, onNewConnectionErr error, hasCalledOnExpiredCert bool) {
				require.Error(t, onNewConnectionErr)
				require.False(t, hasCalledOnExpiredCert,
					"Expected the onExpiredCert callback to not be called by the middleware")
			},
		},
		{
			name:    "With valid cert",
			clock:   clockBeforeCertExpiry,
			dbRoute: validDbRoute,
			expectation: func(t *testing.T, onNewConnectionErr error, hasCalledOnExpiredCert bool) {
				require.NoError(t, onNewConnectionErr)
				require.False(t, hasCalledOnExpiredCert,
					"Expected the onExpiredCert callback to not be called by the middleware")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			hasCalledOnExpiredCert := false

			middleware := &dbMiddleware{
				onExpiredCert: func(context.Context) error {
					hasCalledOnExpiredCert = true
					return nil
				},
				log:     logrus.WithField(trace.Component, "middleware"),
				dbRoute: tt.dbRoute,
			}

			localProxy, err := alpn.NewLocalProxy(alpn.LocalProxyConfig{
				RemoteProxyAddr: "localhost",
				Protocols:       []alpncommon.Protocol{alpncommon.ProtocolHTTP},
				ParentContext:   ctx,
				Clock:           tt.clock,
			})
			require.NoError(t, err)

			localProxy.SetCerts([]tls.Certificate{tlsCert})

			err = middleware.OnNewConnection(ctx, localProxy, nil /* net.Conn, not used by middleware */)
			tt.expectation(t, err, hasCalledOnExpiredCert)
		})
	}
}
