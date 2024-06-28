package okta

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
}

// accessListSyncTestContext contains test information for testing the access list synchronizer.
type accessListSyncTestContext struct {
	svc     *accessListSync
	clock   clockwork.FakeClock
	emitter *eventstest.ChannelEmitter
	client  *testOktaClient
	ap      *testAccessPoint
	apps    map[string]types.Application
	groups  map[string]types.UserGroup
}

func (a *accessListSyncTestContext) addApp(app types.Application) {
	a.apps[app.GetName()] = app
}

func (a *accessListSyncTestContext) addGroup(group types.UserGroup) {
	a.groups[group.GetName()] = group
	a.client.addGroupToMapping(group.GetName())
}

func (a *accessListSyncTestContext) advanceAndWaitForSync() {
	a.clock.BlockUntil(1)
	a.clock.Advance(a.svc.syncInterval)
	a.clock.BlockUntil(1)
}

func initAccessListSync(t *testing.T, ctx context.Context) *accessListSyncTestContext {
	t.Helper()

	clock := clockwork.NewFakeClock()
	client := newTestClient()
	ap := newTestAccessPoint(t, clockwork.NewFakeClock())
	emitter := eventstest.NewChannelEmitter(1)
	stopCh := make(chan struct{}, 1)
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	_, err := ap.UpsertRole(ctx, services.NewSystemOktaAccessRole())
	require.NoError(t, err)
	_, err = ap.UpsertRole(ctx, services.NewSystemOktaRequesterRole())
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	alsCtx := &accessListSyncTestContext{
		clock:   clock,
		emitter: emitter,
		client:  client,
		ap:      ap,
		apps:    map[string]types.Application{},
		groups:  map[string]types.UserGroup{},
	}

	alSync, err := newAccessListSync(accessListSyncConfig{
		Clock:       clock,
		ClusterName: testClusterName,
		Client:      client,
		Owners:      []string{"owner1", "owner2"},
		Emitter:     emitter,
		Access:      ap,
		AccessLists: ap,
		OrgURL:      testOrgURL,
		AppsGetter: func() map[string]types.Application {
			return alsCtx.apps
		},
		GroupsGetter: func() map[string]types.UserGroup {
			return alsCtx.groups
		},
		SynchronizerSuccess: &atomic.Bool{},
		SynchronizingMu:     &sync.RWMutex{},
		StopChannel:         stopCh,
	})
	require.NoError(t, err)

	alSync.synchronizerSuccess.Store(true)

	alsCtx.svc = alSync

	t.Cleanup(func() {
		stopCh <- struct{}{}
	})

	// Start the synchronization process in the background.
	go alSync.startSync(context.Background())

	clock.BlockUntil(1)

	return alsCtx
}

