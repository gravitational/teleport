// Copyright 2024 Gravitational, Inc
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
