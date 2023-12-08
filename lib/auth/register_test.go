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
