package okta

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	oktaapitest "github.com/gravitational/teleport/e/lib/okta/api/apitest"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/app"
)

func TestSynchronizeGroups(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testSynchronizeGroups)
}
func testSynchronizeGroups(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	client := newTestOktaClient()
	svc, emitter := newTestService(t, ap, client)

	// Add in one app to get a group to app mapping from
	client.OktaApps = []okta.App{
		// This app should be added.
		&okta.Application{
			Id:     "app1",
			Name:   "app-name",
			Status: "ACTIVE",
			Label:  "app label",
			Links: map[string]any{
				"appLinks": []any{
					map[string]any{
						"name": "applink-name1",
						"href": "https://www.link1.com",
					},
				},
			},
		},
	}
	client.AppsToGroups["app1"] = []oktaGroupID{"group4"}

	// Add a few groups to ignore since they don't have an origin of Okta.
	addGroup(t, "ignored1", types.OriginConfigFile, "", ap)
	addGroup(t, "ignored2", types.OriginConfigFile, "", ap)

	// Add a group to be ignored because it's from a different Okta org.
	addGroup(t, "diff-group", types.OriginOkta, "https://different-okta-org.com", ap)

	// Add a few groups that should be deleted since they're not present in the client.
	addGroup(t, "group1", types.OriginOkta, svc.orgURL, ap)
	addGroup(t, "group2", types.OriginOkta, svc.orgURL, ap)

	// Add a group to be updated.
	addGroup(t, "group3", types.OriginOkta, svc.orgURL, ap)

	// Add okta groups.
	client.OktaGroups = []*okta.Group{
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

	require.NoError(t, svc.seedAppsReconciler(ctx))
	require.NoError(t, svc.seedGroupsReconciler(ctx))

	svc.userReconciler = nil // disable user reconciliation
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
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	_, err = ap.GetUserGroup(ctx, "group2")
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// These should be created.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Empty(t, group3.GetApplications())
	group4, err := ap.GetUserGroup(ctx, "group4")
	require.NoError(t, err)
	require.Equal(t, "group name (group 4 description)", group4.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, "app1", "applink-name1")}, group4.GetApplications())

	// This should have never been created.
	_, err = ap.GetUserGroup(ctx, "group5")
	require.True(t, trace.IsNotFound(err))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(1), event.Added)
		require.Equal(t, int32(0), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(1), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(2), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaAccessListSync) {
		require.True(t, event.Success)
		require.Equal(t, testClusterName, event.ClusterName)
		require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
		require.Zero(t, event.NumAppFilters)
		require.Zero(t, event.NumGroupFilters)
		require.Zero(t, event.NumApps)
		require.Zero(t, event.NumGroups)
		require.Zero(t, event.NumRoles)
		require.Zero(t, event.NumAccessLists)
		require.Zero(t, event.NumAccessListMembers)
	})

	// App1 is now assigned to group3 as well.
	client.AppsToGroups["app1"] = []oktaGroupID{"group3", "group4"}

	require.NoError(t, svc.synchronize(ctx))

	// Verify app1 is now in group3's application list.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, "app1", "applink-name1")}, group3.GetApplications())

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaAccessListSync) {
		require.True(t, event.Success)
		require.Equal(t, testClusterName, event.ClusterName)
		require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
		require.Zero(t, event.NumAppFilters)
		require.Zero(t, event.NumGroupFilters)
		require.Zero(t, event.NumApps)
		require.Zero(t, event.NumGroups)
		require.Zero(t, event.NumRoles)
		require.Zero(t, event.NumAccessLists)
		require.Zero(t, event.NumAccessListMembers)
	})

	// Let's explicitly set group3 to have no applications to simulate the apps and group
	// mappings getting out of sync with one another.
	group3.SetApplications(nil)
	require.NoError(t, ap.UpdateUserGroup(ctx, group3))

	svc.groups.Store(group3.GetName(), group3)

	require.NoError(t, svc.synchronize(ctx))

	// Verify app1 is now in group3's application list.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, "app1", "applink-name1")}, group3.GetApplications())

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaAccessListSync) {
		require.True(t, event.Success)
		require.Equal(t, testClusterName, event.ClusterName)
		require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
		require.Zero(t, event.NumAppFilters)
		require.Zero(t, event.NumGroupFilters)
		require.Zero(t, event.NumApps)
		require.Zero(t, event.NumGroups)
		require.Zero(t, event.NumRoles)
		require.Zero(t, event.NumAccessLists)
		require.Zero(t, event.NumAccessListMembers)
	})

	// We need to simulate the user group backend getting out of sync with the reconciler here.

	// This will cause create to be re-run on group 3, which should be handled.
	svc.groups.Delete(group3.GetName())

	// This will cause update to be run on a non-existent group, which should be handled.
	require.NoError(t, ap.DeleteUserGroup(ctx, group4.GetName()))
	svc.groups.Write(func(groups map[string]types.UserGroup) {
		groups[group4.GetName()].GetMetadata().Labels["dummy"] = "update"
	})
	require.NoError(t, svc.synchronize(ctx))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(1), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaAccessListSync) {
		require.True(t, event.Success)
		require.Equal(t, testClusterName, event.ClusterName)
		require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
		require.Zero(t, event.NumAppFilters)
		require.Zero(t, event.NumGroupFilters)
		require.Zero(t, event.NumApps)
		require.Zero(t, event.NumGroups)
		require.Zero(t, event.NumRoles)
		require.Zero(t, event.NumAccessLists)
		require.Zero(t, event.NumAccessListMembers)
	})

	// This will cause delete to be run on a non-existent group, which should be handled.
	require.NoError(t, ap.DeleteUserGroup(ctx, group4.GetName()))
	client.OktaGroups = client.OktaGroups[0:1]

	require.NoError(t, svc.synchronize(ctx))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(0), event.Updated)
		require.Equal(t, int32(1), event.Deleted)
	})
	expectAuditEvent(t, emitter, func(event *apievents.OktaAccessListSync) {
		require.True(t, event.Success)
		require.Equal(t, testClusterName, event.ClusterName)
		require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
		require.Zero(t, event.NumAppFilters)
		require.Zero(t, event.NumGroupFilters)
		require.Zero(t, event.NumApps)
		require.Zero(t, event.NumGroups)
		require.Zero(t, event.NumRoles)
		require.Zero(t, event.NumAccessLists)
		require.Zero(t, event.NumAccessListMembers)
	})
}

