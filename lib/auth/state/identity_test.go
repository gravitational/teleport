// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestReadTLSIdentityFromKeyPairAgentPin verifies that ReadTLSIdentityFromKeyPair correctly handles
// agent pin TLS certs.
func TestReadTLSIdentityFromKeyPairAgentPin(t *testing.T) {
	clock := clockwork.NewFakeClock()

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	keyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)
	caCertPEM := []byte(fixtures.TLSCACertPEM)

	tests := []struct {
		desc            string
		pin             *scopesv1.Pin
		wantRole        types.SystemRole
		wantSystemRoles []string
	}{
		{
			desc: "single-role agent pin",
			pin: &scopesv1.Pin{
				Kind:  scopesv1.PinKind_PIN_KIND_AGENT,
				Scope: "/staging",
				SystemRoles: &scopesv1.SystemRoles{
					Primary: string(types.RoleNode),
				},
			},
			wantRole: types.RoleNode,
		},
		{
			desc: "multi-role instance agent pin",
			pin: &scopesv1.Pin{
				Kind:  scopesv1.PinKind_PIN_KIND_AGENT,
				Scope: "/staging",
				SystemRoles: &scopesv1.SystemRoles{
					Primary:    string(types.RoleInstance),
					Additional: []string{string(types.RoleNode), string(types.RoleKube)},
				},
			},
			wantRole:        types.RoleInstance,
			wantSystemRoles: []string{string(types.RoleNode), string(types.RoleKube)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tlsID := tlsca.Identity{
				Username: "node-uuid.test-cluster",
				ScopePin: tt.pin,
			}
			subj, err := tlsID.Subject()
			require.NoError(t, err)

			certPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
				Clock:     clock,
				PublicKey: key.Public(),
				Subject:   subj,
				NotAfter:  clock.Now().Add(time.Hour),
			})
			require.NoError(t, err)

			identity, err := ReadTLSIdentityFromKeyPair(keyPEM, certPEM, [][]byte{caCertPEM})
			require.NoError(t, err)

			require.Equal(t, tt.wantRole, identity.ID.Role)
			require.Equal(t, tt.wantSystemRoles, identity.SystemRoles)
			require.True(t, identity.HasSystemRole(tt.wantRole))
		})
	}
}

func TestHasDNSNames(t *testing.T) {
	require.False(t, (&Identity{XCert: nil}).HasDNSNames(nil))

	type testCase struct {
		dnsNames    []string
		ipAddresses []net.IP

		requested []string
		check     require.BoolAssertionFunc
	}

	testCases := []testCase{
		{
			dnsNames:    []string{},
			ipAddresses: []net.IP{},
			requested:   []string{},
			check:       require.True,
		},
		{
			dnsNames:    nil,
			ipAddresses: nil,
			requested:   []string{},
			check:       require.True,
		},
		{
			dnsNames:    []string{"foo", "bar"},
			ipAddresses: []net.IP{},
			requested:   []string{"foo"},
			check:       require.True,
		},
		{
			dnsNames:    []string{"foo", "bar"},
			ipAddresses: []net.IP{},
			requested:   []string{"foo", "1.2.3.4"},
			check:       require.False,
		},
		{
			dnsNames:    []string{"foo", "bar"},
			ipAddresses: []net.IP{net.IPv4(1, 2, 3, 4)},
			requested:   []string{"foo", "1.2.3.4"},
			check:       require.True,
		},
	}
	for _, tc := range testCases {
		identity := &Identity{
			XCert: &x509.Certificate{
				DNSNames:    tc.dnsNames,
				IPAddresses: tc.ipAddresses,
			},
		}
		tc.check(t, identity.HasDNSNames(tc.requested))
	}

}
