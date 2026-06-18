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
		role: scopedaccessv1.ScopedRole_builder{
			Metadata: headerv1.Metadata_builder{Name: "test-role"}.Build(),
			Scope:    "/test",
			Spec:     spec,
			Version:  types.V1,
		}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "10m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        10 * time.Minute,
		},
		{
			name: "ssh timeout less restrictive than global is ignored",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "2h",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "defaults timeout used when ssh timeout absent",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "15m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        15 * time.Minute,
		},
		{
			name: "ssh timeout overrides defaults timeout",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "5m",
				}.Build(),
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "20m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			// ssh (20m) overrides defaults (5m), and 20m < 30m so 20m wins
			expect: 20 * time.Minute,
		},
		{
			name: "ssh timeout overrides defaults even when defaults is more restrictive",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "5m",
				}.Build(),
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					// SSH block explicitly overrides defaults with a less restrictive value;
					// the SSH block takes precedence, and since 25m < 30m global it still applies.
					ClientIdleTimeout: "25m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        25 * time.Minute,
		},
		{
			name: "role timeout applied when global is unlimited (zero)",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "1h",
				}.Build(),
			}.Build(),
			globalTimeout: 0, // zero means no global limit
			expect:        time.Hour,
		},
		{
			name: "empty ssh timeout defers to global",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "invalid duration string returns error",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "not-a-duration",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expectErr:     true,
		},
		{
			name: "zero duration string defers to global",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "0s",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "various valid duration formats",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ClientIdleTimeout: "1h30m",
				}.Build(),
			}.Build(),
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

func TestSSHAccessCheckerAdjustDisconnectExpiredCert(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		spec       *scopedaccessv1.ScopedRoleSpec
		defaultVal bool
		expect     bool
	}{
		{
			name: "unset defers to default false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			defaultVal: false,
			expect:     false,
		},
		{
			name: "unset defers to default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			defaultVal: true,
			expect:     true,
		},
		{
			name: "explicit true overrides default false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					DisconnectExpiredCert: ptr(true),
				}.Build(),
			}.Build(),
			defaultVal: false,
			expect:     true,
		},
		{
			name: "explicit false overrides default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
		{
			name: "unset ssh block defaults to default block",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.AdjustDisconnectExpiredCert(tt.defaultVal))
		})
	}
}

func TestSSHAccessCheckerLockingMode(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name        string
		spec        *scopedaccessv1.ScopedRoleSpec
		defaultMode constants.LockingMode
		expect      constants.LockingMode
	}{
		{
			name: "unset defers to default",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "strict from role",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					Lock: scopedaccessv1.Lock_builder{
						Mode: string(constants.LockingModeStrict),
					}.Build(),
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeStrict,
		},
		{
			name: "best effort from role",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					Lock: scopedaccessv1.Lock_builder{
						Mode: string(constants.LockingModeBestEffort),
					}.Build(),
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "invalid value falls back to default",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{

					Lock: scopedaccessv1.Lock_builder{
						Mode: "invalid",
					}.Build(),
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeStrict,
		},
		{
			name: "empty mode falls back to default",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{

					Lock: &scopedaccessv1.Lock{},
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeStrict,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.LockingMode(tt.defaultMode))
		})
	}
}

func TestSSHAccessCheckerPermitX11Forwarding(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect bool
	}{
		{
			name: "nil ssh block defaults to false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			expect: false,
		},
		{
			name: "set true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PermitX11Forwarding: ptr(true),
				}.Build(),
			}.Build(),
			expect: true,
		},
		{
			name: "set false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PermitX11Forwarding: ptr(false),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			expectErr: true,
		},
		{
			name: "set true allows",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ForwardAgent: ptr(true),
				}.Build(),
			}.Build(),
			expectErr: false,
		},
		{
			name: "set false denies",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					ForwardAgent: ptr(false),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					FileCopy: nil,
				}.Build(),
			}.Build(),
			expect: true,
		},
		{
			name: "true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					FileCopy: ptr(true),
				}.Build(),
			}.Build(),
			expect: true,
		},
		{
			name: "false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					FileCopy: ptr(false),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: nil,
				}.Build(),
			}.Build(),
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name: "both enabled",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
						Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: ptr(true)}.Build(),
						Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: ptr(true)}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON,
		},
		{
			name: "both disabled",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
						Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: ptr(false)}.Build(),
						Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: ptr(false)}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF,
		},
		{
			name: "local enabled remote disabled",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
						Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: ptr(true)}.Build(),
						Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: ptr(false)}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL,
		},
		{
			name: "local disabled remote enabled",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
						Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: ptr(false)}.Build(),
						Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: ptr(true)}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_REMOTE,
		},
		{
			name: "local and remote enabled not set defaults to ON",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
						Local:  &scopedaccessv1.SSHLocalPortForwarding{},
						Remote: &scopedaccessv1.SSHRemotePortForwarding{},
					}.Build(),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: &scopedaccessv1.ScopedRoleSSH{},
			}.Build(),
			expect: 0,
		},
		{
			name: "set to 5",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					MaxSessions: ptr(int64(5)),
				}.Build(),
			}.Build(),
			expect: 5,
		},
		{
			name: "set to 0",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					MaxSessions: ptr(int64(0)),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostSudoers: nil,
				}.Build(),
			}.Build(),
			expect: nil,
		},
		{
			name: "has sudoers",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostSudoers: []string{"ALL=(ALL) NOPASSWD: ALL"},
				}.Build(),
			}.Build(),
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