func TestSynchronizeAppsImportError(t *testing.T) {
	appNames := []string{"app1", "app2", "app3"}

	testCases := []struct {
		name string

		// assertSyncResult is the assertion for the synhronization result.
		assertSyncResult require.ErrorAssertionFunc

		// importErrors describes the error to return when the okta client
		// is queried for a given app's assigned groups. Defaults to
		// returning the test client's configured group list for all
		// apps not specified in the map.
		importErrors map[oktaAppID]error

		// expectAppErrors holds assertions about the applications held in the
		// Sync service applications map after the possibly-failed sync.
		// Defaults to require.NotNil for all values specified in the map
		assertAppValue map[string]require.ValueAssertionFunc
	}{
		// Asserts that a random error while fetching application group data is
		// propagated and is treated as a sync-stopping offense
		{
			name:             "Error cancels sync",
			assertSyncResult: require.Error,
			importErrors: map[oktaAppID]error{
				"app2": errors.New("Some transient error"),
			},
		},

		// Asserts that a missing application is not treated as sync-stopping
		// error, but deletes the application as per normal
		{
			name:             "Not found is not an error",
			assertSyncResult: require.NoError,
			importErrors: map[oktaAppID]error{
				"app2": trace.NotFound("No such application"),
			},
			assertAppValue: map[string]require.ValueAssertionFunc{
				"app2": require.Nil,
			},
		},
	}

	for _, tt := range testCases {
		subtest := func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			// GIVEN a running Okta sync service
			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			client := newTestOktaClient()
			// Increase the BackendTasksPerSecond so the test doesn't timeout with the
			// flaky tests detector -count 100 flag.
			svc, emitter := newTestService(t, ap, client, withBackendTasksPerSecond(10))
			require.NoError(t, svc.seedAppsReconciler(ctx))
			require.NoError(t, svc.seedGroupsReconciler(ctx))

			// ALSO GIVEN a mocked Okta organization with several applications
			// configured
			for _, appName := range appNames {
				client.OktaApps = append(client.OktaApps, &okta.Application{
					Id:     appName,
					Name:   fmt.Sprintf("An app called %q", appName),
					Status: "ACTIVE",
					Label:  fmt.Sprintf("label %s", appName),
					Links: map[string]any{
						"appLinks": []any{
							map[string]any{
								"name": "applink",
								"href": "https://www.link1.com/" + appName,
							},
						},
					},
				})
			}
			client.AppsToGroups["app1"] = []oktaGroupID{"group1"}

			// ALSO GIVEN a set of Teleport Applications created by pre-syncing
			// the Okta organization with the Teleport cluster
			svc.userReconciler = nil // disable user sync
			err := svc.synchronize(ctx)
			require.NoError(t, err)
			expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
				require.Equal(t, int32(3), event.Added)
			})

			// GIVEN ALSO an okta client rigged to fail when fetching groups for
			// specific applications
			client.MonkeyPatch.GetAppGroups =
				func(_ context.Context, appID oktaAppID) ([]oktaGroupID, error) {
					if err, ok := tt.importErrors[appID]; ok {
						return nil, err
					}
					return client.AppsToGroups[appID], nil
				}

			// WHEN I attempt to synchronize the Teleport cluster with the
			// upstream organization
			err = svc.synchronize(ctx)

			// EXPECT that the operation succeeds or fails appropriately
			// according to the test case
			tt.assertSyncResult(t, err)

			// EXPECT that all of the Teleport Applications derived from upstream
			// Okta apps have been appropriately preserved or deleted
			for _, appName := range appNames {
				appID := mustAppName(t, appName, "applink")
				assertAppValue, hasCustom := tt.assertAppValue[appName]
				if !hasCustom {
					assertAppValue = require.NotNil
				}

				app, _ := svc.appServers.Load(appID)
				assertAppValue(t, app, "App %s", appName)
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, subtest)
		})
	}
}

