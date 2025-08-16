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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindMatchingProxyDNS(t *testing.T) {
	tests := []struct {
		name             string
		fqdn             string
		proxyPublicAddrs []string
		expected         string
	}{
		{
			name:             "Exact match",
			fqdn:             "proxy.example.com",
			proxyPublicAddrs: []string{"proxy.example.com"},
			expected:         "proxy.example.com",
		},
		{
			name:             "Tail match",
			fqdn:             "app.proxy.example.com",
			proxyPublicAddrs: []string{"proxy.example.com"},
			expected:         "proxy.example.com",
		},
		{
			name:             "Multiple Proxy public addrs",
			fqdn:             "app.proxy.example.com",
			proxyPublicAddrs: []string{"other.example.com", "proxy.example.com"},
			expected:         "proxy.example.com",
		},
		{
			name:             "Multiple Proxy public addrs with port",
			fqdn:             "app.proxy.example.com",
			proxyPublicAddrs: []string{"other.example.com:3080", "proxy.example.com:3080"},
			expected:         "proxy.example.com:3080",
		},
		{
			name:             "No match returns first proxy public addrs",
			fqdn:             "nonexistent.domain.com",
			proxyPublicAddrs: []string{"proxy.example.com", "other.example.com"},
			expected:         "proxy.example.com",
		},
		{
			name:             "Empty FQDN returns empty string",
			fqdn:             "",
			proxyPublicAddrs: []string{"proxy.example.com"},
			expected:         "",
		},
		{
			name:             "Empty proxy list returns empty string",
			fqdn:             "some.domain.com",
			proxyPublicAddrs: []string{},
			expected:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindMatchingProxyDNS(tt.fqdn, tt.proxyPublicAddrs)
			require.Equal(t, tt.expected, result)
		})
	}
}
