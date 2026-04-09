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

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
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

func ptr[T any](v T) *T { return &v }

func TestSSHAccessCheckerPermitX11Forwarding(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect bool
	}{
		{
			name: "nil ssh block defaults to false",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			},
			expect: false,
		},
		{
			name: "set true",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PermitX11Forwarding: ptr(true),
				},
			},
			expect: true,
		},
		{
			name: "set false",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PermitX11Forwarding: ptr(false),
				},
			},
			expect: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.PermitX11Forwarding())
		})
	}
}

func TestSSHAccessCheckerCheckAgentForward(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name      string
		spec      *scopedaccessv1.ScopedRoleSpec
		expectErr bool
	}{
		{
			name: "nil agent forwarding denies",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			},
			expectErr: true,
		},
		{
			name: "set true allows",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ForwardAgent: ptr(true),
				},
			},
			expectErr: false,
		},
		{
			name: "set false denies",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					ForwardAgent: ptr(false),
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			err := checker.CheckAgentForward("testuser")
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSSHAccessCheckerCanCopyFiles(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect bool
	}{
		{
			name: "nil file_copy defaults to true",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					FileCopy: nil,
				},
			},
			expect: true,
		},
		{
			name: "true",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					FileCopy: ptr(true),
				},
			},
			expect: true,
		},
		{
			name: "false",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					FileCopy: ptr(false),
				},
			},
			expect: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.CanCopyFiles())
		})
	}
}

func TestSSHAccessCheckerSSHPortForwardMode(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect decisionpb.SSHPortForwardMode
	}{
		{
			name: "nil port forwarding defaults to ON",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: nil,
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name: "both enabled",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: &scopedaccessv1.SSHPortForwarding{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{Enabled: ptr(true)},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{Enabled: ptr(true)},
					},
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name: "both disabled",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: &scopedaccessv1.SSHPortForwarding{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{Enabled: ptr(false)},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{Enabled: ptr(false)},
					},
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
		{
			name: "local enabled remote disabled",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: &scopedaccessv1.SSHPortForwarding{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{Enabled: ptr(true)},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{Enabled: ptr(false)},
					},
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
		},
		{
			name: "local disabled remote enabled",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: &scopedaccessv1.SSHPortForwarding{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{Enabled: ptr(false)},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{Enabled: ptr(true)},
					},
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_REMOTE,
		},
		{
			name: "local and remote enabled not set defaults to ON",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					PortForwarding: &scopedaccessv1.SSHPortForwarding{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{},
					},
				},
			},
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.SSHPortForwardMode())
		})
	}
}

func TestSSHAccessCheckerMaxSessions(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect int64
	}{
		{
			name: "not set defaults to 0",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			},
			expect: 0,
		},
		{
			name: "set to 5",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					MaxSessions: ptr(int64(5)),
				},
			},
			expect: 5,
		},
		{
			name: "set to 0",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					MaxSessions: ptr(int64(0)),
				},
			},
			expect: 0,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.MaxSessions())
		})
	}
}

func TestSSHAccessCheckerHostSudoers(t *testing.T) {
	t.Parallel()

	srv := &types.ServerV2{}

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect []string
	}{
		{
			name: "no sudoers",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostSudoers: nil,
				},
			},
			expect: nil,
		},
		{
			name: "has sudoers",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostSudoers: []string{"ALL=(ALL) NOPASSWD: ALL"},
				},
			},
			expect: []string{"ALL=(ALL) NOPASSWD: ALL"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			sudoers, err := checker.HostSudoers(srv)
			require.NoError(t, err)
			require.Equal(t, tt.expect, sudoers)
		})
	}
}

func TestSSHAccessCheckerHostUsers(t *testing.T) {
	t.Parallel()

	srv := &types.ServerV2{}

	tts := []struct {
		name         string
		spec         *scopedaccessv1.ScopedRoleSpec
		traits       wrappers.Traits
		expectNil    bool
		expectErr    bool
		expectMode   decisionpb.HostUserMode
		expectGroups []string
		expectShell  string
		expectUID    string
		expectGID    string
	}{
		{
			name: "nil host_user_creation - denied",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: nil,
				},
			},
			expectNil: true,
		},
		{
			name: "empty mode string - denied",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode: "",
					},
				},
			},
			expectNil: true,
		},
		{
			name: "mode off - denied",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode: "off",
					},
				},
			},
			expectNil: true,
		},
		{
			name: "mode keep",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode:   "keep",
						Groups: []string{"test"},
						Shell:  "/bin/bash",
					},
				},
			},
			expectNil:    false,
			expectMode:   decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
			expectGroups: []string{"test"},
			expectShell:  "/bin/bash",
		},
		{
			name: "mode insecure-drop",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode:   "insecure-drop",
						Groups: []string{"test"},
					},
				},
			},
			expectNil:    false,
			expectMode:   decisionpb.HostUserMode_HOST_USER_MODE_DROP,
			expectGroups: []string{"test"},
		},
		{
			name: "uid and gid from traits",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode: "keep",
					},
				},
			},
			traits: wrappers.Traits{
				constants.TraitHostUserUID: []string{"1001"},
				constants.TraitHostUserGID: []string{"1001"},
			},
			expectNil:  false,
			expectMode: decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
			expectUID:  "1001",
			expectGID:  "1001",
		},
		{
			name: "invalid mode returns error",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					HostUserCreation: &scopedaccessv1.CreateHostUser{
						Mode: "invalid",
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newScopedCheckerWithRole(tt.spec).SSH()
			checker.checker.scopedCompatChecker = newAccessChecker(&AccessInfo{
				Traits: tt.traits,
			}, "local", NewRoleSet())

			decision, err := checker.HostUsers(srv)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, decision)

			if tt.expectNil {
				require.Nil(t, decision.Info)
				require.NotEmpty(t, decision.DeniedBy)
				return
			}

			require.NotNil(t, decision.Info, "expected Info to be non-nil (allowed)")
			require.NotEmpty(t, decision.AllowedBy, "expected AllowedBy to be populated")
			require.Equal(t, tt.expectMode, decision.Info.Mode)
			require.Equal(t, tt.expectGroups, decision.Info.Groups)
			require.Equal(t, tt.expectShell, decision.Info.Shell)
			require.Equal(t, tt.expectUID, decision.Info.Uid)
			require.Equal(t, tt.expectGID, decision.Info.Gid)
		})
	}
}
