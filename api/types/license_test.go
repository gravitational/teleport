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
	tt := []struct {
		name        string
		setter      func(License, Bool)
		getter      func(License) Bool
		unsetValues [](func(License) Bool)
	}{
		{
			name:   "Set ReportsUsage",
			setter: License.SetReportsUsage,
			getter: License.GetReportsUsage,
			unsetValues: []func(License) Bool{
				License.GetCloud,
				License.GetSupportsKubernetes,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsDesktopAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Cloud",
			setter: License.SetCloud,
			getter: License.GetCloud,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetSupportsKubernetes,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsDesktopAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Kubernetes Support",
			setter: License.SetSupportsKubernetes,
			getter: License.GetSupportsKubernetes,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetCloud,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsDesktopAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Application Access Support",
			setter: License.SetSupportsApplicationAccess,
			getter: License.GetSupportsApplicationAccess,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetCloud,
				License.GetSupportsKubernetes,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsDesktopAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Database Access Support",
			setter: License.SetSupportsDatabaseAccess,
			getter: License.GetSupportsDatabaseAccess,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetCloud,
				License.GetSupportsKubernetes,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDesktopAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Desktop Access Support",
			setter: License.SetSupportsDesktopAccess,
			getter: License.GetSupportsDesktopAccess,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetCloud,
				License.GetSupportsKubernetes,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsModeratedSessions,
			},
		},
		{
			name:   "Set Moderated Sessions Support",
			setter: License.SetSupportsModeratedSessions,
			getter: License.GetSupportsModeratedSessions,
			unsetValues: []func(License) Bool{
				License.GetReportsUsage,
				License.GetCloud,
				License.GetSupportsKubernetes,
				License.GetSupportsApplicationAccess,
				License.GetSupportsDatabaseAccess,
				License.GetSupportsDesktopAccess,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			license := &LicenseV3{
				Spec: LicenseSpecV3{
					SupportsApplicationAccess: NewBoolP(false),
				},
			}
			tc.setter(license, true)
			require.True(t, bool(tc.getter(license)))
			for _, unset := range tc.unsetValues {
				require.False(t, bool(unset(license)))
			}
		})
	}

	// Manually test Application Access.
	// If unset application access is set to true by default.
	license := &LicenseV3{}
	require.True(t, bool(license.GetSupportsApplicationAccess()))
	require.False(t, bool(license.GetReportsUsage()))
	require.False(t, bool(license.GetCloud()))
	require.False(t, bool(license.GetSupportsKubernetes()))
	require.False(t, bool(license.GetSupportsDatabaseAccess()))
	require.False(t, bool(license.GetSupportsDesktopAccess()))
	require.False(t, bool(license.GetSupportsModeratedSessions()))
}