func TestSSHAccessCheckerSessionRecordingMode(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect constants.SessionRecordingMode
	}{
		{
			name:   "unset defaults to best_effort",
			spec:   scopedaccessv1.ScopedRoleSpec_builder{Ssh: &scopedaccessv1.ScopedRoleSSH{}}.Build(),
			expect: constants.SessionRecordingModeBestEffort,
		},
		{
			name: "ssh strict",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					SessionRecording: scopedaccessv1.SessionRecording_builder{
						Mode: string(constants.SessionRecordingModeStrict),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: constants.SessionRecordingModeStrict,
		},
		{
			name: "defaults used when ssh unset",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					SessionRecording: scopedaccessv1.SessionRecording_builder{
						Mode: string(constants.SessionRecordingModeStrict),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: constants.SessionRecordingModeStrict,
		},
		{
			name: "ssh overrides defaults",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					SessionRecording: scopedaccessv1.SessionRecording_builder{
						Mode: string(constants.SessionRecordingModeBestEffort),
					}.Build(),
				}.Build(),
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					SessionRecording: scopedaccessv1.SessionRecording_builder{
						Mode: string(constants.SessionRecordingModeBestEffort),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: constants.SessionRecordingModeBestEffort,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.SessionRecordingMode())
		})
	}
}

func TestSSHAccessCheckerEnhancedRecording(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		spec   *scopedaccessv1.ScopedRoleSpec
		expect map[string]bool
	}{
		{
			name: "no events",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					EnhancedRecording: nil,
				}.Build(),
			}.Build(),
			expect: map[string]bool{},
		},
		{
			name: "has events",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					EnhancedRecording: scopedaccessv1.EnhancedRecording_builder{
						Network: ptr(true),
						Command: ptr(true),
						Disk:    ptr(true),
					}.Build(),
				}.Build(),
			}.Build(),
			expect: map[string]bool{"command": true, "network": true, "disk": true},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).SSH()
			require.Equal(t, tt.expect, checker.EnhancedRecordingSet())
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: nil,
				}.Build(),
			}.Build(),
			expectNil: true,
		},
		{
			name: "empty mode string - denied",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode: "",
					}.Build(),
				}.Build(),
			}.Build(),
			expectNil: true,
		},
		{
			name: "mode off - denied",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode: "off",
					}.Build(),
				}.Build(),
			}.Build(),
			expectNil: true,
		},
		{
			name: "mode keep",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode:   "keep",
						Groups: []string{"test"},
						Shell:  "/bin/bash",
					}.Build(),
				}.Build(),
			}.Build(),
			expectNil:    false,
			expectMode:   decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
			expectGroups: []string{"test"},
			expectShell:  "/bin/bash",
		},
		{
			name: "mode insecure-drop",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode:   "insecure-drop",
						Groups: []string{"test"},
					}.Build(),
				}.Build(),
			}.Build(),
			expectNil:    false,
			expectMode:   decisionpb.HostUserMode_HOST_USER_MODE_DROP,
			expectGroups: []string{"test"},
		},
		{
			name: "uid and gid from traits",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode: "keep",
					}.Build(),
				}.Build(),
			}.Build(),
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
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Ssh: scopedaccessv1.ScopedRoleSSH_builder{
					HostUserCreation: scopedaccessv1.CreateHostUser_builder{
						Mode: "invalid",
					}.Build(),
				}.Build(),
			}.Build(),
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
			require.Equal(t, tt.expectMode, decision.Info.GetMode())
			require.Equal(t, tt.expectGroups, decision.Info.GetGroups())
			require.Equal(t, tt.expectShell, decision.Info.GetShell())
			require.Equal(t, tt.expectUID, decision.Info.GetUid())
			require.Equal(t, tt.expectGID, decision.Info.GetGid())
		})
	}
}
