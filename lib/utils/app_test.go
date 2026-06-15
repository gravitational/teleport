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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultAppFQDN(t *testing.T) {
	tests := []struct {
		name                string
		appName             string
		proxyPublicAddrHost string
		clusterName         string
		want                string
	}{
		{
			name:                "proxy host used directly",
			appName:             "app",
			proxyPublicAddrHost: "teleport.test",
			clusterName:         "cluster.example.com",
			want:                "app.teleport.test",
		},
		{
			name:                "proxy host wins over cluster name",
			appName:             "app",
			proxyPublicAddrHost: "10.0.0.5",
			clusterName:         "cluster.example.com",
			want:                "app.10.0.0.5",
		},
		{
			name:                "cluster name used when proxy host is empty",
			appName:             "app",
			proxyPublicAddrHost: "",
			clusterName:         "cluster.example.com",
			want:                "app.cluster.example.com",
		},
		{
			name:                "host is lowercased and port stripped",
			appName:             "app",
			proxyPublicAddrHost: "Teleport.Test:443",
			clusterName:         "",
			want:                "app.teleport.test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultAppFQDN(tt.appName, tt.proxyPublicAddrHost, tt.clusterName)
			require.Equal(t, tt.want, got)
		})
	}
}
