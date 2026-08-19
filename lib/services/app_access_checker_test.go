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
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

// TestAppAccessCheckerAdjustClientIdleTimeout verifies the idle timeout selection logic:
// <protocol>.client_idle_timeout takes precedence over defaults.client_idle_timeout, either value
// is only applied when it is more restrictive than the supplied global default, and invalid
// or empty values defer to the global default.
func TestAppAccessCheckerAdjustClientIdleTimeout(t *testing.T) {
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
			name: "app timeout more restrictive than global",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "10m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        10 * time.Minute,
		},
		{
			name: "app timeout less restrictive than global is ignored",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "2h",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "defaults timeout used when app timeout absent",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "15m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        15 * time.Minute,
		},
		{
			name: "app timeout overrides defaults timeout",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "5m",
				}.Build(),
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "20m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			// app (20m) overrides defaults (5m), and 20m < 30m so 20m wins
			expect: 20 * time.Minute,
		},
		{
			name: "app timeout overrides defaults even when defaults is more restrictive",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					ClientIdleTimeout: "5m",
				}.Build(),
				App: scopedaccessv1.ScopedRoleApp_builder{
					// App block explicitly overrides defaults with a less restrictive value;
					// the App block takes precedence, and since 25m < 30m global it still applies.
					ClientIdleTimeout: "25m",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        25 * time.Minute,
		},
		{
			name: "role timeout applied when global is unlimited (zero)",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "1h",
				}.Build(),
			}.Build(),
			globalTimeout: 0, // zero means no global limit
			expect:        time.Hour,
		},
		{
			name: "empty app timeout defers to global",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "invalid duration string returns error",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "not-a-duration",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expectErr:     true,
		},
		{
			name: "zero duration string defers to global",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					ClientIdleTimeout: "0s",
				}.Build(),
			}.Build(),
			globalTimeout: 30 * time.Minute,
			expect:        30 * time.Minute,
		},
		{
			name: "various valid duration formats",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
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
			checker := newScopedCheckerWithRole(tt.spec).App()
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

func TestAppAccessCheckerAdjustDisconnectExpiredCert(t *testing.T) {
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
				App: &scopedaccessv1.ScopedRoleApp{},
			}.Build(),
			defaultVal: false,
			expect:     false,
		},
		{
			name: "unset defers to default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: &scopedaccessv1.ScopedRoleApp{},
			}.Build(),
			defaultVal: true,
			expect:     true,
		},
		{
			name: "explicit true overrides default false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					DisconnectExpiredCert: ptr(true),
				}.Build(),
			}.Build(),
			defaultVal: false,
			expect:     true,
		},
		{
			name: "explicit false overrides default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
		{
			name: "unset app block defaults to default block",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
				App: &scopedaccessv1.ScopedRoleApp{},
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).App()
			require.Equal(t, tt.expect, checker.AdjustDisconnectExpiredCert(tt.defaultVal))
		})
	}
}

func TestAppAccessCheckerLockingMode(t *testing.T) {
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
				App: &scopedaccessv1.ScopedRoleApp{},
			}.Build(),
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "strict from role",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				App: scopedaccessv1.ScopedRoleApp_builder{
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
				App: scopedaccessv1.ScopedRoleApp_builder{
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
				App: scopedaccessv1.ScopedRoleApp_builder{
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
				App: scopedaccessv1.ScopedRoleApp_builder{

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
			checker := newScopedCheckerWithRole(tt.spec).App()
			require.Equal(t, tt.expect, checker.LockingMode(tt.defaultMode))
		})
	}
}