func TestSynchronizeApplications(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testSynchronizeApplications)
}
func testSynchronizeApplications(t *testing.T) {
	ctx := t.Context()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	client := newTestOktaClient()
	svc, emitter := newTestService(t, ap, client)

	const app1, app2, app3, app4, app5 = "app1", "app2", "app3", "app4", "app5"
	const link1, link2 = "app_link_1", "app_link_2"
	const href1, href2 = "https://link1.example.com", "https://link2.exmpale.com"

	// Add apps stored in the backend.
	for _, oktaID := range []string{app1, app2, app3} {
		uri := href1
		if oktaID == app3 {
			// For app3 set an incorrect URI to see if it was updated.
			uri = "https://www.wrong1.link.exmaple.com"
		}
		upsertAppServer(t, ap, newAppServer(t,
			types.Metadata{
				Name: mustAppName(t, oktaID, link1),
				Labels: map[string]string{
					types.OktaAppIDLabel:     oktaID,
					types.OriginLabel:        types.OriginOkta,
					teleport.OktaOrgURLLabel: svc.orgURL,
				},
			},
			types.AppSpecV3{
				URI: uri,
			},
		))
	}

	// Add okta apps.
	client.OktaApps = []okta.App{
		// This app should trigger an update.
		oktaapitest.NewApplication(oktaapitest.ApplicationArgs{
			ID:     app3,
			Label:  "App with ID " + app3,
			Status: oktaapitest.StatusActive,
			Links: []oktaapitest.AppLink{
				{Name: link1, Href: href1},
			},
		}),
		// This app should be created.
		oktaapitest.NewApplication(oktaapitest.ApplicationArgs{
			ID:     app4,
			Label:  "App with ID " + app4,
			Status: oktaapitest.StatusActive,
			Links: []oktaapitest.AppLink{
				{Name: link1, Href: href1},
				{Name: link2, Href: href2},
			},
		}),
		// This app should fail but not interrupt the sync.
		oktaapitest.NewApplication(oktaapitest.ApplicationArgs{
			ID:     app5,
			Label:  "App with ID " + app5,
			Status: oktaapitest.StatusActive,
			Links:  nil, // no links, so the app should not be synced
		}),
		// This app should fail but not interrupt the sync.
		&dummyOktaApp{},
	}
	client.AppsToGroups[app4] = []oktaGroupID{"group4"}

	// Verify conditions before synchronizing.
	requireOktaAppServers(t, ap, []string{
		mustAppName(t, app1, link1),
		mustAppName(t, app2, link1),
		mustAppName(t, app3, link1),
	})
	// Verify app3 link is not href1 before sync.
	require.NotEqual(t, href1, testGetAppServer(t, ap, mustAppName(t, app3, link1)).GetApp().GetURI())

	require.NoError(t, svc.seedAppsReconciler(ctx))
	require.NoError(t, svc.seedGroupsReconciler(ctx))
	svc.userReconciler = nil // disable user sync
	require.NoError(t, svc.synchronize(ctx))

	// Verify conditions after synchronizing.
	requireOktaAppServers(t, svc.accessPoint, []string{
		// app1, app2 got deleted because they are not present in Okta.
		// app3 is still there, but its link got updated which will be verified later.
		mustAppName(t, app3, link1),
		// There are 2 app_severs for Okta app4, because it has 2 links.
		mustAppName(t, app4, link1),
		mustAppName(t, app4, link2),
		// There is no app_server for Okta app5, because it has no links.
	})
	// Verify app3 link is href1 after sync.
	require.Equal(t, href1, testGetAppServer(t, ap, mustAppName(t, app3, link1)).GetApp().GetURI())

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(2), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(2), event.Deleted)
	})
}

