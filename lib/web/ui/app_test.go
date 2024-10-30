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

func TestMakeAppsLabelFilter(t *testing.T) {
	type testCase struct {
		AppsOrSPs types.AppServersOrSAMLIdPServiceProviders
		expected  []App
		name      string
	}

	testCases := []testCase{
		{
			name: "Single App with teleport.internal/ label",
			AppsOrSPs: types.AppServersOrSAMLIdPServiceProviders{createAppServerOrSPFromApp(&types.AppV3{
				Metadata: types.Metadata{
					Name: "App1",
					Labels: map[string]string{
						"first":                "value1",
						"teleport.internal/dd": "hidden1",
					},
				},
			})},
			expected: []App{
				{
					Name: "App1",
					Labels: []ui.Label{
						{
							Name:  "first",
							Value: "value1",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := MakeAppsConfig{
				AppServersAndSAMLIdPServiceProviders: tc.AppsOrSPs,
			}
			apps := MakeApps(config)

			for i, app := range apps {
				expectedLabels := tc.expected[i].Labels

				require.Equal(t, expectedLabels, app.Labels)
			}
		})
	}
}

func TestMakeApps(t *testing.T) {
	tests := []struct {
		name             string
		appsOrSPs        types.AppServersOrSAMLIdPServiceProviders
		appsToUserGroups map[string]types.UserGroups
		expected         []App
	}{
		{
			name:     "empty",
			expected: []App{},
		},
		{
			name: "app without user groups",
			appsOrSPs: types.AppServersOrSAMLIdPServiceProviders{
				createAppServerOrSPFromApp(newApp(t, "1", "1.com", "", map[string]string{"label1": "value1"})),
				createAppServerOrSPFromApp(newApp(t, "2", "2.com", "group 2 friendly name", map[string]string{
					"label2": "value2", types.OriginLabel: types.OriginOkta,
				}))},
			expected: []App{
				{
					Kind:       types.KindApp,
					Name:       "1",
					URI:        "1.com",
					PublicAddr: "1.com",
					FQDN:       "1.com",
					Labels:     []ui.Label{{Name: "label1", Value: "value1"}},
					UserGroups: []UserGroupAndDescription{},
				},
				{
					Kind:        types.KindApp,
					Name:        "2",
					Description: "group 2 friendly name",
					URI:         "2.com",
					PublicAddr:  "2.com",
					FQDN:        "2.com",
					Labels: []ui.Label{
						{Name: "label2", Value: "value2"},
						{Name: types.OriginLabel, Value: types.OriginOkta},
					},
					FriendlyName: "group 2 friendly name",
					UserGroups:   []UserGroupAndDescription{},
				},
			},
		},
		{
			name: "app with user groups",
			appsOrSPs: types.AppServersOrSAMLIdPServiceProviders{
				createAppServerOrSPFromApp(newApp(t, "1", "1.com", "", map[string]string{"label1": "value1"})),
				createAppServerOrSPFromApp(newApp(t, "2", "2.com", "group 2 friendly name", map[string]string{
					"label2": "value2", types.OriginLabel: types.OriginOkta,
				}))},
			appsToUserGroups: map[string]types.UserGroups{
				"1": {
					newGroup(t, "group1", "group1 desc", nil),
					newGroup(t, "group2", "group2 desc", nil),
				},
				"2": {
					newGroup(t, "group2", "group2 desc", nil),
					newGroup(t, "group3", "group3 desc", nil),
				},
				// This will be ignored
				"3": {
					newGroup(t, "group3", "group3 desc", nil),
				},
			},
			expected: []App{
				{
					Kind:       types.KindApp,
					Name:       "1",
					URI:        "1.com",
					PublicAddr: "1.com",
					FQDN:       "1.com",
					Labels:     []ui.Label{{Name: "label1", Value: "value1"}},
					UserGroups: []UserGroupAndDescription{
						{Name: "group1", Description: "group1 desc"},
						{Name: "group2", Description: "group2 desc"},
					},
				},
				{
					Kind:        types.KindApp,
					Name:        "2",
					Description: "group 2 friendly name",
					URI:         "2.com",
					PublicAddr:  "2.com",
					FQDN:        "2.com",
					Labels: []ui.Label{
						{Name: "label2", Value: "value2"},
						{Name: types.OriginLabel, Value: types.OriginOkta},
					},
					FriendlyName: "group 2 friendly name",
					UserGroups: []UserGroupAndDescription{
						{Name: "group2", Description: "group2 desc"},
						{Name: "group3", Description: "group3 desc"},
					},
				},
			},
		},
		{
			name: "saml idp service provider",
			appsOrSPs: types.AppServersOrSAMLIdPServiceProviders{createAppServerOrSPFromSAMLIdPServiceProvider(&types.SAMLIdPServiceProviderV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "grafana_saml",
					},
				},
			})},
			expected: []App{
				{
					Kind:        types.KindApp,
					Name:        "grafana_saml",
					Description: "SAML Application",
					PublicAddr:  "",
					Labels:      []ui.Label{},
					SAMLApp:     true,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			apps := MakeApps(MakeAppsConfig{
				AppServersAndSAMLIdPServiceProviders: test.appsOrSPs,
				AppsToUserGroups:                     test.appsToUserGroups,
			})

			require.Empty(t, cmp.Diff(test.expected, apps))
		})
	}
}

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
				Kind:          types.KindApp,
				Name:          "test_app",
				Description:   "SAML Application",
				PublicAddr:    "",
				Labels:        []ui.Label{},
				SAMLApp:       true,
				SAMLAppPreset: "unspecified",
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
					Preset: "test_preset",
				},
			},
			expected: App{
				Kind:          types.KindApp,
				Name:          "test_app",
				Description:   "SAML Application",
				PublicAddr:    "",
				Labels:        []ui.Label{},
				SAMLApp:       true,
				SAMLAppPreset: "test_preset",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			apps := MakeAppTypeFromSAMLApp(&test.sp, MakeAppsConfig{})
			require.Empty(t, cmp.Diff(test.expected, apps))
		})
	}
}

// createAppServerOrSPFromApp returns a AppServerOrSAMLIdPServiceProvider given an App.
//
//nolint:staticcheck // SA1019. Kept to be deleted along with the API in 16.0.
func createAppServerOrSPFromApp(app types.Application) types.AppServerOrSAMLIdPServiceProvider {
	//nolint:staticcheck // SA1019. Kept to be deleted along with the API in 16.0.
	appServerOrSP := &types.AppServerOrSAMLIdPServiceProviderV1{
		Resource: &types.AppServerOrSAMLIdPServiceProviderV1_AppServer{
			AppServer: &types.AppServerV3{
				Spec: types.AppServerSpecV3{
					App: app.(*types.AppV3),
				},
			},
		},
	}

	return appServerOrSP
}

// createAppServerOrSPFromApp returns a AppServerOrSAMLIdPServiceProvider given a SAMLIdPServiceProvider.
//
//nolint:staticcheck // SA1019. Kept to be deleted along with the API in 16.0.
func createAppServerOrSPFromSAMLIdPServiceProvider(sp types.SAMLIdPServiceProvider) types.AppServerOrSAMLIdPServiceProvider {
	appServerOrSP := &types.AppServerOrSAMLIdPServiceProviderV1{
		Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
			SAMLIdPServiceProvider: sp.(*types.SAMLIdPServiceProviderV1),
		},
	}

	return appServerOrSP
}
