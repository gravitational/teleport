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

	tts := []struct {
		name       string
		spec       *scopedaccessv1.ScopedRoleSpec
		defaultVal bool
		expect     bool
	}{
		{
			name: "unset defers to default false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			}.Build(),
			defaultVal: false,
			expect:     false,
		},
		{
			name: "unset kube and default defers to default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			}.Build(),
			defaultVal: true,
			expect:     true,
		},
		{
			name: "explicit true overrides default false",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: scopedaccessv1.ScopedRoleKube_builder{
					DisconnectExpiredCert: ptr(true),
				}.Build(),
			}.Build(),
			defaultVal: false,
			expect:     true,
		},
		{
			name: "explicit false overrides default true",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: scopedaccessv1.ScopedRoleKube_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
		{
			name: "default block is applied when kube block is empty",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					DisconnectExpiredCert: ptr(false),
				}.Build(),
				Kube: &scopedaccessv1.ScopedRoleKube{},
			}.Build(),
			defaultVal: true,
			expect:     false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			checker := newScopedCheckerWithRole(tt.spec).Kube()
			require.Equal(t, tt.expect, checker.AdjustDisconnectExpiredCert(tt.defaultVal))
		})
	}
}

func TestKubeAccessCheckerLockingMode(t *testing.T) {

	tts := []struct {
		name        string
		spec        *scopedaccessv1.ScopedRoleSpec
		defaultMode constants.LockingMode
		expect      constants.LockingMode
	}{
		{
			name: "unset kube and default blocks defers to default mode",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: &scopedaccessv1.ScopedRoleKube{},
			}.Build(),
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeBestEffort,
		},
		{
			name: "strict from role",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Kube: scopedaccessv1.ScopedRoleKube_builder{
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
				Kube: scopedaccessv1.ScopedRoleKube_builder{
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
				Kube: scopedaccessv1.ScopedRoleKube_builder{

					Lock: scopedaccessv1.Lock_builder{
						Mode: "invalid",
					}.Build(),
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeStrict,
			expect:      constants.LockingModeStrict,
		},
		{
			name: "empty mode falls back to default block",
			spec: scopedaccessv1.ScopedRoleSpec_builder{
				Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
					Lock: scopedaccessv1.Lock_builder{
						Mode: string(constants.LockingModeStrict),
					}.Build(),
				}.Build(),
				Kube: scopedaccessv1.ScopedRoleKube_builder{
					Lock: nil,
				}.Build(),
			}.Build(),
			defaultMode: constants.LockingModeBestEffort,
			expect:      constants.LockingModeStrict,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			checker := newScopedCheckerWithRole(tt.spec).Kube()
			require.Equal(t, tt.expect, checker.LockingMode(tt.defaultMode))
		})
	}
}
