/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLicenseSettersAndGetters(t *testing.T) {
	license := LicenseV3{}

	l := func() License {
		return &license
	}

	reset := func() {
		license = LicenseV3{}
		fv := Bool(false)
		// Manually disble application access because it will be set
		// to true by default if the value is unset.
		license.Spec.SupportsApplicationAccess = &fv
	}

	tt := []struct {
		name        string
		setter      func(Bool)
		getter      func() Bool
		unsetValues [](func() Bool)
	}{
		{
			name:   "Set ReportsUsage",
			setter: l().SetReportsUsage,
			getter: l().GetReportsUsage,
			unsetValues: []func() Bool{
				l().GetCloud,
				l().GetSupportsKubernetes,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsDesktopAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Cloud",
			setter: l().SetCloud,
			getter: l().GetCloud,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetSupportsKubernetes,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsDesktopAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Kubernetes Support",
			setter: l().SetSupportsKubernetes,
			getter: l().GetSupportsKubernetes,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetCloud,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsDesktopAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Application Access Support",
			setter: l().SetSupportsApplicationAccess,
			getter: l().GetSupportsApplicationAccess,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetCloud,
				l().GetSupportsKubernetes,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsDesktopAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Database Access Support",
			setter: l().SetSupportsDatabaseAccess,
			getter: l().GetSupportsDatabaseAccess,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetCloud,
				l().GetSupportsKubernetes,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDesktopAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Desktop Access Support",
			setter: l().SetSupportsDesktopAccess,
			getter: l().GetSupportsDesktopAccess,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetCloud,
				l().GetSupportsKubernetes,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Moderated Sessions Support",
			setter: l().SetSupportsModeratedSessions,
			getter: l().GetSupportsModeratedSessions,
			unsetValues: []func() Bool{
				l().GetReportsUsage,
				l().GetCloud,
				l().GetSupportsKubernetes,
				l().GetSupportsApplicationAccess,
				l().GetSupportsDatabaseAccess,
				l().GetSupportsDesktopAccess,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			tc.setter(true)
			require.True(t, bool(tc.getter()))
			for _, unset := range tc.unsetValues {
				require.False(t, bool(unset()))
			}
		})
	}

	// Manually test Application Access.
	// If unset application access is set to true by default.
	license = LicenseV3{}
	require.True(t, bool(l().GetSupportsApplicationAccess()))
	require.False(t, bool(l().GetReportsUsage()))
	require.False(t, bool(l().GetCloud()))
	require.False(t, bool(l().GetSupportsKubernetes()))
	require.False(t, bool(l().GetSupportsDatabaseAccess()))
	require.False(t, bool(l().GetSupportsDesktopAccess()))
	require.False(t, bool(l().GetSupportsModeratedSessions()))
}