// TestSynchronizeIgnoresHostID checks if app_servers sync doesn't update anything in the backend
// if only host_id changes. It also checks if app_server is deleted even though the host_id doesn't
// match the one configured in the Service.
func TestSynchronizeIgnoresHostID(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testSynchronizeIgnoresHostID)
}
func testSynchronizeIgnoresHostID(t *testing.T) {
	ctx := t.Context()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	client := newTestOktaClient()
	svc, emitter := newTestService(t, ap, client)

	require.Equal(t, testHostID, svc.hostID)

	const app1, app2, app3, app4 = "app1", "app2", "app3", "app4"
	const link1, href1 = "app_link_1", "https://link1.test"

	// Initialized backends with app servers skipping app4.
	for _, pair := range [][2]string{
		{app1, "test_host_id_1"},    // app1 to delete
		{app1, "test_host_id_12"},   // same app1, different host ID
		{app2, oktaAppServerHostID}, // app2 with desired host ID
		{app3, "test_host_id_3"},    // app3 with host ID to fix
		{app3, svc.hostID},          // same app3 but with actual host ID form the Service to verify there isn't anything special about it
	} {
		oktaID, hostID := pair[0], pair[1]
		name := mustAppName(t, oktaID, link1)
		publicAddr, err := app.FindPublicAddr(ctx, ap, "", name)
		require.NoError(t, err)
		upsertAppServer(t, ap, newAppServer(t,
			types.Metadata{
				Name: name,
				Labels: map[string]string{
					types.OktaAppIDLabel:          oktaID,
					types.OktaAppNameLabel:        "App with ID " + oktaID,
					types.OktaAppDescriptionLabel: link1,
					types.OriginLabel:             types.OriginOkta,
					teleport.OktaOrgURLLabel:      svc.orgURL,
				},
				Description: "App with ID " + oktaID,
			},
			types.AppSpecV3{
				URI:        href1,
				PublicAddr: publicAddr,
				UserGroups: []string{},
			},
			withHostID(hostID),
		))
	}

	// Add apps to Okta client skipping the app1.
	for _, oktaID := range []string{app2, app3, app4} {
		client.OktaApps = append(client.OktaApps, oktaapitest.NewApplication(oktaapitest.ApplicationArgs{
			ID:     oktaID,
			Label:  "App with ID " + oktaID,
			Status: oktaapitest.StatusActive,
			Links: []oktaapitest.AppLink{
				{Name: link1, Href: href1},
			},
		}))
	}

	// Verify conditions before synchronizing.
	requireOktaAppServers(t, ap, []string{
		// duplicated apps have different host IDs
		mustAppName(t, app1, link1),
		mustAppName(t, app1, link1),
		mustAppName(t, app2, link1),
		mustAppName(t, app3, link1),
		mustAppName(t, app3, link1),
	})

	require.NoError(t, svc.seedAppsReconciler(ctx))
	require.NoError(t, svc.seedGroupsReconciler(ctx))
	svc.userReconciler = nil // disable user sync
	require.NoError(t, svc.synchronize(ctx))

	// Verify conditions after synchronizing.
	requireOktaAppServers(t, svc.accessPoint, []string{
		// app1 got deleted because it isn't present in Okta, and app4 was added
		mustAppName(t, app2, link1),
		mustAppName(t, app3, link1),
		mustAppName(t, app4, link1),
	})

	// Verify the host_id is set to "okta-integration" fixed value for all synced Okta apps.
	requireAppServerExists(t, ap, oktaAppServerHostID, mustAppName(t, app2, link1))
	requireAppServerExists(t, ap, oktaAppServerHostID, mustAppName(t, app3, link1))
	requireAppServerExists(t, ap, oktaAppServerHostID, mustAppName(t, app4, link1))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(1), event.Added)
		require.Equal(t, int32(0), event.Updated)
		require.Equal(t, int32(1), event.Deleted)
	})
}

