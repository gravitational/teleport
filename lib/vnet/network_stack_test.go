// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
)

// testIPv6Prefix is a fixed ULA prefix used in probe tests so the expected probe address is deterministic.
var testIPv6Prefix = tcpip.AddrFrom16([16]byte{0xfd, 0xec, 0x1f, 0xed, 0x13, 0x9f})

// testProbeIPv6 is the IPv6 probe address returned by ResolveAAAA for diagnostic queries
var testProbeIPv6 = [16]byte{0xfd, 0xec, 0x1f, 0xed, 0x13, 0x9f, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}

// testProbeIPv4 is the IPv4 probe address returned by ResolveA for diagnostic queries
var testProbeIPv4 = [4]byte{100, 64, 0, 2}

// TestResolveADiagProbeNoIPv4 verifies that A queries for probe names return NODATA
// when diagProbeIPv4 has not been set yet.
func TestResolveADiagProbeNoIPv4(t *testing.T) {
	ns := &networkStack{
		state:         newState(),
		ipv6Prefix:    testIPv6Prefix,
		diagProbeIPv6: testProbeIPv6,
	}

	const probeFQDN = "vnet-diag-abcdef.company.test."

	result, err := ns.ResolveA(t.Context(), probeFQDN)
	require.NoError(t, err)
	require.True(t, result.NoRecord, "A query for probe must return NoRecord (NODATA)")
	require.Equal(t, [4]byte{}, result.A, "probe A query must not return any address")
	require.Empty(t, ns.state.assignedIPs, "probe query must not mutate assignedIPs")
	require.Empty(t, ns.state.tcpHandlers, "probe query must not create a TCP handler")
}

// TestResolveADiagProbeWithIPv4 verifies that A queries for probe names return the
// stashed diagProbeIPv4 once it has been set.
func TestResolveADiagProbeWithIPv4(t *testing.T) {
	ns := &networkStack{
		state:         newState(),
		ipv6Prefix:    testIPv6Prefix,
		diagProbeIPv6: testProbeIPv6,
	}
	ns.diagProbeIPv4.Store(&testProbeIPv4)

	const probeFQDN = "vnet-diag-abcdef.company.test."

	result, err := ns.ResolveA(t.Context(), probeFQDN)
	require.NoError(t, err)
	require.False(t, result.NoRecord, "A query for probe must return an answer when diagProbeIPv4 is set")
	require.Equal(t, testProbeIPv4, result.A, "A query for probe must return diagProbeIPv4")
	require.Empty(t, ns.state.assignedIPs, "probe query must not mutate assignedIPs")
	require.Empty(t, ns.state.tcpHandlers, "probe query must not create a TCP handler")
}

func TestResolveAAAADiagProbe(t *testing.T) {
	ns := &networkStack{
		state:         newState(),
		ipv6Prefix:    testIPv6Prefix,
		diagProbeIPv6: testProbeIPv6,
	}

	const probeFQDN = "vnet-diag-abcdef.company.test."

	result, err := ns.ResolveAAAA(t.Context(), probeFQDN)
	require.NoError(t, err)
	require.Equal(t, testProbeIPv6, result.AAAA, "AAAA query for probe must return diagProbeIPv6")
	require.Equal(t, [4]byte{}, result.A, "AAAA result must not include an A record")
	require.Empty(t, ns.state.assignedIPs, "probe query must not mutate assignedIPs")
	require.Empty(t, ns.state.tcpHandlers, "probe query must not create a TCP handler")
}

func TestResolveAAAADiagProbeCaseInsensitive(t *testing.T) {
	ns := &networkStack{
		state:         newState(),
		ipv6Prefix:    testIPv6Prefix,
		diagProbeIPv6: testProbeIPv6,
	}

	for _, fqdn := range []string{
		"VNET-DIAG-abc.company.test.",
		"Vnet-Diag-abc.company.test.",
		"vNeT-dIaG-abc.company.test.",
	} {
		result, err := ns.ResolveAAAA(t.Context(), fqdn)
		require.NoError(t, err, fqdn)
		require.Equal(t, testProbeIPv6, result.AAAA, fqdn)
	}
	require.Empty(t, ns.state.assignedIPs)
}
