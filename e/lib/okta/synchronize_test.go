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
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
)

func TestSynchronizeGroups(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, client := newTestService(t, ap)

	// Add a few groups to ignore since they don't have an origin of Okta.
	addGroup(t, "ignored1", types.OriginConfigFile, "", ap)
	addGroup(t, "ignored2", types.OriginConfigFile, "", ap)

	// Add a group to be ignored because it's from a different Okta org..
	addGroup(t, "diff-group", types.OriginOkta, "https://different-okta-org.com", ap)

	// Add a few groups that should be deleted since they're not present in the client.
	addGroup(t, "group1", types.OriginOkta, svc.orgURL, ap)
	addGroup(t, "group2", types.OriginOkta, svc.orgURL, ap)

	// Add a group to be updated.
	addGroup(t, "group3", types.OriginOkta, svc.orgURL, ap)

	// Add okta groups.
	client.oktaGroups = []*okta.Group{
		// This group should trigger an update.
		{
			Id: "group3",
			Profile: &okta.GroupProfile{
				Name:        "group name",
				Description: "group 3 description",
			},
		},
		// This group should be created.
		{
			Id: "group4",
			Profile: &okta.GroupProfile{
				Name:        "group name",
				Description: "group 4 description",
			},
		},
		// This group should fail but not interrupt the sync.
		{
			Id: "group5",
		},
	}

	group3, err := ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Empty(t, group3.GetMetadata().Description)

	require.NoError(t, svc.startSynchronizerReconcilers(ctx))

	require.NoError(t, svc.synchronize(ctx))

	// Verify the groups that are present.
	// These should be ignored
	_, err = ap.GetUserGroup(ctx, "ignored1")
	require.NoError(t, err)
	_, err = ap.GetUserGroup(ctx, "ignored2")
	require.NoError(t, err)
	_, err = ap.GetUserGroup(ctx, "diff-group")
	require.NoError(t, err)

	// These should have been deleted.
	_, err = ap.GetUserGroup(ctx, "group1")
	require.True(t, trace.IsNotFound(err))
	_, err = ap.GetUserGroup(ctx, "group2")
	require.True(t, trace.IsNotFound(err))
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group 3 description", group3.GetMetadata().Description)
	group4, err := ap.GetUserGroup(ctx, "group4")
	require.NoError(t, err)
	require.Equal(t, "group 4 description", group4.GetMetadata().Description)

	// This should have never been created.
	_, err = ap.GetUserGroup(ctx, "group5")
	require.True(t, trace.IsNotFound(err))
}

func TestSynchronizeApplications(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, client := newTestService(t, ap)
	require.NoError(t, svc.startSynchronizerReconcilers(ctx))

	// Add a few apps that should be deleted since they're not present in the client.
	addApp(t, "app1", types.OriginOkta, svc.orgURL, svc)
	addApp(t, "app2", types.OriginOkta, svc.orgURL, svc)

	// Add an app to be updated.
	app3Name, err := appName(svc.hash, "app3", "applink-name1")
	require.NoError(t, err)
	addApp(t, app3Name, types.OriginOkta, svc.orgURL, svc)

	// Add okta apps.
	client.oktaApps = []okta.App{
		// This app should trigger an update.
		&okta.Application{
			Id:     "app3",
			Name:   "app-name",
			Status: "ACTIVE",
			Label:  "app label",
			Links: map[string]interface{}{
				"appLinks": []interface{}{
					map[string]interface{}{
						"name": "applink-name1",
						"href": "https://www.link1.com",
					},
				},
			},
		},
		// This app should be created.
		&okta.Application{
			Id:     "app4",
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
		// This app should fail but not interrupt the sync.
		&okta.Application{
			Id:     "app5",
			Name:   "app-name",
			Status: "INACTIVE",
			Label:  "app label",
		},
		// This app should fail but not interrupt the sync.
		&dummyOktaApp{},
	}

	apps := mapOfAllApps(t, svc)
	require.Len(t, apps, 3)

	app3 := apps[app3Name]
	require.Equal(t, "https://test.com", app3.GetURI())

	require.NoError(t, svc.synchronize(ctx))

	apps = mapOfAllApps(t, svc)
	require.Len(t, apps, 3)

	// Verify the apps that are present.
	// These should have been deleted.
	_, ok := apps["app1"]
	require.False(t, ok)
	_, ok = apps["app2"]
	require.False(t, ok)

	// This should have been updated
	app3 = apps[app3Name]
	require.Equal(t, "https://www.link1.com", app3.GetURI())

	// This should have been created
	app4Link1Name, err := appName(svc.hash, "app4", "applink-name1")
	require.NoError(t, err)
	app4Link1 := apps[app4Link1Name]
	require.Equal(t, "https://www.link1.com", app4Link1.GetURI())
	app4Link2Name, err := appName(svc.hash, "app4", "applink-name2")
	require.NoError(t, err)
	app4Link2 := apps[app4Link2Name]
	require.Equal(t, "https://www.link2.com", app4Link2.GetURI())
}

func addApp(t *testing.T, name, origin, orgURL string, svc *Service) {
	labels := map[string]string{
		types.OriginLabel: types.OriginOkta,
	}
	if orgURL != "" {
		labels[teleport.OktaOrgURLLabel] = orgURL
	}

	app, err := types.NewAppV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.AppSpecV3{
		URI: "https://test.com",
	})
	require.NoError(t, err)

	svc.appsMu.Lock()
	defer svc.appsMu.Unlock()
	svc.apps[name] = app
}

func addGroup(t *testing.T, name, origin, orgURL string, ap auth.OktaAccessPoint) {
	labels := map[string]string{
		types.OriginLabel: types.OriginOkta,
	}
	if orgURL != "" {
		labels[teleport.OktaOrgURLLabel] = orgURL
	}

	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:   name,
		Labels: labels,
	})
	require.NoError(t, err)

	require.NoError(t, ap.CreateUserGroup(context.Background(), userGroup))
}

func mapOfAllApps(t *testing.T, svc *Service) map[string]*types.AppV3 {
	svc.appsMu.Lock()
	defer svc.appsMu.Unlock()

	appMap := map[string]*types.AppV3{}
	for _, app := range svc.apps {
		appMap[app.GetName()] = app
	}

	return appMap
}