func TestEmitSyncEventsInBatches(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testEmitSyncEventsInBatches)
}
func testEmitSyncEventsInBatches(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, emitter := newTestService(t, ap, newTestOktaClient())
	require.NoError(t, svc.seedAppsReconciler(ctx))
	require.NoError(t, svc.seedGroupsReconciler(ctx))

	added := genEventResources(50, "added")
	updated := genEventResources(100, "updated")
	deleted := genEventResources(100, "deleted")

	go svc.emitSyncEventsInBatches(ctx, events.OktaApplicationsUpdateEvent, events.OktaApplicationsUpdateCode, added, updated, deleted)

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(50), event.Added)
		require.Equal(t, int32(50), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
		verifyEventResources(t, event.AddedResources, 0, 50, "added")
		verifyEventResources(t, event.UpdatedResources, 0, 50, "updated")
		verifyEventResources(t, event.DeletedResources, 0, 0, "deleted")
	})

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(50), event.Updated)
		require.Equal(t, int32(50), event.Deleted)
		verifyEventResources(t, event.AddedResources, 0, 0, "added")
		verifyEventResources(t, event.UpdatedResources, 50, 50, "updated")
		verifyEventResources(t, event.DeletedResources, 0, 50, "deleted")
	})

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(0), event.Updated)
		require.Equal(t, int32(50), event.Deleted)
		verifyEventResources(t, event.AddedResources, 0, 0, "added")
		verifyEventResources(t, event.UpdatedResources, 0, 0, "updated")
		verifyEventResources(t, event.DeletedResources, 50, 50, "deleted")
	})
}

func TestTickerUpdates(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testTickerUpdates)
}
func testTickerUpdates(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, _ := newTestService(t, ap, newTestOktaClient())
	clock := clockwork.NewFakeClock()
	svc.clock = clock
	svc.timeBetweenImports = time.Second * 10

	interval := svc.getSynchronizerInterval(ctx)
	require.Equal(t, svc.timeBetweenImports, interval)

	// Set the time between syncs to 300 seconds
	pref, err := ap.GetAuthPreference(ctx)
	require.NoError(t, err)
	pref.SetOktaSyncPeriod(time.Second * 300)
	pref, err = ap.UpdateAuthPreference(ctx, pref)
	require.NoError(t, err)

	interval = svc.getSynchronizerInterval(ctx)
	require.Equal(t, pref.GetOktaSyncPeriod(), interval)

	// Set the duration back to zero, should set the ticker back to 10 seconds.
	pref.SetOktaSyncPeriod(0)
	_, err = ap.UpdateAuthPreference(ctx, pref)
	require.NoError(t, err)

	interval = svc.getSynchronizerInterval(ctx)
	require.Equal(t, svc.timeBetweenImports, interval)
}

