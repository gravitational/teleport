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

package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzScopedAppPublicAddr(f *testing.F) {
	f.Add("grafana", "/staging/west", "grafana", "/prod")
	f.Add("some-app", "/test", "other-app", "/dev")
	f.Add("app", "/team", "app", "/team")

	const proxy = "proxy.example.com"
	f.Fuzz(func(t *testing.T, name1, scope1, name2, scope2 string) {
		addr := ScopedAppPublicAddr(scope1, name1, proxy)
		// same named apps and scopes should always pass
		if name1 == name2 && scope1 == scope2 {
			require.True(t, ScopedAppPublicAddrValid(scope2, name2, addr))
		} else {
			require.False(t, ScopedAppPublicAddrValid(scope2, name2, addr),
				"address for (%q, %q) must not validate for (%q, %q)", name1, scope1, name2, scope2)
		}
	})
}

func TestScopedAppPublicAddr(t *testing.T) {
	const proxy = "proxy.example.com"
	// This is the computed value for the app grafana on the /staging/west scope
	// Any changes to the ScopedAppPublicAddr function should also change the const value here.
	const addr = "r6rav2u7627smkmskj5ep2ttwizny3gn" + "." + proxy
	require.Equal(t, addr, ScopedAppPublicAddr("/staging/west", "grafana", proxy))

	// Distinct (name, scope) pairs hash to distinct labels.
	require.NotEqual(t, addr, ScopedAppPublicAddr("/prod", "grafana", proxy))
	require.NotEqual(t, addr, ScopedAppPublicAddr("/staging/west", "other", proxy))

	// A trailing port on the proxy is stripped and the host is lowercased.
	require.Equal(t, addr, ScopedAppPublicAddr("/staging/west", "grafana", "Proxy.Example.com:443"))
}

func TestVerifyScopedAppPublicAddr(t *testing.T) {
	const proxy = "teleport.example.com"
	validAddr := ScopedAppPublicAddr("/staging/west", "grafana", proxy)

	tests := []struct {
		name             string
		scope, app, addr string
		want             bool
	}{
		{
			name:  "matches",
			scope: "/staging/west",
			app:   "grafana",
			addr:  validAddr,
			want:  true,
		},
		{
			name:  "wrong scope",
			scope: "/prod",
			app:   "grafana",
			addr:  validAddr,
			want:  false,
		},
		{
			name:  "wrong name",
			scope: "/staging/west",
			app:   "other",
			addr:  validAddr,
			want:  false,
		},
		{
			name:  "empty addr",
			scope: "/staging/west",
			app:   "grafana",
			addr:  "",
			want:  false,
		},
		{
			name:  "plain app name, not a hash label",
			scope: "/staging/west",
			app:   "grafana",
			addr:  "grafana." + proxy,
			want:  false,
		},
		{
			name:  "without proxy",
			scope: "/staging/west",
			app:   "grafana",
			addr:  generateScopedSubDomain("grafana", "/staging/west"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ScopedAppPublicAddrValid(tt.scope, tt.app, tt.addr))
		})
	}
}
