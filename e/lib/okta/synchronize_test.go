package okta

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
)

func TestSynchronizeGroups(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, client, emitter := newTestService(t, ap)
	svc.leadershipAcquired.Store(true)

	// Add in one app to get a group to app mapping from
	client.oktaApps = []okta.App{
		// This app should be added.
		&okta.Application{
			Id:     "app1",
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
	}
	client.appsToGroups["app1"] = []string{"group4"}

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

	// These should be created.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Empty(t, group3.GetApplications())
	group4, err := ap.GetUserGroup(ctx, "group4")
	require.NoError(t, err)
	require.Equal(t, "group name (group 4 description)", group4.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, svc.hash, "app1", "applink-name1")}, group4.GetApplications())

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

	// App1 is now assigned to group3 as well.
	client.appsToGroups["app1"] = []string{"group3", "group4"}

	require.NoError(t, svc.synchronize(ctx))

	// Verify app1 is now in group3's application list.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, svc.hash, "app1", "applink-name1")}, group3.GetApplications())

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

	// Let's explicitly set group3 to have no applications to simulate the apps and group
	// mappings getting out of sync with one another.
	group3.SetApplications(nil)
	require.NoError(t, ap.UpdateUserGroup(ctx, group3))

	svc.groupsMu.Lock()
	svc.groups[group3.GetName()] = group3
	svc.groupsMu.Unlock()

	require.NoError(t, svc.synchronize(ctx))

	// Verify app1 is now in group3's application list.
	group3, err = ap.GetUserGroup(ctx, "group3")
	require.NoError(t, err)
	require.Equal(t, "group name (group 3 description)", group3.GetMetadata().Description)
	require.Equal(t, []string{mustAppName(t, svc.hash, "app1", "applink-name1")}, group3.GetApplications())

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})

	// We need to simulate the user group backend getting out of sync with the reconciler here.

	// This will cause create to be re-run on group 3, which should be handled.
	delete(svc.groups, group3.GetName())

	// This will cause update to be run on a non-existent group, which should be handled.
	require.NoError(t, ap.DeleteUserGroup(ctx, group4.GetName()))
	svc.groups[group4.GetName()].GetMetadata().Labels["dummy"] = "update"

	require.NoError(t, svc.synchronize(ctx))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(1), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(0), event.Deleted)
	})

	// This will cause delete to be run on a non-existent group, which should be handled.
	require.NoError(t, ap.DeleteUserGroup(ctx, group4.GetName()))
	client.oktaGroups = client.oktaGroups[0:1]

	require.NoError(t, svc.synchronize(ctx))

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaGroupsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaGroupsUpdateCode, event.GetCode())
		require.Equal(t, int32(0), event.Added)
		require.Equal(t, int32(0), event.Updated)
		require.Equal(t, int32(1), event.Deleted)
	})
}

func TestSynchronizeApplications(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, client, emitter := newTestService(t, ap)
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
	client.appsToGroups["app4"] = []string{"group4"}

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
	require.Equal(t, []string{"group4"}, app4Link1.GetUserGroups())
	app4Link2Name, err := appName(svc.hash, "app4", "applink-name2")
	require.NoError(t, err)
	app4Link2 := apps[app4Link2Name]
	require.Equal(t, "https://www.link2.com", app4Link2.GetURI())
	require.Equal(t, []string{"group4"}, app4Link2.GetUserGroups())

	expectAuditEvent(t, emitter, func(event *apievents.OktaResourcesUpdate) {
		require.Equal(t, events.OktaApplicationsUpdateEvent, event.GetType())
		require.Equal(t, events.OktaApplicationsUpdateCode, event.GetCode())
		require.Equal(t, int32(2), event.Added)
		require.Equal(t, int32(1), event.Updated)
		require.Equal(t, int32(2), event.Deleted)
	})
}

func TestEmitSyncEventsInBatches(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, _, emitter := newTestService(t, ap)
	require.NoError(t, svc.startSynchronizerReconcilers(ctx))

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
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, _, _ := newTestService(t, ap)
	clock := clockwork.NewFakeClock()
	svc.clock = clock
	svc.timeBetweenSyncs = time.Second * 10

	ticker, timeBetweenSyncs := svc.setupSynchronizerTicker(ctx)
	require.Equal(t, svc.timeBetweenSyncs, timeBetweenSyncs)

	// 10 second timeout expected here.
	advanceAndDontExpectSignal(t, clock, time.Second*5, ticker)
	advanceAndExpectSignal(t, clock, time.Second*20, ticker)

	// Set the time between syncs to 300 seconds
	pref, err := ap.GetAuthPreference(ctx)
	require.NoError(t, err)
	pref.SetOktaSyncPeriod(time.Second * 300)
	require.NoError(t, ap.SetAuthPreference(ctx, pref))

	timeBetweenSyncs = svc.updateSynchronizerTicker(ctx, ticker, timeBetweenSyncs)

	// The next tick should require 300 seconds.
	advanceAndDontExpectSignal(t, clock, time.Second*20, ticker)
	advanceAndExpectSignal(t, clock, time.Second*310, ticker)

	// Set the duration back to zero, should set the ticker back to 10 seconds.
	pref.SetOktaSyncPeriod(0)
	require.NoError(t, ap.SetAuthPreference(ctx, pref))

	svc.updateSynchronizerTicker(ctx, ticker, timeBetweenSyncs)
	advanceAndExpectSignal(t, clock, time.Second*20, ticker)
}

func advanceAndExpectSignal(t *testing.T, clock clockwork.FakeClock, advance time.Duration, ticker clockwork.Ticker) {
	t.Helper()

	// Make sure the ticker is registered as a waiter.
	clock.BlockUntil(1)

	clock.Advance(advance)
	select {
	case <-ticker.Chan():
	default:
		require.Fail(t, "signal expected on channel")
	}
}

func advanceAndDontExpectSignal(t *testing.T, clock clockwork.FakeClock, advance time.Duration, ticker clockwork.Ticker) {
	t.Helper()

	// Make sure the ticker is registered as a waiter.
	clock.BlockUntil(1)

	clock.Advance(advance)
	select {
	case <-ticker.Chan():
		require.Fail(t, "no signal expected on channel")
	default:
	}
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
	}, types.UserGroupSpecV1{})
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

func genEventResources(numResources int, descPrefix string) []*apievents.OktaResource {
	resources := make([]*apievents.OktaResource, numResources)

	for i := 0; i < numResources; i++ {
		resources[i] = &apievents.OktaResource{
			ID:          fmt.Sprintf("%d", i),
			Description: fmt.Sprintf("%s %d", descPrefix, i),
		}
	}

	return resources
}

func verifyEventResources(t *testing.T, resources []*apievents.OktaResource, offset, numResources int, descPrefix string) []*apievents.OktaResource {
	require.Len(t, resources, numResources)

	for i := 0; i < numResources; i++ {
		index := offset + i
		require.Equal(t, fmt.Sprintf("%d", index), resources[i].ID)
		require.Equal(t, fmt.Sprintf("%s %d", descPrefix, index), resources[i].Description)
	}

	return resources
}