func addGroup(t *testing.T, name, origin, orgURL string, ap authclient.OktaAccessPoint) {
	labels := map[string]string{
		types.OriginLabel: types.OriginOkta,
	}
	if orgURL != "" {
		labels[teleport.OktaOrgURLLabel] = orgURL
	}

	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)

	require.NoError(t, ap.CreateUserGroup(context.Background(), userGroup))
}

func genEventResources(numResources int, descPrefix string) []*apievents.OktaResource {
	resources := make([]*apievents.OktaResource, numResources)

	for i := range numResources {
		resources[i] = &apievents.OktaResource{
			ID:          fmt.Sprintf("%d", i),
			Description: fmt.Sprintf("%s %d", descPrefix, i),
		}
	}

	return resources
}

func verifyEventResources(t *testing.T, resources []*apievents.OktaResource, offset, numResources int, descPrefix string) []*apievents.OktaResource {
	require.Len(t, resources, numResources)

	for i := range numResources {
		index := offset + i
		require.Equal(t, fmt.Sprintf("%d", index), resources[i].ID)
		require.Equal(t, fmt.Sprintf("%s %d", descPrefix, index), resources[i].Description)
	}

	return resources
}

const entityDescriptor = `
<?xml version="1.0"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2021-02-26T15:57:24Z" cacheDuration="PT1614787044S" entityID="http://some.entity.id">
	<md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
	<md:KeyDescriptor use="signing">
		<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
		<ds:X509Data>
			<ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
		</ds:X509Data>
		</ds:KeyInfo>
	</md:KeyDescriptor>
	<md:KeyDescriptor use="encryption">
		<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
		<ds:X509Data>
			<ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
		</ds:X509Data>
		</ds:KeyInfo>
	</md:KeyDescriptor>
	<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
	<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://example.com/saml/acs/example"/>
	</md:IDPSSODescriptor>
</md:EntityDescriptor>`

