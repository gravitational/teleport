/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package ui

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ui"
)

func newApp(t *testing.T, name, publicAddr, description string, labels map[string]string) types.Application {
	app, err := types.NewAppV3(types.Metadata{
		Name:        name,
		Description: description,
		Labels:      labels,
	}, types.AppSpecV3{
		URI:        publicAddr,
		PublicAddr: publicAddr,
	})
	require.NoError(t, err)
	return app
}

func TestMakeAppTypeFromSAMLApp(t *testing.T) {
	tests := []struct {
		name             string
		sp               types.SAMLIdPServiceProviderV1
		appsToUserGroups map[string]types.UserGroups
		expected         App
	}{
		{
			name: "saml service provider with empty preset returns unspecified",
			sp: types.SAMLIdPServiceProviderV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "test_app",
					},
				},
				Spec: types.SAMLIdPServiceProviderSpecV1{
					Preset: "",
				},
			},
			expected: App{
				Kind:              types.KindApp,
				Name:              "test_app",
				Description:       "SAML Application",
				PublicAddr:        "",
				Labels:            []ui.Label{},
				SAMLApp:           true,
				SAMLAppPreset:     "unspecified",
				SAMLAppLaunchURLs: []SAMLAppLaunchURL{},
			},
		},
		{
			name: "saml service provider with preset",
			sp: types.SAMLIdPServiceProviderV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "test_app",
					},
				},
				Spec: types.SAMLIdPServiceProviderSpecV1{
					Preset:     "test_preset",
					LaunchURLs: []string{"https://example.com"},
				},
			},
			expected: App{
				Kind:              types.KindApp,
				Name:              "test_app",
				Description:       "SAML Application",
				PublicAddr:        "",
				Labels:            []ui.Label{},
				SAMLApp:           true,
				SAMLAppPreset:     "test_preset",
				SAMLAppLaunchURLs: []SAMLAppLaunchURL{{URL: "https://example.com"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			apps := MakeAppTypeFromSAMLApp(&test.sp, MakeAppsConfig{})
			require.Empty(t, cmp.Diff(test.expected, apps))
		})
	}
}
