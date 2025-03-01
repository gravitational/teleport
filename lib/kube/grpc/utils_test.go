/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package kubev1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWebAddrAndkubeSNI(t *testing.T) {
	const localKubeSNI = "kube-teleport-proxy-alpn.teleport.cluster.local"
	tests := []struct {
		name      string
		proxyAddr string
		sni       string
		host      string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "empty proxy address",
			proxyAddr: "",
			assertErr: require.Error,
		},
		{
			name:      "invalid address format",
			proxyAddr: "not-a-valid-addr",
			assertErr: require.Error,
		},
		{
			name:      "invalid port format",
			proxyAddr: ":not-a-valid-port",
			assertErr: require.Error,
		},
		{
			name:      "valid localhost address",
			proxyAddr: "localhost:3000",
			sni:       localKubeSNI,
			host:      "https://localhost:3000",
			assertErr: require.NoError,
		},
		{
			name:      "valid ip address",
			proxyAddr: "1.2.3.4:3000",
			sni:       localKubeSNI,
			host:      "https://1.2.3.4:3000",
			assertErr: require.NoError,
		},
		{
			name:      "valid wildcard address",
			proxyAddr: "0.0.0.0:3000",
			sni:       localKubeSNI,
			host:      "https://localhost:3000",
			assertErr: require.NoError,
		},
		{
			name:      "specify port only",
			proxyAddr: ":3000",
			sni:       localKubeSNI,
			host:      "https://localhost:3000",
			assertErr: require.NoError,
		},
		{
			name:      "double colons in address",
			proxyAddr: "::3000",
			assertErr: require.Error,
		},
		{
			name:      "valid ipv6 address",
			proxyAddr: "[::1]:3000",
			sni:       localKubeSNI,
			host:      "https://[::1]:3000",
			assertErr: require.NoError,
		},
		{
			name:      "unspecified ipv6 address",
			proxyAddr: "[::]:3000",
			sni:       localKubeSNI,
			host:      "https://localhost:3000",
			assertErr: require.NoError,
		},
		{
			name:      "ipv6 address without port",
			proxyAddr: "[::1]",
			assertErr: require.Error,
		},
		{
			name:      "valid domain address",
			proxyAddr: "ci.goteleport.com:3000",
			sni:       "kube-teleport-proxy-alpn.ci.goteleport.com",
			host:      "https://ci.goteleport.com:3000",
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSNI, gotHost, err := getWebAddrAndKubeSNI(tt.proxyAddr)
			tt.assertErr(t, err)
			require.Equal(t, tt.sni, gotSNI)
			require.Equal(t, tt.host, gotHost)
		})
	}
}
