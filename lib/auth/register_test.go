// Copyright 2023 Gravitational, Inc
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

package auth

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
)

func TestVerifyALPNUpgradedConn(t *testing.T) {
	t.Parallel()

	auth := newTestTLSServer(t)
	proxy, err := NewServerIdentity(auth.Auth(), "test-proxy", types.RoleProxy)
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
			clock:      auth.Clock(),
			checkError: require.NoError,
		},
		{
			name:       "proxy expired",
			serverCert: proxy.TLSCertBytes,
			clock:      clockwork.NewFakeClockAt(auth.Clock().Now().Add(defaults.CATTL + time.Hour)),
			checkError: require.Error,
		},
		{
			name:       "not proxy",
			serverCert: []byte(fixtures.TLSCACertPEM),
			clock:      auth.Clock(),
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