func TestAccessListSync(t *testing.T) {
	ctx := context.Background()

	owners := []string{"owner1", "owner2"}

	t.Run("no apps or groups", func(t *testing.T) {
		c := initAccessListSync(t, ctx)
		c.advanceAndWaitForSync()

		require.Empty(t, c.svc.importAccessLists.Clone())
		require.Empty(t, c.svc.newImportAccessLists.Clone())
		require.Empty(t, c.svc.importAccessListMembers.Clone())
		require.Empty(t, c.svc.newImportAccessListMembers.Clone())
		require.Empty(t, c.svc.importRoles.Clone())
		require.Empty(t, c.svc.newImportRoles.Clone())

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap)
	})

	t.Run("Okta apps and groups but no assignments", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{"group2-reviewer"}, []string{"group2"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, c.svc.importAccessListMembers.Clone())
		require.Empty(t, c.svc.newImportAccessListMembers.Clone())
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"group1":          newRole(t, "group1", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
			"group2":          newRole(t, "group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			"group2-reviewer": newReviewerRole(t, "group2"),
		}, c.svc.importRoles.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importRoles.Clone(), c.svc.newImportRoles.Clone(), cmpOpts...))

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
			require.True(t, event.Success)
			require.Equal(t, testClusterName, event.ClusterName)
			require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
			require.Zero(t, event.NumAppFilters)
			require.Zero(t, event.NumGroupFilters)
			require.Zero(t, event.NumApps)
			require.Equal(t, int32(2), event.NumGroups)
			require.Equal(t, int32(4), event.NumRoles)
			require.Equal(t, int32(2), event.NumAccessLists)
			require.Zero(t, event.NumAccessListMembers)
		})

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, "group1", "group2")
	})

	t.Run("Okta apps have assignments, groups have no assignments", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addUserID("user3", "3")
		c.client.addAppAssignments("app1-okta", "1", "2", "3")
		c.client.addAppAssignments("app2-okta", "1", "2", "3")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1", "app1", "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{"app1-reviewer"}, []string{"app1"}, owners),
			"app2":   newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{"group2-reviewer"}, []string{"group2"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1": newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app1/user2": newAccessListMember(t, "app1", "user2", c.clock.Now()),
			"app1/user3": newAccessListMember(t, "app1", "user3", c.clock.Now()),
			"app2/user1": newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2": newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"app2/user3": newAccessListMember(t, "app2", "user3", c.clock.Now()),
		}, c.svc.importAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessListMembers.Clone(), c.svc.newImportAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":            newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			"app1-reviewer":   newReviewerRole(t, "app1"),
			"app2":            newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			"app2-reviewer":   newReviewerRole(t, "app2"),
			"group1":          newRole(t, "group1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta", "app2-okta"}}, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
			"group2":          newRole(t, "group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			"group2-reviewer": newReviewerRole(t, "group2"),
		}, c.svc.importRoles.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importRoles.Clone(), c.svc.newImportRoles.Clone(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
			require.True(t, event.Success)
			require.Equal(t, testClusterName, event.ClusterName)
			require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
			require.Zero(t, event.NumAppFilters)
			require.Zero(t, event.NumGroupFilters)
			require.Equal(t, int32(2), event.NumApps)
			require.Equal(t, int32(2), event.NumGroups)
			require.Equal(t, int32(8), event.NumRoles)
			require.Equal(t, int32(4), event.NumAccessLists)
			require.Equal(t, int32(6), event.NumAccessListMembers)
		})

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, "app1", "app2", "group1", "group2")
	})

	t.Run("Okta apps have assignments, groups have assignments", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addAppAssignments("app1-okta", "1")
		c.client.addAppAssignments("app2-okta", "1", "2")
		c.client.addGroupAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1", "app1", "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{"app1-reviewer"}, []string{"app1"}, owners),
			"app2":   newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{"group2-reviewer"}, []string{"group2"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1":   newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app2/user1":   newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2":   newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"group1/user1": newAccessListMember(t, "group1", "user1", c.clock.Now()),
			"group1/user2": newAccessListMember(t, "group1", "user2", c.clock.Now()),
		}, c.svc.importAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessListMembers.Clone(), c.svc.newImportAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":            newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			"app1-reviewer":   newReviewerRole(t, "app1"),
			"app2":            newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			"app2-reviewer":   newReviewerRole(t, "app2"),
			"group1":          newRole(t, "group1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta", "app2-okta"}}, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
			"group2":          newRole(t, "group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			"group2-reviewer": newReviewerRole(t, "group2"),
		}, c.svc.importRoles.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importRoles.Clone(), c.svc.newImportRoles.Clone(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
			require.True(t, event.Success)
			require.Equal(t, testClusterName, event.ClusterName)
			require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
			require.Zero(t, event.NumAppFilters)
			require.Zero(t, event.NumGroupFilters)
			require.Equal(t, int32(2), event.NumApps)
			require.Equal(t, int32(2), event.NumGroups)
			require.Equal(t, int32(8), event.NumRoles)
			require.Equal(t, int32(4), event.NumAccessLists)
			require.Equal(t, int32(5), event.NumAccessListMembers)
		})

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, "app1", "app2", "group1", "group2")
	})

	t.Run("Okta apps have assignments, groups have assignments, apps and group filters added.", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.svc.groupFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}
		c.svc.appFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addAppAssignments("admin-app1-okta", "1")
		c.client.addAppAssignments("dev-app2-okta", "1", "2")
		c.client.addAppAssignments("dev-app3-okta", "1", "2")
		c.client.addGroupAssignments("admin-group1", "1", "2")
		c.client.addGroupAssignments("dev-group2", "1", "2")
		c.client.addGroupAssignments("dev-group3", "1", "2")

		c.addApp(newAccessListSyncAppLabelAppName(t, "admin-app1"))
		c.addApp(newAccessListSyncAppLabelAppName(t, "dev-app2"))
		c.addApp(newAccessListSyncAppLabelAppName(t, "dev-app3"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "admin-group1"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "dev-group2"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "dev-group3"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"dev-app2":   newAccessList(t, "dev-app2", "dev-app2", []string{"dev-app2-reviewer"}, []string{"dev-app2"}, owners),
			"dev-app3":   newAccessList(t, "dev-app3", "dev-app3", []string{"dev-app3-reviewer"}, []string{"dev-app3"}, owners),
			"dev-group2": newAccessList(t, "dev-group2", "dev-group2", []string{"dev-group2-reviewer"}, []string{"dev-group2"}, owners),
			"dev-group3": newAccessList(t, "dev-group3", "dev-group3", []string{"dev-group3-reviewer"}, []string{"dev-group3"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"dev-app2/user1":   newAccessListMember(t, "dev-app2", "user1", c.clock.Now()),
			"dev-app2/user2":   newAccessListMember(t, "dev-app2", "user2", c.clock.Now()),
			"dev-app3/user1":   newAccessListMember(t, "dev-app3", "user1", c.clock.Now()),
			"dev-app3/user2":   newAccessListMember(t, "dev-app3", "user2", c.clock.Now()),
			"dev-group2/user1": newAccessListMember(t, "dev-group2", "user1", c.clock.Now()),
			"dev-group2/user2": newAccessListMember(t, "dev-group2", "user2", c.clock.Now()),
			"dev-group3/user1": newAccessListMember(t, "dev-group3", "user1", c.clock.Now()),
			"dev-group3/user2": newAccessListMember(t, "dev-group3", "user2", c.clock.Now()),
		}, c.svc.importAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessListMembers.Clone(), c.svc.newImportAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"dev-app2":            newRole(t, "dev-app2", types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app2-okta"}}, nil),
			"dev-app2-reviewer":   newReviewerRole(t, "dev-app2"),
			"dev-app3":            newRole(t, "dev-app3", types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app3-okta"}}, nil),
			"dev-app3-reviewer":   newReviewerRole(t, "dev-app3"),
			"dev-group2":          newRole(t, "dev-group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group2"}}),
			"dev-group2-reviewer": newReviewerRole(t, "dev-group2"),
			"dev-group3":          newRole(t, "dev-group3", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group3"}}),
			"dev-group3-reviewer": newReviewerRole(t, "dev-group3"),
		}, c.svc.importRoles.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importRoles.Clone(), c.svc.newImportRoles.Clone(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
			require.True(t, event.Success)
			require.Equal(t, testClusterName, event.ClusterName)
			require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
			require.Equal(t, int32(1), event.NumAppFilters)
			require.Equal(t, int32(1), event.NumGroupFilters)
			require.Equal(t, int32(2), event.NumApps)
			require.Equal(t, int32(2), event.NumGroups)
			require.Equal(t, int32(8), event.NumRoles)
			require.Equal(t, int32(4), event.NumAccessLists)
			require.Equal(t, int32(8), event.NumAccessListMembers)
		})

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, "dev-app2", "dev-app3", "dev-group2", "dev-group3")
	})

	t.Run("previously existing apps and groups erased or updated, existing fields preserved", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		// Let's set the app/groups counters to an arbitrary value. This should be reset.
		c.svc.appsImported.Store(12)
		c.svc.groupsImported.Store(12)

		// Create a bunch of resources that the reconciler should clean up.
		preserved := newAccessList(t, "app1", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, []string{"some-other-owner"})
		preserved.Spec.MembershipRequires = accesslist.Requires{Roles: []string{"another-role"}}
		preserved.Spec.OwnershipRequires = accesslist.Requires{Roles: []string{"another-role"}}
		preserved.Spec.Audit.NextAuditDate = c.clock.Now().Add(365 * 2 * 24 * time.Hour)
		_, err := c.ap.UpsertAccessList(ctx, preserved)
		require.NoError(t, err)
		_, err = c.ap.UpsertAccessList(ctx, newAccessList(t, "app2", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners))
		require.NoError(t, err)
		_, err = c.ap.UpsertAccessList(ctx, newAccessList(t, "group3", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners))
		require.NoError(t, err)
		_, err = c.ap.UpsertAccessListMember(ctx, newAccessListMember(t, "app1", "user-to-remove", c.clock.Now()))
		require.NoError(t, err)
		_, err = c.ap.UpsertAccessListMember(ctx, newAccessListMember(t, "group3", "user-to-remove", c.clock.Now()))
		require.NoError(t, err)
		_, err = c.ap.UpsertRole(ctx, newRole(t, "app4", nil, nil))
		require.NoError(t, err)

		// Make sure that we see the current imports reflected as above.
		require.NoError(t, c.svc.refreshCurrentImports(ctx))

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   preserved,
			"app2":   newAccessList(t, "app2", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners),
			"group3": newAccessList(t, "group3", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user-to-remove":   newAccessListMember(t, "app1", "user-to-remove", c.clock.Now()),
			"group3/user-to-remove": newAccessListMember(t, "group3", "user-to-remove", c.clock.Now()),
		}, c.svc.importAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app4": newRole(t, "app4", nil, nil),
		}, c.svc.importRoles.Clone(), cmpOpts...))

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addUserID("user-to-remove", "remove")
		c.client.addAppAssignments("app1-okta", "1")
		c.client.addAppAssignments("app2-okta", "1", "2")
		c.client.addGroupAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		preserved.Spec.Title = "app label"
		preserved.Spec.Grants.Roles = []string{"app1"}
		preserved.Spec.OwnerGrants.Roles = []string{"app1-reviewer"}
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   preserved,
			"app2":   newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{"group2-reviewer"}, []string{"group2"}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1":   newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app2/user1":   newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2":   newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"group1/user1": newAccessListMember(t, "group1", "user1", c.clock.Now()),
			"group1/user2": newAccessListMember(t, "group1", "user2", c.clock.Now()),
		}, c.svc.importAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessListMembers.Clone(), c.svc.newImportAccessListMembers.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":            newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			"app1-reviewer":   newReviewerRole(t, "app1"),
			"app2":            newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			"app2-reviewer":   newReviewerRole(t, "app2"),
			"group1":          newRole(t, "group1", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
			"group2":          newRole(t, "group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			"group2-reviewer": newReviewerRole(t, "group2"),
		}, c.svc.importRoles.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importRoles.Clone(), c.svc.newImportRoles.Clone(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)

		expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
			require.True(t, event.Success)
			require.Equal(t, testClusterName, event.ClusterName)
			require.Equal(t, events.OktaAccessListSyncSuccessCode, event.Code)
			require.Zero(t, event.NumAppFilters)
			require.Zero(t, event.NumGroupFilters)
			require.Equal(t, int32(2), event.NumApps)
			require.Equal(t, int32(2), event.NumGroups)
			require.Equal(t, int32(8), event.NumRoles)
			require.Equal(t, int32(4), event.NumAccessLists)
			require.Equal(t, int32(5), event.NumAccessListMembers)
		})

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, "app1", "app2", "group1", "group2")
	})
}

