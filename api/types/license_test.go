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
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLicenseSettersAndGetters(t *testing.T) {
	// All getters inspected in this test.
	allFields := []func(License) Bool{
		License.GetReportsUsage,
		License.GetSalesCenterReporting,
		License.GetCloud,
		License.GetSupportsKubernetes,
		License.GetSupportsApplicationAccess,
		License.GetSupportsDatabaseAccess,
		License.GetSupportsDesktopAccess,
		License.GetSupportsModeratedSessions,
		License.GetSupportsMachineID,
		License.GetSupportsResourceAccessRequests,
		License.GetTrial,
	}

	// unsetFields returns a list of license fields getters minus
	// the one passed as an argument.
	unsetFields := func(getter func(License) Bool) []func(License) Bool {
		var unsetFields []func(License) Bool
		for _, fieldGetter := range allFields {
			if fnName(fieldGetter) != fnName(getter) {
				unsetFields = append(unsetFields, fieldGetter)
			}
		}
		return unsetFields
	}

	tt := []struct {
		name        string
		setter      func(License, Bool)
		getter      func(License) Bool
		unsetFields [](func(License) Bool)
	}{
		{
			name:        "Set ReportsUsage",
			setter:      License.SetReportsUsage,
			getter:      License.GetReportsUsage,
			unsetFields: unsetFields(License.GetReportsUsage),
		},
		{
			name:        "Set SalesCenterReporting",
			setter:      License.SetSalesCenterReporting,
			getter:      License.GetSalesCenterReporting,
			unsetFields: unsetFields(License.GetSalesCenterReporting),
		},
		{
			name:        "Set Cloud",
			setter:      License.SetCloud,
			getter:      License.GetCloud,
			unsetFields: unsetFields(License.GetCloud),
		},
		{
			name:        "Set Kubernetes Support",
			setter:      License.SetSupportsKubernetes,
			getter:      License.GetSupportsKubernetes,
			unsetFields: unsetFields(License.GetSupportsKubernetes),
		},
		{
			name:        "Set Application Access Support",
			setter:      License.SetSupportsApplicationAccess,
			getter:      License.GetSupportsApplicationAccess,
			unsetFields: unsetFields(License.GetSupportsApplicationAccess),
		},
		{
			name:        "Set Database Access Support",
			setter:      License.SetSupportsDatabaseAccess,
			getter:      License.GetSupportsDatabaseAccess,
			unsetFields: unsetFields(License.GetSupportsDatabaseAccess),
		},
		{
			name:        "Set Desktop Access Support",
			setter:      License.SetSupportsDesktopAccess,
			getter:      License.GetSupportsDesktopAccess,
			unsetFields: unsetFields(License.GetSupportsDesktopAccess),
		},
		{
			name:        "Set Moderated Sessions Support",
			setter:      License.SetSupportsModeratedSessions,
			getter:      License.GetSupportsModeratedSessions,
			unsetFields: unsetFields(License.GetSupportsModeratedSessions),
		},
		{
			name:        "Set Machine ID Support",
			setter:      License.SetSupportsMachineID,
			getter:      License.GetSupportsMachineID,
			unsetFields: unsetFields(License.GetSupportsMachineID),
		},
		{
			name:        "Set Resource Access Request Support",
			setter:      License.SetSupportsResourceAccessRequests,
			getter:      License.GetSupportsResourceAccessRequests,
			unsetFields: unsetFields(License.GetSupportsResourceAccessRequests),
		},
		{
			name:        "Set Trial Support",
			setter:      License.SetTrial,
			getter:      License.GetTrial,
			unsetFields: unsetFields(License.GetTrial),
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
			for _, unset := range tc.unsetFields {
				require.False(t, bool(unset(license)))
			}
		})
	}

	// Manually test Application Access.
	// If unset application access is set to true by default.
	license := &LicenseV3{}
	require.True(t, bool(license.GetSupportsApplicationAccess()))
	require.False(t, bool(license.GetReportsUsage()))
	require.False(t, bool(license.GetSalesCenterReporting()))
	require.False(t, bool(license.GetCloud()))
	require.False(t, bool(license.GetSupportsKubernetes()))
	require.False(t, bool(license.GetSupportsDatabaseAccess()))
	require.False(t, bool(license.GetSupportsDesktopAccess()))
	require.False(t, bool(license.GetSupportsModeratedSessions()))
	require.False(t, bool(license.GetSupportsMachineID()))
	require.False(t, bool(license.GetSupportsResourceAccessRequests()))
	require.False(t, bool(license.GetTrial()))
}

func fnName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
