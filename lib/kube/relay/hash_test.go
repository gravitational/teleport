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

package relay

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodedHashLen(t *testing.T) {
	//nolint:staticcheck // the explicit type in the declaration is the entire
	// point of this statement
	var _ [hashLen]byte = hashForTarget("", "")

	//nolint:testifylint // the parameter order is deliberate,
	// b32hex.EncodedLen(hashLen) is the source of truth and we are testing the
	// value of the const
	require.Equal(t, base32hex.EncodedLen(hashLen), encodedHashLen)
}

func TestHashForTarget(t *testing.T) {
	testCases := []struct {
		teleportClusterName    string
		kubeClusterName        string
		hash                   string
		sniLabelForKubeCluster string
	}{
		{
			teleportClusterName:    "",
			kubeClusterName:        "",
			hash:                   "-\xba]\xbc3\x9es\x16\xae\xa2h?\xaf\x83\x9c\x1b{\x1e\xe21=\xb7\x92\x11%\x88\x11\x8d\xf0f\xaa5",
			sniLabelForKubeCluster: "cluster-5mt5rf1jjpphdbl2d0vqv0ss3dthtohh7mrp4495h08ors36l8qg",
		},
		{
			teleportClusterName:    "teleport.example.com",
			kubeClusterName:        "prod-1",
			hash:                   "%0\x86\x19\x99\x17\x97\xf9uX~\x00\xe4?\xf4aPO\v\xde%gq\xc7\xdf\xc6\x05|\xd1y\xd7r",
			sniLabelForKubeCluster: "cluster-4ko8c6cp2ubvitaofo0e8fvkc584u2uu4ljn3huvoo2npkbpqtp0",
		},

		{
			teleportClusterName:    "8701ae5eecf94a25adc7a90e3f373a25",
			kubeClusterName:        "3929312bc0c74cd491a882f7abd7edbd",
			hash:                   "\x0f\x12\xae\xf8\x83\xc2\xd3 j-\xe9\x87\xea\xdf\x18s\xd7\f\xb6\xcf\xcd\xce\x17\x89\x19c\r\xa2\x99\xd8A\xdd",
			sniLabelForKubeCluster: "cluster-1s9atu43ob9i0qhdt63ulnooefbgpdmfpn71f28pcc6q56eo87eg",
		},
	}

	for _, tc := range testCases {
		hft := hashForTarget(tc.teleportClusterName, tc.kubeClusterName)
		require.Equal(t, tc.hash, string(hft[:]))
		require.Equal(t, tc.sniLabelForKubeCluster, SNILabelForKubeCluster(tc.teleportClusterName, tc.kubeClusterName))
	}
}
