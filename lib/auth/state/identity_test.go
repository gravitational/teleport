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

	"github.com/stretchr/testify/require"
)

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
