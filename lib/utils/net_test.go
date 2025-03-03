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
)

func TestInferProxyPublicAddr(t *testing.T) {
	tests := []struct {
		name              string
		fqdn              string
		proxyDNSNames     []string
		defaultPublicAddr string
		expected          string
		expectErr         bool
	}{
		{
			name:              "Exact match",
			fqdn:              "proxy.example.com",
			proxyDNSNames:     []string{"proxy.example.com"},
			defaultPublicAddr: "default",
			expected:          "proxy.example.com",
		},
		{
			name:              "Tail match",
			fqdn:              "app.proxy.example.com",
			proxyDNSNames:     []string{"proxy.example.com"},
			defaultPublicAddr: "default",
			expected:          "proxy.example.com",
		},
		{
			name:              "No match returns default",
			fqdn:              "nonexistent.domain.com",
			proxyDNSNames:     []string{"proxy.example.com"},
			defaultPublicAddr: "default",
			expected:          "default",
		},
		{
			name:              "Empty FQDN returns default",
			fqdn:              "",
			proxyDNSNames:     []string{"proxy.example.com"},
			defaultPublicAddr: "default",
			expected:          "default",
		},
		{
			name:              "Empty proxy list returns default",
			fqdn:              "some.domain.com",
			proxyDNSNames:     []string{},
			defaultPublicAddr: "default",
			expected:          "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferProxyPublicAddr(tt.fqdn, tt.proxyDNSNames, tt.defaultPublicAddr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