func newAccessListSyncAppWithLabels(t *testing.T, name string, labels map[string]string) types.Application {
	t.Helper()

	app, err := types.NewAppV3(
		types.Metadata{
			Name:        name,
			Description: "app label",
			Labels:      labels,
		},
		types.AppSpecV3{
			URI:        "https://www.link1.com",
			PublicAddr: fmt.Sprintf("%s.%s", name, testClusterName),
		},
	)
	require.NoError(t, err)
	return app
}

func newAccessListSyncApp(t *testing.T, name string) types.Application {
	t.Helper()

	return newAccessListSyncAppWithLabels(t, name, map[string]string{
		types.OriginLabel:             types.OriginOkta,
		types.OktaAppNameLabel:        "app label",
		types.OktaAppDescriptionLabel: "applink-name1",
		eteleport.OktaOrgURLLabel:     testOrgURL,
		eteleport.OktaAppIDLabel:      name + "-okta",
	})
}

func newAccessListSyncAppLabelAppName(t *testing.T, name string) types.Application {
	t.Helper()

	return newAccessListSyncAppWithLabels(t, name, map[string]string{
		types.OriginLabel:             types.OriginOkta,
		types.OktaAppNameLabel:        name,
		types.OktaAppDescriptionLabel: "applink-name1",
		eteleport.OktaOrgURLLabel:     testOrgURL,
		eteleport.OktaAppIDLabel:      name + "-okta",
	})
}

