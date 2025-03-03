/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package insecure

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/fixtures"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestVerifyALPNUpgradedConn(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServer(t)
	proxy, err := auth.NewServerIdentity(srv.Auth(), "test-proxy", types.RoleProxy)
	require.NoError(t, err)

	tests := []struct {
		name       string
		serverCert []byte
		clock      clockwork.Clock
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "proxy verified",
			serverCert: proxy.TLSCertBytes,
			clock:      srv.Clock(),
			checkError: require.NoError,
		},
		{
			name:       "proxy expired",
			serverCert: proxy.TLSCertBytes,
			clock:      clockwork.NewFakeClockAt(srv.Clock().Now().Add(defaults.CATTL + time.Hour)),
			checkError: require.Error,
		},
		{
			name:       "not proxy",
			serverCert: []byte(fixtures.TLSCACertPEM),
			clock:      srv.Clock(),
			checkError: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverCert, err := utils.ReadCertificates(test.serverCert)
			require.NoError(t, err)

			test.checkError(t, verifyALPNUpgradedConn(test.clock)(tls.ConnectionState{
				PeerCertificates: serverCert,
			}))
		})
	}
}

func newTestTLSServer(t testing.TB) *auth.TestTLSServer {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func TestBuildTLSConfig(t *testing.T) {
	tests := []struct {
		name           string
		proxyAddr      string
		alpnEnvValue   bool
		insecure       bool
		expectInsecure bool
	}{
		{
			name:           "insecure",
			proxyAddr:      "localhost:1234",
			alpnEnvValue:   false,
			insecure:       true,
			expectInsecure: true,
		},
		{
			name:           "secure",
			proxyAddr:      "localhost:1234",
			alpnEnvValue:   false,
			insecure:       false,
			expectInsecure: false,
		},
		{
			name:           "alpn upgrade required",
			proxyAddr:      "localhost:1234",
			alpnEnvValue:   true,
			insecure:       false,
			expectInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(apidefaults.TLSRoutingConnUpgradeEnvVar, strconv.FormatBool(tt.alpnEnvValue))
			tlsConfig, alpnUpgrade := buildTLSConfig(context.Background(), ConnectionConfig{
				ProxyServer: tt.proxyAddr,
				Insecure:    tt.insecure,
				Log:         slog.Default(),
				Clock:       clockwork.NewRealClock(),
			})
			require.Equal(t, alpnUpgrade, tt.alpnEnvValue)
			require.Equal(t, tlsConfig.InsecureSkipVerify, tt.expectInsecure)
			require.Contains(t, tlsConfig.NextProtos, string(common.ProtocolProxyGRPCInsecure))
		})
	}
}
