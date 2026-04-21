package okta

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaapitest "github.com/gravitational/teleport/e/lib/okta/api/apitest"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	// IneligibleStatus is a dynamic field calculated by the ineligibility reconciler
	// or in case of empty requirements preset to eligible
	// Note that IneligibleStatus is FE only consumed field and doesn't have any
	// impact on access list functionality or RBAC.
	cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
	cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "IneligibleStatus"),
}

// accessListSyncTestContext contains test information for testing the access list synchronizer.
type accessListSyncTestContext struct {
	svc     *accessListSync
	clock   clocki.FakeClock
	emitter *eventstest.ChannelEmitter
	ap      *testAccessPoint

	apps   map[string]types.AppServer
	groups map[string]types.UserGroup

	oktaClient *oktaapitest.Client
	oktaData   *oktaapitest.LocalData
}

func (a *accessListSyncTestContext) addApp(app types.AppServer) types.AppServer {
	a.apps[app.GetName()] = app
	return app
}

func (a *accessListSyncTestContext) appAccessRoleName(id string) string {
	v, ok := a.apps[id]
	if !ok {
		return ""
	}
	displayText, _ := v.GetLabel(types.OktaAppNameLabel)
	return common.CreateOktaAccessRoleFriendlyName(displayText, id)
}
func (a *accessListSyncTestContext) appReviewerRoleName(id string) string {
	v, ok := a.apps[id]
	if !ok {
		return ""
	}
	displayText, _ := v.GetLabel(types.OktaAppNameLabel)
	return common.CreateOktaReviewerRoleFriendlyName(displayText, id)
}
func (a *accessListSyncTestContext) groupAccessRoleName(id string) string {
	v, ok := a.groups[id]
	if !ok {
		return ""
	}
	displayText, _ := v.GetLabel(types.OktaGroupNameLabel)
	return common.CreateOktaAccessRoleFriendlyName(displayText, id)
}
func (a *accessListSyncTestContext) groupReviewerRoleName(id string) string {
	v, ok := a.groups[id]
	if !ok {
		return ""
	}
	displayText, _ := v.GetLabel(types.OktaGroupNameLabel)
	return common.CreateOktaReviewerRoleFriendlyName(displayText, id)
}

func (a *accessListSyncTestContext) addGroup(group types.UserGroup) {
	a.groups[group.GetName()] = group
	a.oktaData.UpsertGroupForId(oktaapi.OktaGroupID(group.GetName()))
}

func initAccessListSync(t *testing.T, ctx context.Context) *accessListSyncTestContext {
	t.Helper()

	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clockwork.NewFakeClock())
	emitter := eventstest.NewChannelEmitter(1)
	stopCh := make(chan struct{}, 1)

	_, err := ap.UpsertRole(ctx, services.NewSystemOktaAccessRole(modules.BuildEnterprise))
	require.NoError(t, err)
	_, err = ap.UpsertRole(ctx, services.NewSystemOktaRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)

	alsCtx := &accessListSyncTestContext{
		clock:   clock,
		emitter: emitter,
		ap:      ap,

		apps:   map[string]types.AppServer{},
		groups: map[string]types.UserGroup{},

		oktaClient: oktaClient,
		oktaData:   oktaData,
	}

	alSync, err := newAccessListSync(accessListSyncConfig{
		Clock:       clock,
		ClusterName: testClusterName,
		Client:      oktaClient,
		Owners:      []string{"owner1", "owner2"},
		Emitter:     emitter,
		Access:      ap,
		AccessLists: ap,
		OrgURL:      oktaapitest.TestOrgURL,
		AppsGetter: func() map[string]types.AppServer {
			return alsCtx.apps
		},
		GroupsGetter: func() map[string]types.UserGroup {
			return alsCtx.groups
		},
		StopChannel:   stopCh,
		ServiceStatus: nullStatusUpdate{},
		Backend:       ap,
	})
	require.NoError(t, err)

	alsCtx.svc = alSync

	t.Cleanup(func() {
		stopCh <- struct{}{}
	})

	alSync.init(t.Context())

	return alsCtx
}

func TestAccessListSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owners := []string{"owner1", "owner2"}

	t.Run("no apps or groups", func(t *testing.T) {
		c := initAccessListSync(t, ctx)
		c.svc.sync(ctx)

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

		c.addApp(newAccessListSyncAppServer(t, "app1"))
		c.addApp(newAccessListSyncAppServer(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))
		c.svc.sync(ctx)

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"group1": newAccessList(t, "group1", "group label", []string{c.groupReviewerRoleName("group1")}, []string{c.groupAccessRoleName("group1")}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{c.groupReviewerRoleName("group2")}, []string{c.groupAccessRoleName("group2")}, owners),
		}, c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.importAccessLists.Clone(), c.svc.importAccessLists.Clone(), cmpOpts...))
		require.Empty(t, c.svc.importAccessListMembers.Clone())
		require.Empty(t, c.svc.newImportAccessListMembers.Clone())
		require.Empty(t, cmp.Diff(map[string]types.Role{
			c.groupAccessRoleName("group1"):   newRole(t, c.groupAccessRoleName("group1"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			c.groupReviewerRoleName("group1"): newReviewerRole(t, c.groupReviewerRoleName("group1"), c.groupAccessRoleName("group1")),
			c.groupAccessRoleName("group2"):   newRole(t, c.groupAccessRoleName("group2"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			c.groupReviewerRoleName("group2"): newReviewerRole(t, c.groupReviewerRoleName("group2"), c.groupAccessRoleName("group2")),
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, c.groupAccessRoleName("group1"), c.groupAccessRoleName("group2"))
	})

	t.Run("Okta apps have assignments, groups have no assignments", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.oktaData.UpsertUserForId("user1", "1")
		c.oktaData.UpsertUserForId("user2", "2")
		c.oktaData.UpsertUserForId("user3", "3")
		c.oktaData.UpsertAppForId("app1-okta")
		c.oktaData.UpsertAppForId("app2-okta")
		c.oktaData.UpsertAppUserAssignments("app1-okta", "1", "2", "3")
		c.oktaData.UpsertAppUserAssignments("app2-okta", "1", "2", "3")

		c.addApp(newAccessListSyncAppServer(t, "app1"))
		c.addApp(newAccessListSyncAppServer(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1", "app1", "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.svc.sync(ctx)

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{c.appReviewerRoleName("app1")}, []string{c.appAccessRoleName("app1")}, owners),
			"app2":   newAccessList(t, "app2", "app label", []string{c.appReviewerRoleName("app2")}, []string{c.appAccessRoleName("app2")}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{c.groupReviewerRoleName("group1")}, []string{c.groupAccessRoleName("group1")}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{c.groupReviewerRoleName("group2")}, []string{c.groupAccessRoleName("group2")}, owners),
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
			c.appAccessRoleName("app1"):       newRole(t, c.appAccessRoleName("app1"), types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			c.appReviewerRoleName("app1"):     newReviewerRole(t, c.appReviewerRoleName("app1"), c.appAccessRoleName("app1")),
			c.appAccessRoleName("app2"):       newRole(t, c.appAccessRoleName("app2"), types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			c.appReviewerRoleName("app2"):     newReviewerRole(t, c.appReviewerRoleName("app2"), c.appAccessRoleName("app2")),
			c.groupAccessRoleName("group1"):   newRole(t, c.groupAccessRoleName("group1"), types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta", "app2-okta"}}, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			c.groupReviewerRoleName("group1"): newReviewerRole(t, c.groupReviewerRoleName("group1"), c.groupAccessRoleName("group1")),
			c.groupAccessRoleName("group2"):   newRole(t, c.groupAccessRoleName("group2"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			c.groupReviewerRoleName("group2"): newReviewerRole(t, c.groupReviewerRoleName("group2"), c.groupAccessRoleName("group2")),
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, c.appAccessRoleName("app1"), c.appAccessRoleName("app2"), c.groupAccessRoleName("group1"), c.groupAccessRoleName("group2"))
	})

	t.Run("Okta apps have assignments, groups have assignments", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.oktaData.UpsertUserForId("user1", "1")
		c.oktaData.UpsertUserForId("user2", "2")
		c.oktaData.UpsertAppForId("app1-okta")
		c.oktaData.UpsertAppForId("app2-okta")
		c.oktaData.UpsertGroupForId("group1")

		c.oktaData.UpsertAppUserAssignments("app1-okta", "1")
		c.oktaData.UpsertAppUserAssignments("app2-okta", "1", "2")
		c.oktaData.UpsertGroupUserAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncAppServer(t, "app1"))
		c.addApp(newAccessListSyncAppServer(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1", "app1", "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.svc.sync(ctx)

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{c.appReviewerRoleName("app1")}, []string{c.appAccessRoleName("app1")}, owners),
			"app2":   newAccessList(t, "app2", "app label", []string{c.appReviewerRoleName("app2")}, []string{c.appAccessRoleName("app2")}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{c.groupReviewerRoleName("group1")}, []string{c.groupAccessRoleName("group1")}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{c.groupReviewerRoleName("group2")}, []string{c.groupAccessRoleName("group2")}, owners),
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
			c.appAccessRoleName("app1"):       newRole(t, c.appAccessRoleName("app1"), types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			c.appReviewerRoleName("app1"):     newReviewerRole(t, c.appReviewerRoleName("app1"), c.appAccessRoleName("app1")),
			c.appAccessRoleName("app2"):       newRole(t, c.appAccessRoleName("app2"), types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			c.appReviewerRoleName("app2"):     newReviewerRole(t, c.appReviewerRoleName("app2"), c.appAccessRoleName("app2")),
			c.groupAccessRoleName("group1"):   newRole(t, c.groupAccessRoleName("group1"), types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta", "app2-okta"}}, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			c.groupReviewerRoleName("group1"): newReviewerRole(t, c.groupReviewerRoleName("group1"), c.groupAccessRoleName("group1")),
			c.groupAccessRoleName("group2"):   newRole(t, c.groupAccessRoleName("group2"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			c.groupReviewerRoleName("group2"): newReviewerRole(t, c.groupReviewerRoleName("group2"), c.groupAccessRoleName("group2")),
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap, c.appAccessRoleName("app1"), c.appAccessRoleName("app2"), c.groupAccessRoleName("group1"), c.groupAccessRoleName("group2"))
	})

	t.Run("Okta apps have assignments, groups have assignments, apps and group filters added.", func(t *testing.T) {
		c := initAccessListSync(t, ctx)

		c.svc.groupFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}
		c.svc.appFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}

		c.oktaData.UpsertUserForId("user1", "1")
		c.oktaData.UpsertUserForId("user2", "2")
		c.oktaData.UpsertAppForId("admin-app1-okta")
		c.oktaData.UpsertAppForId("dev-app2-okta")
		c.oktaData.UpsertAppForId("dev-app3-okta")
		c.oktaData.UpsertGroupForId("admin-group1")
		c.oktaData.UpsertGroupForId("dev-group2")
		c.oktaData.UpsertGroupForId("dev-group3")

		c.oktaData.UpsertAppUserAssignments("admin-app1-okta", "1")
		c.oktaData.UpsertAppUserAssignments("dev-app2-okta", "1", "2")
		c.oktaData.UpsertAppUserAssignments("dev-app3-okta", "1", "2")
		c.oktaData.UpsertGroupUserAssignments("admin-group1", "1", "2")
		c.oktaData.UpsertGroupUserAssignments("dev-group2", "1", "2")
		c.oktaData.UpsertGroupUserAssignments("dev-group3", "1", "2")

		c.addApp(newAccessListSyncAppServerLabelAppName(t, "admin-app1"))
		c.addApp(newAccessListSyncAppServerLabelAppName(t, "dev-app2"))
		c.addApp(newAccessListSyncAppServerLabelAppName(t, "dev-app3"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "admin-group1"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "dev-group2"))
		c.addGroup(newAccessListSyncGroupLabelGroupName(t, "dev-group3"))

		c.svc.sync(ctx)

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"dev-app2":   newAccessList(t, "dev-app2", "dev-app2", []string{c.appReviewerRoleName("dev-app2")}, []string{c.appAccessRoleName("dev-app2")}, owners),
			"dev-app3":   newAccessList(t, "dev-app3", "dev-app3", []string{c.appReviewerRoleName("dev-app3")}, []string{c.appAccessRoleName("dev-app3")}, owners),
			"dev-group2": newAccessList(t, "dev-group2", "dev-group2", []string{c.groupReviewerRoleName("dev-group2")}, []string{c.groupAccessRoleName("dev-group2")}, owners),
			"dev-group3": newAccessList(t, "dev-group3", "dev-group3", []string{c.groupReviewerRoleName("dev-group3")}, []string{c.groupAccessRoleName("dev-group3")}, owners),
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
			c.appAccessRoleName("dev-app2"):       newRole(t, c.appAccessRoleName("dev-app2"), types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app2-okta"}}, nil),
			c.appReviewerRoleName("dev-app2"):     newReviewerRole(t, c.appReviewerRoleName("dev-app2"), c.appAccessRoleName("dev-app2")),
			c.appAccessRoleName("dev-app3"):       newRole(t, c.appAccessRoleName("dev-app3"), types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app3-okta"}}, nil),
			c.appReviewerRoleName("dev-app3"):     newReviewerRole(t, c.appReviewerRoleName("dev-app3"), c.appAccessRoleName("dev-app3")),
			c.groupAccessRoleName("dev-group2"):   newRole(t, c.groupAccessRoleName("dev-group2"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group2"}}),
			c.groupReviewerRoleName("dev-group2"): newReviewerRole(t, c.groupReviewerRoleName("dev-group2"), c.groupAccessRoleName("dev-group2")),
			c.groupAccessRoleName("dev-group3"):   newRole(t, c.groupAccessRoleName("dev-group3"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group3"}}),
			c.groupReviewerRoleName("dev-group3"): newReviewerRole(t, c.groupReviewerRoleName("dev-group3"), c.groupAccessRoleName("dev-group3")),
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap,
			c.appAccessRoleName("dev-app2"),
			c.appAccessRoleName("dev-app3"),
			c.groupAccessRoleName("dev-group2"),
			c.groupAccessRoleName("dev-group3"))
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

		c.oktaData.UpsertUserForId("user1", "1")
		c.oktaData.UpsertUserForId("user2", "2")
		c.oktaData.UpsertUserForId("user-to-remove", "remove")
		c.oktaData.UpsertAppForId("app1-okta")
		c.oktaData.UpsertAppForId("app2-okta")
		c.oktaData.UpsertGroupForId("group1")

		c.oktaData.UpsertAppUserAssignments("app1-okta", "1")
		c.oktaData.UpsertAppUserAssignments("app2-okta", "1", "2")
		c.oktaData.UpsertGroupUserAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncAppServer(t, "app1"))
		c.addApp(newAccessListSyncAppServer(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.svc.sync(ctx)

		preserved.Spec.Title = "app label"
		preserved.Spec.Grants.Roles = []string{c.appAccessRoleName("app1")}
		preserved.Spec.OwnerGrants.Roles = []string{c.appReviewerRoleName("app1")}
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   preserved,
			"app2":   newAccessList(t, "app2", "app label", []string{c.appReviewerRoleName("app2")}, []string{c.appAccessRoleName("app2")}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{c.groupReviewerRoleName("group1")}, []string{c.groupAccessRoleName("group1")}, owners),
			"group2": newAccessList(t, "group2", "group label", []string{c.groupReviewerRoleName("group2")}, []string{c.groupAccessRoleName("group2")}, owners),
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
			c.appAccessRoleName("app1"):       newRole(t, c.appAccessRoleName("app1"), types.Labels{eteleport.OktaAppIDLabel: []string{"app1-okta"}}, nil),
			c.appReviewerRoleName("app1"):     newReviewerRole(t, c.appReviewerRoleName("app1"), c.appAccessRoleName("app1")),
			c.appAccessRoleName("app2"):       newRole(t, c.appAccessRoleName("app2"), types.Labels{eteleport.OktaAppIDLabel: []string{"app2-okta"}}, nil),
			c.appReviewerRoleName("app2"):     newReviewerRole(t, c.appReviewerRoleName("app2"), c.appAccessRoleName("app2")),
			c.groupAccessRoleName("group1"):   newRole(t, c.groupAccessRoleName("group1"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			c.groupReviewerRoleName("group1"): newReviewerRole(t, c.groupReviewerRoleName("group1"), c.groupAccessRoleName("group1")),
			c.groupAccessRoleName("group2"):   newRole(t, c.groupAccessRoleName("group2"), nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group2"}}),
			c.groupReviewerRoleName("group2"): newReviewerRole(t, c.groupReviewerRoleName("group2"), c.groupAccessRoleName("group2")),
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

		expectOktaAccessRequesterSearchAsRoles(t, ctx, c.ap,
			c.appAccessRoleName("app1"),
			c.appAccessRoleName("app2"),
			c.groupAccessRoleName("group1"),
			c.groupAccessRoleName("group2"),
		)
	})

	t.Run("Failed group import", func(t *testing.T) {
		groups := []string{"group1", "group2", "group3"}

		testCases := []struct {
			name string
			// importErrors describes the error to return when the okta client
			// is queried for a given groups's assigned members. Defaults to
			// returning the test client's configured member list for all
			// groups not specified in the map.
			importErrors map[oktaGroupID]error

			// assertEvent asserts the properties of the OktaAccessListSync event
			// generated by the sync process
			assertEvent func(*testing.T, *apievents.OktaAccessListSync)

			// expectAccessListErrors describes the error expectation when
			// testing for the existence of an AccessList. Defaults to `require.NoError`
			// for all Access Lists not specified in the map
			expectAccessListErrors map[string]require.ErrorAssertionFunc
		}{
			{
				name: "Error cancels sync",
				importErrors: map[oktaGroupID]error{
					"group2": errors.New("Some transient error"),
				},
				assertEvent: func(t *testing.T, event *apievents.OktaAccessListSync) {
					require.False(t, event.Success)
					require.Less(t, event.NumGroups, int32(len(groups)))
				},
				expectAccessListErrors: map[string]require.ErrorAssertionFunc{},
			}, {
				name: "Not found is not an error",
				importErrors: map[oktaGroupID]error{
					"group2": trace.NotFound("No such group"),
				},
				assertEvent: func(t *testing.T, event *apievents.OktaAccessListSync) {
					require.True(t, event.Success)
					require.Equal(t, int32(len(groups)-1), event.NumGroups)
				},
				expectAccessListErrors: map[string]require.ErrorAssertionFunc{
					"group2": requireNotFound,
				},
			},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {

				// GIVEN an Okta integration with multiple synced groups
				c := initAccessListSync(t, ctx)
				for _, grp := range groups {
					c.addGroup(newAccessListSyncGroup(t, grp))
				}
				c.svc.sync(ctx)
				expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
					require.True(t, event.Success)
					require.Equal(t, int32(3), event.NumGroups)
				})

				// ALSO GIVEN an Okta client that is rigged to fail when importing a
				// specific Okta group
				oldGetGroupAssignmentsFunc := c.oktaClient.GetGroupAssignmentsFunc
				c.oktaClient.GetGroupAssignmentsFunc =
					func(t *testing.T, ctx context.Context, groupID oktaGroupID) ([]oktaUserID, error) {
						if err, ok := tt.importErrors[groupID]; ok {
							return nil, err
						}
						return oldGetGroupAssignmentsFunc(t, ctx, groupID)
					}

				// WHEN I force a new Access List Sync
				c.svc.sync(ctx)

				// EXPECT that an appropriate sync event has been emitted
				tt.assertEvent(t, requireAuditEvent[*apievents.OktaAccessListSync](t, c.emitter))

				// EXPECT that all of the Teleport Access Lists representing the Okta
				// Groups have been appropriately preserved or deleted after the
				// sync
				for _, grp := range groups {
					_, err := c.ap.AccessLists.GetAccessList(context.Background(), grp)

					assertErrorValue, hasCustom := tt.expectAccessListErrors[grp]
					if !hasCustom {
						assertErrorValue = require.NoError
					}
					assertErrorValue(t, err, "Access List %q", grp)
				}
			})
		}
	})

	t.Run("Failed app import", func(t *testing.T) {
		appNames := []string{"app1", "app2", "app3"}

		testCases := []struct {
			name string

			// importErrors describes the error to return when the okta client
			// is queried for a given app's assigned users. Defaults to
			// returning the test client's configured assignment list for all
			// apps not specified in the map.
			importErrors map[oktaAppID]error

			// assertEvent asserts the properties of the OktaAccessListSync event
			// generated by the sync process
			assertEvent func(*testing.T, *apievents.OktaAccessListSync)

			// expectAccessListErrors describes the error expectation when
			// testing for the existence of an AccessList. Defaults to `require.NoError`
			// for all Access Lists not specified in the map.
			expectAccessListErrors map[string]require.ErrorAssertionFunc
		}{
			{
				name: "Error cancels sync",
				importErrors: map[oktaAppID]error{
					"app2-okta": errors.New("Some transient error"),
				},
				assertEvent: func(t *testing.T, event *apievents.OktaAccessListSync) {
					require.False(t, event.Success)
					require.Less(t, event.NumApps, int32(len(appNames)))
				},
			},
			{
				name: "Not found is not an error",
				importErrors: map[oktaAppID]error{
					"app2-okta": trace.NotFound("No such app"),
				},
				assertEvent: func(t *testing.T, event *apievents.OktaAccessListSync) {
					require.True(t, event.Success)
					require.Equal(t, int32(len(appNames)-1), event.NumApps)
				},
				expectAccessListErrors: map[string]require.ErrorAssertionFunc{
					"app2": requireNotFound,
				},
			},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				// GIVEN an Okta integration with multiple synced users and apps
				c := initAccessListSync(t, ctx)
				c.oktaData.UpsertUserForId("user1", "1")
				c.oktaData.UpsertUserForId("user2", "2")
				for _, appName := range appNames {
					app := c.addApp(newAccessListSyncAppServer(t, appName))
					appID, _ := app.GetLabel(eteleport.OktaAppIDLabel)
					c.oktaData.UpsertAppForId(oktaapi.OktaAppID(appID))
					c.oktaData.UpsertAppUserAssignments(oktaapi.OktaAppID(appID), "1", "2")
				}
				c.svc.sync(ctx)
				expectAuditEvent(t, c.emitter, func(event *apievents.OktaAccessListSync) {
					require.True(t, event.Success)
					require.Equal(t, int32(3), event.NumApps)
				})

				// ALSO GIVEN an Okta client that is rigged to fail when importing a
				// specific Okta app
				oldGetAppAssignmentsFunc := c.oktaClient.GetAppAssignmentsFunc
				c.oktaClient.GetAppAssignmentsFunc =
					func(t *testing.T, ctx context.Context, app oktaapi.OktaAppID) ([]oktaapi.AppAssignment, error) {
						if err, ok := tt.importErrors[app]; ok {
							return nil, err
						}
						return oldGetAppAssignmentsFunc(t, ctx, app)
					}

				// WHEN I force a new Access List Sync
				c.svc.sync(ctx)

				// EXPECT that an appropriate sync event has been emitted
				tt.assertEvent(t, requireAuditEvent[*apievents.OktaAccessListSync](t, c.emitter))

				// EXPECT that all of the Teleport Access Lists representing the Okta
				// Apps have been appropriately preserved or deleted after the
				// sync
				for _, appName := range appNames {
					_, err := c.ap.AccessLists.GetAccessList(context.Background(), appName)
					assertErrorValue, hasCustom := tt.expectAccessListErrors[appName]
					if !hasCustom {
						assertErrorValue = require.NoError
					}
					assertErrorValue(t, err, "Access list %q", appName)
				}
			})
		}
	})
}

func newAccessListSyncAppServerWithLabels(t *testing.T, name string, labels map[string]string) types.AppServer {
	t.Helper()

	return newAppServer(t,
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
}

func newAccessListSyncAppServer(t *testing.T, name string) types.AppServer {
	t.Helper()

	return newAccessListSyncAppServerWithLabels(t, name, map[string]string{
		types.OriginLabel:             types.OriginOkta,
		types.OktaAppNameLabel:        "app label",
		types.OktaAppDescriptionLabel: "applink-name1",
		eteleport.OktaOrgURLLabel:     testOrgURL,
		eteleport.OktaAppIDLabel:      name + "-okta",
	})
}

func newAccessListSyncAppServerLabelAppName(t *testing.T, name string) types.AppServer {
	t.Helper()

	return newAccessListSyncAppServerWithLabels(t, name, map[string]string{
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
	accessList.Status = accesslist.Status{
		OwnerOf:  []string{},
		MemberOf: []string{},
	}

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

func newReviewerRole(t *testing.T, roleName string, reviewRequestRole string) types.Role {
	t.Helper()

	role, err := types.NewRoleWithVersion(roleName, types.V7, types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{reviewRequestRole},
			},
		},
	})
	require.NoError(t, err)
	labelsCpy := maps.Clone(expectedLabels)
	labelsCpy[eteleport.OktaACLReviewerRoleLabel] = "true"
	role.SetStaticLabels(labelsCpy)
	return role
}

func newRole(t *testing.T, name string, appLabels, groupLabels types.Labels) types.Role {
	t.Helper()

	var rules []types.Rule
	if len(groupLabels) > 0 {
		rules = append(rules, types.NewRule(types.KindUserGroup, services.RO()))
	}

	role, err := types.NewRoleWithVersion(name, types.V7, types.RoleSpecV6{
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
	role, err := ap.AccessService.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedRoles, role.GetSearchAsRoles(types.Allow))
}

func requireNotFound(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}
