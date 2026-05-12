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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

func TestKubeAccessCheckerAdjustDisconnectExpiredCert(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		spec       *scopedaccessv1.ScopedRoleSpec
		defaultVal bool
		expect     bool
	}{
		{
			name: "unset defers to default false",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			},
			defaultVal: false,
			expect:     false,
		},
		{
			name: "unset kube and default defers to default true",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			},
			defaultVal: true,
			expect:     true,
		},
		{
			name: "explicit true overrides default false",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{
					DisconnectExpiredCert: ptr(true),
				},
			},
			defaultVal: false,
			expect:     true,
		},
		{
			name: "explicit false overrides default true",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{
					DisconnectExpiredCert: ptr(false),
				},
			},
			defaultVal: true,
			expect:     false,
		},
		{
			name: "default block is applied when kube block is empty",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Defaults: &scopedaccessv1.ScopedRoleDefaults{
					DisconnectExpiredCert: ptr(false),
				},
				Kube: &scopedaccessv1.ScopedRoleKube{},
			},
			defaultVal: true,
			expect:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).Kube()
			require.Equal(t, tt.expect, checker.AdjustDisconnectExpiredCert(tt.defaultVal))
		})
	}
}

func TestKubeAccessCheckerLockingMode(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name        string
		spec        *scopedaccessv1.ScopedRoleSpec
		defaultMode constants.LockingMode
		expect      constants.LockingMode
	}{
		{
			name: "unset kube and default blocks defers to default mode",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			},
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "strict from role",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{
					Lock: &scopedaccessv1.Lock{
						Mode: string(constants.LockingModeStrict),
					},
				},
			},
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeStrict,
		},
		{
			name: "best effort from role",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{
					Lock: &scopedaccessv1.Lock{
						Mode: string(constants.LockingModeBestEffort),
					},
				},
			},
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "invalid value falls back to default",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Kube: &scopedaccessv1.ScopedRoleKube{

					Lock: &scopedaccessv1.Lock{
						Mode: "invalid",
					},
				},
			},
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeStrict,
		},
		{
			name: "empty mode falls back to default block",
			spec: &scopedaccessv1.ScopedRoleSpec{
				Defaults: &scopedaccessv1.ScopedRoleDefaults{
					Lock: &scopedaccessv1.Lock{
						Mode: string(constants.LockingModeStrict),
					},
				},
				Kube: &scopedaccessv1.ScopedRoleKube{
					Lock: nil,
				},
			},
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeStrict,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := newScopedCheckerWithRole(tt.spec).Kube()
			require.Equal(t, tt.expect, checker.LockingMode(tt.defaultMode))
		})
	}
}