func TestSynchronizeUsers(t *testing.T) {
	t.Parallel()

	const (
		samlConnectorName = "upstream-okta"
		oktaSAMLAppID     = "some-saml-app"
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	now := time.Now()
	userNames := []string{"alpha", "beta", "gamma"}

	setupTest := func(subtestT *testing.T) (*Service, *testOktaClient) {
		ap := newTestAccessPoint(t, clockwork.NewFakeClockAt(now))

		connector, err := types.NewSAMLConnector(
			samlConnectorName,
			types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://example.com",
				EntityDescriptor:         entityDescriptor,
				AttributesToRoles: []types.AttributeMapping{
					{
						Name:  "groups",
						Value: "*",
						Roles: []string{"okta-requester"},
					},
				},
			},
		)
		require.NoError(subtestT, err, "creating SAML connector")

		_, err = ap.CreateSAMLConnector(ctx, connector)
		require.NoError(subtestT, err, "registering SAML connector")

		client := newTestOktaClient()
		svc, _ := newTestService(subtestT, ap, client,
			withUserSyncEnabled(types.OktaUserSyncSourceSamlApp),
			withOktaAppID(oktaSAMLAppID),
			withSSOConnector(samlConnectorName),
			withClock(ap.Clock()),
		)

		for i, name := range userNames {
			client.OktaAppUsers = append(client.OktaAppUsers,
				&okta.AppUser{
					Id:         fmt.Sprintf("%08d", i+1),
					ExternalId: name + "@example.org",
					Profile: map[string]any{
						"firstName": name,
					},
					Status: userStatusProvisioned,
					Credentials: &okta.AppUserCredentials{
						UserName: name + "@example.org",
					},
				})
		}

		return svc, client
	}

	t.Run("Status updated on success", func(t *testing.T) {

		// GIVEN
		//  - a test Teleport cluster,
		//  - a configured Okta integration service, and
		//  - a mock Okta organization populated with users
		serviceUnderTest, _ := setupTest(t)

		// WHEN I attempt to sync the Teleport user DB with the upstream Okta
		// organization...
		err := serviceUnderTest.syncUsers(ctx)

		// EXPECT the sync to succeed
		require.NoError(t, err)

		// EXPECT that the status information has been updated
		userSyncStatus := serviceUnderTest.serviceStatus.details.UsersSyncDetails
		require.True(t, userSyncStatus.LastSuccessful.Equal(now),
			"Expected last success timestamp %s, got %s", now, userSyncStatus.LastSuccessful)
		require.Equal(t, len(userNames), int(userSyncStatus.NumUsersSynced))

		// EXPECT that the failure  information is untouched
		require.Nil(t, userSyncStatus.LastFailed)
		require.Empty(t, userSyncStatus.Error)
	})

	t.Run("Status updated on Okta User listing failure", func(t *testing.T) {
		const errorText = "invalid token"

		// GIVEN
		//  - a test Teleport cluster,
		//  - a configured Okta integration service, and
		//  - a mock Okta organization populated with users
		serviceUnderTest, client := setupTest(t)

		// ALSO GIVEN an Okta client rigged to simulate an access denied error
		// while enumerating Okta users...
		client.MonkeyPatch.IterateAppUsers =
			func(context.Context, oktaAppID, func(*okta.AppUser) error) error {
				return trace.AccessDenied("%s", errorText)
			}

		// WHEN I try to sync the Teleport user DB with Okta
		err := serviceUnderTest.syncUsers(ctx)

		// EXPECT the operation to fail
		require.Error(t, err)

		// EXPECT that the failure & error status info has been updated
		userSyncStatus := serviceUnderTest.serviceStatus.details.UsersSyncDetails
		require.True(t, userSyncStatus.LastFailed.Equal(now),
			"Expected last failure timestamp %s, got %s", now, userSyncStatus.LastFailed)
		require.Contains(t, userSyncStatus.Error, errorText)

		// EXPECT that the success case info has not been touched
		require.Nil(t, userSyncStatus.LastSuccessful)
		require.Zero(t, userSyncStatus.NumUsersSynced)
	})

	// TODO(tcsc): figure out how to force a reconciliation failure to assert
	// that the status is updated in that case
}

