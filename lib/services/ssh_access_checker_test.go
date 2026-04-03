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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
)

// newScopedCheckerWithRole is a test helper that builds a minimal ScopedAccessChecker
// with a role whose Spec is set to the provided value.
func newScopedCheckerWithRole(spec *scopedaccessv1.ScopedRoleSpec) *ScopedAccessChecker {
	return &ScopedAccessChecker{
		role: &scopedaccessv1.ScopedRole{
			Metadata: &headerv1.Metadata{Name: "test-role"},
			Scope:    "/test",
			Spec:     spec,
			Version:  types.V1,
		},
		// scopedCompatChecker is nil; tests that don't exercise delegation paths don't need it.
	}
}

// TestSSHAccessCheckerAdjustClientIdleTimeout verifies the idle timeout selection logic:
// ssh.client_idle_timeout takes precedence over defaults.client_idle_timeout, either value
// is only applied when it is more restrictive than the supplied global default, and invalid
// or empty values defer to the global default.
func TestSSHAccessCheckerAdjustClientIdleTimeout(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name          string
		spec          *scopedaccessv1.ScopedRoleSpec
		globalTimeout time.Duration
		expect        time.Duration
		expectErr     bool
	}{
		{
			name:          "no timeout set defers to global",
			spec:          &scopedaccessv1.ScopedRoleSpec{},
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "ssh timeout more restrictive than global",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "10m",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        10 * time.Minute,
		},
		{
			name: "ssh timeout less restrictive than global is ignored",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "2h",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "defaults timeout used when ssh timeout absent",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Defaults: &scopedaccessv1.ScopedRoleDefaults{
					ClientIdleTimeout: "15m",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        15 * time.Minute,
		},
		{
			name: "ssh timeout overrides defaults timeout",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Defaults: &scopedaccessv1.ScopedRoleDefaults{
					ClientIdleTimeout: "5m",
				},
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "20m",
				},
			},
			globalTimeout: 30 * time.Minute,
			// ssh (20m) overrides defaults (5m), and 20m < 30m so 20m wins
			expect: 20 * time.Minute,
		},
		{
			name: "ssh timeout overrides defaults even when defaults is more restrictive",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Defaults: &scopedaccessv1.ScopedRoleDefaults{
					ClientIdleTimeout: "5m",
				},
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					// SSH block explicitly overrides defaults with a less restrictive value;
					// the SSH block takes precedence, and since 25m < 30m global it still applies.
					ClientIdleTimeout: "25m",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        25 * time.Minute,
		},
		{
			name: "role timeout applied when global is unlimited (zero)",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "1h",
				},
			},
			globalTimeout: 0, // zero means no global limit
			expect:        time.Hour,
		},
		{
			name: "empty ssh timeout defers to global",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "invalid duration string returns error",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "not-a-duration",
				},
			},
			globalTimeout: 30 * time.Minute,
			expectErr:     true,
		},
		{
			name: "zero duration string defers to global",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "0s",
				},
			},
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "various valid duration formats",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ClientIdleTimeout: "1h30m",
				},
			},
			globalTimeout: 3 * time.Hour,
			expect:        time.Hour + 30*time.Minute,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			got, err := checker.AdjustClientIdleTimeout(tt.globalTimeout)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestSSHAccessCheckerX11Forwarding(t *testing.T) {

}
