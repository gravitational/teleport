/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
)

func TestOktaGroupToUserGroup(t *testing.T) {
	oktaGroup := &okta.Group{
		Id: "okta-group-id",
	}

	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	service, _ := newTestService(t, ap)
	_, err := service.oktaGroupToUserGroup(oktaGroup)
	require.ErrorIs(t, trace.BadParameter("the okta group okta-group-id has no profile"), err)

	oktaGroup = &okta.Group{
		Id: "okta-group-id",
		Profile: &okta.GroupProfile{
			Name:        "group name",
			Description: "group description",
		},
	}

	userGroup, err := service.oktaGroupToUserGroup(oktaGroup)
	require.NoError(t, err)

	expected, err := types.NewUserGroup(types.Metadata{
		Name:        "okta-group-id",
		Description: "group name (group description)",
		Labels: map[string]string{
			types.OriginLabel:         types.OriginOkta,
			teleport.OktaOrgURLLabel:  service.orgURL,
			teleport.OktaGroupIDLabel: "okta-group-id",
		},
	})
	require.NoError(t, err)
	require.Equal(t, expected, userGroup)
}

func TestIsGroupValid(t *testing.T) {
	tests := []struct {
		name             string
		group            *okta.Group
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is valid",
			group: &okta.Group{
				Id: "group-id",
				Profile: &okta.GroupProfile{
					Name: "group",
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "no profile",
			group: &okta.Group{
				Id: "group-id",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("the okta group group-id has no profile"))
			},
		},
		{
			name: "everyone",
			group: &okta.Group{
				Id: "group-id",
				Profile: &okta.GroupProfile{
					Name: oktaGroupEveryone,
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("group group-id is Everyone"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.errAssertionFunc(t, isGroupValid(test.group))
		})
	}
}

type dummyOktaApp struct{}

func (d *dummyOktaApp) IsApplicationInstance() bool {
	return false
}

func TestOktaAppToApplications(t *testing.T) {
	tests := []struct {
		name             string
		oktaApp          *okta.Application
		errAssertionFunc require.ErrorAssertionFunc
		expected         []*types.AppV3
	}{
		{
			name: "happy path",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Name:   "app-name",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]interface{}{
					"appLinks": []interface{}{
						map[string]interface{}{
							"name": "applink-name1",
							"href": "https://www.link1.com",
						},
						map[string]interface{}{
							"name": "applink-name2",
							"href": "https://www.link2.com",
						},
					},
				},
			},
			errAssertionFunc: require.NoError,
			expected: []*types.AppV3{
				newApp(t,
					types.Metadata{
						Name:        "hleAaWyYRHhA",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:        types.OriginOkta,
							teleport.OktaOrgURLLabel: testOrgURL,
							teleport.OktaAppIDLabel:  "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link1.com",
						PublicAddr: fmt.Sprintf("hleAaWyYRHhA.%s", testClusterName),
					},
				),
				newApp(t,
					types.Metadata{
						Name:        "utGMAFLgNkBj",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:        types.OriginOkta,
							teleport.OktaOrgURLLabel: testOrgURL,
							teleport.OktaAppIDLabel:  "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link2.com",
						PublicAddr: fmt.Sprintf("utGMAFLgNkBj.%s", testClusterName),
					},
				),
			},
		},
		{
			name: "not active",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "INACTIVE",
				Label:  "app label",
				Links: map[string]interface{}{
					"appLinks": []interface{}{
						map[string]interface{}{
							"name": "applink-name",
							"href": "https://wwww.link.com",
						},
					},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is not active"))
			},
		},
		{
			name: "empty links",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("links is missing in okta application object app-id (app label)"))
			},
		},
		{
			name: "no app links",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]interface{}{
					"appLinks": []interface{}{},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("app links is empty in okta application object app-id (app label)"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			service, _ := newTestService(t, ap)
			apps, err := service.oktaAppToApp(test.oktaApp)
			test.errAssertionFunc(t, err)
			require.Equal(t, test.expected, apps)
		})
	}
}

func TestIsAppValid(t *testing.T) {
	trueBool := true
	falseBool := false

	tests := []struct {
		name             string
		app              *okta.Application
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is valid",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &falseBool}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "not active",
			app: &okta.Application{
				Id:     "app-id",
				Label:  "app label",
				Status: "INACTIVE",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is not active"))
			},
		},
		{
			name: "okta admin console",
			app: &okta.Application{
				Id:     "app-id",
				Label:  "Okta Admin Console",
				Status: oktaActive,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id is the Okta admin console"))
			},
		},
		{
			name: "hidden",
			app: &okta.Application{
				Id:         "app-id",
				Label:      "app label",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &trueBool}},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is hidden from the web"))
			},
		},
		{
			name: "only visibility present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "only hide present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{}},
			},
			errAssertionFunc: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.errAssertionFunc(t, isAppValid(test.app))
		})
	}
}
