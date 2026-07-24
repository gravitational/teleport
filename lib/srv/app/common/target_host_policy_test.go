/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"net/netip"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestTargetHostPolicy(t *testing.T) {
	t.Parallel()

	allow := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	deny := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}}

	t.Run("enabled reflects configured restrictions", func(t *testing.T) {
		t.Parallel()
		require.False(t, TargetHostPolicy{}.Enabled())
		require.True(t, allow.Enabled())
		require.True(t, deny.Enabled())
	})

	t.Run("check rejects setting both lists", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, allow.Check())
		require.NoError(t, deny.Check())

		both := TargetHostPolicy{AllowedPrefixes: allow.AllowedPrefixes, DeniedPrefixes: deny.DeniedPrefixes}
		require.True(t, trace.IsBadParameter(both.Check()), "got %v", both.Check())
	})

	t.Run("allow list blocks non-members without a matched prefix", func(t *testing.T) {
		t.Parallel()
		require.False(t, allow.blocked(netip.MustParseAddr("10.1.2.3")))

		blockedAddr := netip.MustParseAddr("192.0.2.1")
		require.True(t, allow.blocked(blockedAddr))
		require.False(t, allow.deniedPrefix(blockedAddr).IsValid())
		require.Equal(t, targetHostPolicyAllow, allow.mode())
	})

	t.Run("deny list blocks members and reports the matched prefix", func(t *testing.T) {
		t.Parallel()
		blockedAddr := netip.MustParseAddr("169.254.169.254")
		require.True(t, deny.blocked(blockedAddr))
		require.Equal(t, "169.254.0.0/16", deny.deniedPrefix(blockedAddr).String())

		require.False(t, deny.blocked(netip.MustParseAddr("10.1.2.3")))
		require.Equal(t, targetHostPolicyDeny, deny.mode())
	})
}

func TestHTTPProxyConfiguredInEnv(t *testing.T) {
	// Neutralize any ambient proxy variables so the test is deterministic.
	// Not parallel: mutates the process environment.
	clearProxyEnv := func(t *testing.T) {
		for _, v := range httpProxyEnvVars {
			t.Setenv(v, "")
		}
	}

	t.Run("none set", func(t *testing.T) {
		clearProxyEnv(t)
		_, ok := HTTPProxyConfiguredInEnv()
		require.False(t, ok)
	})

	t.Run("HTTPS_PROXY set", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")
		name, ok := HTTPProxyConfiguredInEnv()
		require.True(t, ok)
		require.Equal(t, "HTTPS_PROXY", name)
	})

	t.Run("lowercase http_proxy set", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("http_proxy", "http://proxy.example.com:3128")
		name, ok := HTTPProxyConfiguredInEnv()
		require.True(t, ok)
		require.Equal(t, "http_proxy", name)
	})
}
