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

package gateway

import (
	"context"
	"crypto/tls"
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestDBMiddleware_OnNewConnection(t *testing.T) {
	testCert, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil)
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
				onExpiredCert: func(context.Context) (tls.Certificate, error) {
					hasCalledOnExpiredCert = true
					return tls.Certificate{}, nil
				},
				logger:  slog.With(teleport.ComponentKey, "middleware"),
				dbRoute: tt.dbRoute,
			}

			localProxy, err := alpn.NewLocalProxy(alpn.LocalProxyConfig{
				RemoteProxyAddr: "localhost",
				Protocols:       []alpncommon.Protocol{alpncommon.ProtocolHTTP},
				ParentContext:   ctx,
				Clock:           tt.clock,
			})
			require.NoError(t, err)

			localProxy.SetCert(tlsCert)

			err = middleware.OnNewConnection(ctx, localProxy)
			tt.expectation(t, err, hasCalledOnExpiredCert)
		})
	}
}