func newAccessListSyncGroupWithLabels(t *testing.T, name string, labels map[string]string, apps ...string) types.UserGroup {
	t.Helper()

	group, err := types.NewUserGroup(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.UserGroupSpecV1{
		Applications: apps,
	})
	require.NoError(t, err)
	return group
}

func newAccessListSyncGroup(t *testing.T, name string, apps ...string) types.UserGroup {
	t.Helper()

	return newAccessListSyncGroupWithLabels(t, name, map[string]string{
		types.OriginLabel:               types.OriginOkta,
		types.OktaGroupNameLabel:        "group label",
		types.OktaGroupDescriptionLabel: "group description",
		eteleport.OktaOrgURLLabel:       testOrgURL,
		eteleport.OktaGroupIDLabel:      name,
	}, apps...)
}

func newAccessListSyncGroupLabelGroupName(t *testing.T, name string) types.UserGroup {
	t.Helper()

	return newAccessListSyncGroupWithLabels(t, name, map[string]string{
		types.OriginLabel:               types.OriginOkta,
		types.OktaGroupNameLabel:        name,
		types.OktaGroupDescriptionLabel: "group description",
		eteleport.OktaOrgURLLabel:       testOrgURL,
		eteleport.OktaGroupIDLabel:      name,
	})
}