func Test_fetchOktaUsers(t *testing.T) {
	oktaUsers := []*okta.User{
		{
			Id: "org-user-00000001",
			Profile: &okta.UserProfile{
				"login": "org.user1@example.org",
			},
			Status: userStatusActive,
		},
		{
			Id:      "org-user-00000002",
			Profile: nil,
			Status:  userStatusActive,
		},
		{
			Id: "org-user-00000003",
			Profile: &okta.UserProfile{
				"login": "org.user3@example.org",
			},
			Status: userStatusActive,
		},
		{
			Id: "org-user-00000004",
			Profile: &okta.UserProfile{
				"login": "org.user4.suspended@example.org",
			},
			Status: userStatusSuspended,
		},
	}

	validOktaUsers := []string{
		"org.user1@example.org",
		"org.user3@example.org",
	}

	// In real life app users should be a subset of the org users, but for this tests it does
	// not matter and trying to mimic that would make the test more blurry.
	oktaAppUsers := []*okta.AppUser{
		{
			Id:         "okta-app-user-00000001",
			ExternalId: "alpha@example.org",
			Profile:    map[string]any{},
			Credentials: &okta.AppUserCredentials{
				UserName: "alpha@example.org",
			},
			Status: userStatusActive,
		},
		{
			Id:          "okta-app-user-00000002",
			ExternalId:  "missing-credentials@example.org",
			Profile:     map[string]any{},
			Credentials: &okta.AppUserCredentials{},
			Status:      userStatusActive,
		},
		{
			Id:         "okta-app-user-00000003",
			ExternalId: "beta@example.org",
			Profile:    map[string]any{},
			Credentials: &okta.AppUserCredentials{
				UserName: "beta@example.org",
			},
			Status: userStatusActive,
		},
		{
			Id:         "okta-app-user-00000004",
			ExternalId: "missing-status@example.org",
			Profile:    map[string]any{},
			Credentials: &okta.AppUserCredentials{
				UserName: "missing-status@example.org",
			},
		},
	}

	validOktaAppUsers := []string{
		"alpha@example.org",
		"beta@example.org",
	}

	testCases := []struct {
		name                   string
		appId                  string
		userSyncSource         types.OktaUserSyncSource
		expectedUsers          []string
		expectedUserSyncSource types.OktaUserSyncSource
		expectedErr            string
	}{
		{
			name:                   "org sync source",
			appId:                  "",
			userSyncSource:         types.OktaUserSyncSourceOrg,
			expectedUsers:          validOktaUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceOrg,
			expectedErr:            "",
		},
		{
			name:                   "org sync source even if app ID is set",
			appId:                  "non-empty-app-id",
			userSyncSource:         types.OktaUserSyncSourceOrg,
			expectedUsers:          validOktaUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceOrg,
			expectedErr:            "",
		},
		{
			name:                   "SAML app sync source",
			appId:                  "non-empty-app-id",
			userSyncSource:         types.OktaUserSyncSourceSamlApp,
			expectedUsers:          validOktaAppUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceSamlApp,
			expectedErr:            "",
		},
		{
			name:                   "SAML app sync source, but app ID missing",
			appId:                  "",
			userSyncSource:         types.OktaUserSyncSourceSamlApp,
			expectedUsers:          nil,
			expectedUserSyncSource: types.OktaUserSyncSourceSamlApp,
			expectedErr:            `user sync source = "saml_app", but Okta SAML app ID is empty`,
		},
		{
			name:                   "unknown sync source and no app ID means org sync source",
			appId:                  "",
			userSyncSource:         types.OktaUserSyncSourceUnknown,
			expectedUsers:          validOktaUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceOrg,
			expectedErr:            "",
		},
		{
			name:                   "unknown sync source and non-empty app ID means SAML app sync source",
			appId:                  "some-app-id",
			userSyncSource:         types.OktaUserSyncSourceUnknown,
			expectedUsers:          validOktaAppUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceSamlApp,
			expectedErr:            "",
		},
		{
			name:                   "empty sync source and no app ID means org sync source",
			appId:                  "",
			userSyncSource:         types.OktaUserSyncSource(""),
			expectedUsers:          validOktaUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceOrg,
			expectedErr:            "",
		},
		{
			name:                   "empty sync source and no non-empty app ID means SAML app sync source",
			appId:                  "some-app-id",
			userSyncSource:         types.OktaUserSyncSource(""),
			expectedUsers:          validOktaAppUsers,
			expectedUserSyncSource: types.OktaUserSyncSourceSamlApp,
			expectedErr:            "",
		},
	}

	for _, tc := range testCases {
		subtest := func(t *testing.T) {
			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			client := newTestOktaClient()
			svc, _ := newTestService(t, ap, client,
				withUserSyncEnabled(tc.userSyncSource),
				withOktaAppID("to-be-replaced-below-app-id"),
				withSSOConnector("fetchOktaUsers-test-sso-connector"),
				withClock(ap.Clock()),
			)
			// Setup App ID. Needs to be set outside constructor to bypass validation.
			svc.oktaSAMLAppID = tc.appId
			// Setup Okta.
			client.OktaUsers = oktaUsers
			client.OktaAppUsers = oktaAppUsers

			users, userSyncSource, err := svc.fetchOktaUsers(t.Context())
			require.Equal(t, tc.expectedUserSyncSource, userSyncSource)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)

				require.Len(t, users, len(tc.expectedUsers))
				for _, u := range tc.expectedUsers {
					require.Contains(t, users, u, "expected user = %q", u)
				}
			}
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, subtest)
		})
	}
}
