// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomFreeIPv4InNet(t *testing.T) {
	t.Parallel()

	cidr := "192.168.1.0/24"

	_, ipNet, err := net.ParseCIDR(cidr)
	require.NoError(t, err)

	// Total available IPs in the range excluding 0 and 1 suffixes which should not be assigned
	leadingOnes, totalBits := ipNet.Mask.Size()
	freeIPCount := 1<<(totalBits-leadingOnes) - 2

	assignedIPs := make(map[ipv4]struct{}, freeIPCount)
	ipIsFree := func(ip ipv4) bool {
		_, taken := assignedIPs[ip]
		return !taken
	}

	// Assign every free IP.
	for i := 0; i < freeIPCount; i++ {
		ip, err := randomFreeIPv4InNet(ipNet, ipIsFree)
		require.NoError(t, err)
		assignedIPs[ip] = struct{}{}
	}

	require.Len(t, assignedIPs, freeIPCount)

	// Try to assign 1 more IP.
	_, err = randomFreeIPv4InNet(ipNet, ipIsFree)
	require.ErrorContains(t, err, "Exhausted all IPs in range")
}