var expectedLabels = map[string]string{
	types.OriginLabel:                  types.OriginOkta,
	types.TeleportInternalResourceType: types.SystemResource,
	eteleport.OktaOrgURLLabel:          testOrgURL,
}

func newAccessList(t *testing.T, name, title string, ownerGrantRoles, grantRoles, ownerNames []string) *accesslist.AccessList {
	t.Helper()

	owners := make([]accesslist.Owner, len(ownerNames))
	for i, owner := range ownerNames {
		owners[i] = accesslist.Owner{
			Name:        owner,
			Description: "default owner from access list synchronizer",
		}
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name:   name,
		Labels: expectedLabels,
	}, accesslist.Spec{
		Title: title,
		OwnerGrants: accesslist.Grants{
			Roles: ownerGrantRoles,
		},
		Grants: accesslist.Grants{
			Roles: grantRoles,
		},
		Owners: owners,
	})
	require.NoError(t, err)

	return accessList
}

func newAccessListMember(t *testing.T, accessListName, memberName string, joined time.Time) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(header.Metadata{
		Name:   memberName,
		Labels: expectedLabels,
	}, accesslist.AccessListMemberSpec{
		AccessList: accessListName,
		Name:       memberName,
		Joined:     joined,
		AddedBy:    ImporterName,
	})
	require.NoError(t, err)

	return member
}

func newReviewerRole(t *testing.T, grantRoleName string) types.Role {
	t.Helper()

	role, err := types.NewRole(grantRoleName+ReviewerSuffix, types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{grantRoleName},
			},
		},
	})
	require.NoError(t, err)
	role.SetStaticLabels(expectedLabels)
	return role
}

func newRole(t *testing.T, name string, appLabels, groupLabels types.Labels) types.Role {
	t.Helper()

	var rules []types.Rule
	if len(groupLabels) > 0 {
		rules = append(rules, types.NewRule(types.KindUserGroup, services.RO()))
	}

	role, err := types.NewRole(name, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules:       rules,
			AppLabels:   appLabels,
			GroupLabels: groupLabels,
		},
	})
	require.NoError(t, err)
	role.SetStaticLabels(expectedLabels)
	return role
}

func verifyServiceMatchesBackend(t *testing.T, ap *testAccessPoint, svc *accessListSync) {
	t.Helper()

	accessLists := svc.importAccessLists.Clone()
	accessListMembers := svc.importAccessListMembers.Clone()
	roles := svc.importRoles.Clone()

	ctx := context.Background()
	for _, accessList := range accessLists {
		get, err := ap.GetAccessList(ctx, accessList.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(accessList, get, cmpOpts...))
	}

	for _, accessListMember := range accessListMembers {
		get, err := ap.GetAccessListMember(ctx, accessListMember.Spec.AccessList, accessListMember.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(accessListMember, get, cmpOpts...))
	}

	for _, role := range roles {
		get, err := ap.GetRole(ctx, role.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(role, get, cmpOpts...))
	}
}

func expectOktaAccessRequesterSearchAsRoles(t *testing.T, ctx context.Context, ap *testAccessPoint, expectedRoles ...string) {
	role, err := ap.Access.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedRoles, role.GetSearchAsRoles(types.Allow))
}
