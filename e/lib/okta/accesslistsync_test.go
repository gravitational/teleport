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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/modules"
)

var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
}

// accessListSyncTestContext contains test information for testing the access list synchronizer.
type accessListSyncTestContext struct {
	svc    *accessListSync
	clock  clockwork.FakeClock
	client *testOktaClient
	ap     *testAccessPoint
	apps   map[string]types.Application
	groups map[string]types.UserGroup
}

func (a *accessListSyncTestContext) addApp(app types.Application) {
	a.apps[app.GetName()] = app
}

func (a *accessListSyncTestContext) addGroup(group types.UserGroup) {
	a.groups[group.GetName()] = group
}

func (a *accessListSyncTestContext) advanceAndWaitForSync() {
	a.clock.BlockUntil(1)
	a.clock.Advance(a.svc.syncInterval)
	a.clock.BlockUntil(1)
}

func initAccessListSync(t *testing.T) *accessListSyncTestContext {
	t.Helper()

	clock := clockwork.NewFakeClock()
	client := newTestClient()
	ap := newTestAccessPoint(t, clockwork.NewFakeClock())
	stopCh := make(chan struct{}, 1)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: true,
			Cloud:                      true,
		},
	})

	alsCtx := &accessListSyncTestContext{
		clock:  clock,
		client: client,
		ap:     ap,
		apps:   map[string]types.Application{},
		groups: map[string]types.UserGroup{},
	}

	alSync, err := newAccessListSync(accessListSyncConfig{
		Clock:       clock,
		Client:      client,
		Owners:      []string{"owner1", "owner2"},
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
		c := initAccessListSync(t)
		c.advanceAndWaitForSync()

		require.Empty(t, c.svc.getImportAccessLists())
		require.Empty(t, c.svc.getNewImportAccessLists())
		require.Empty(t, c.svc.getImportAccessListMembers())
		require.Empty(t, c.svc.getNewImportAccessListMembers())
		require.Empty(t, c.svc.getImportRoles())
		require.Empty(t, c.svc.getNewImportRoles())
	})

	t.Run("Okta apps and groups but no assignments", func(t *testing.T) {
		c := initAccessListSync(t)
		c.advanceAndWaitForSync()

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		require.NoError(t, c.svc.importOktaNativeAssignmentsAsAccessLists(ctx))

		require.Empty(t, c.svc.getImportAccessLists())
		require.Empty(t, c.svc.getNewImportAccessLists())
		require.Empty(t, c.svc.getImportAccessListMembers())
		require.Empty(t, c.svc.getNewImportAccessListMembers())
		require.Empty(t, c.svc.getImportRoles())
		require.Empty(t, c.svc.getNewImportRoles())
	})

	t.Run("Okta apps have assignments, groups have no assignments", func(t *testing.T) {
		c := initAccessListSync(t)

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addUserID("user3", "3")
		c.client.addAppAssignments("app1", "1", "2", "3")
		c.client.addAppAssignments("app2", "1", "2", "3")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1": newAccessList(t, "app1", "app label", []string{"app1-reviewer"}, []string{"app1"}, owners),
			"app2": newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
		}, c.svc.getImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessLists(), c.svc.getNewImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1": newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app1/user2": newAccessListMember(t, "app1", "user2", c.clock.Now()),
			"app1/user3": newAccessListMember(t, "app1", "user3", c.clock.Now()),
			"app2/user1": newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2": newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"app2/user3": newAccessListMember(t, "app2", "user3", c.clock.Now()),
		}, c.svc.getImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessListMembers(), c.svc.getNewImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":          newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1"}}, nil),
			"app1-reviewer": newReviewerRole(t, "app1"),
			"app2":          newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2"}}, nil),
			"app2-reviewer": newReviewerRole(t, "app2"),
		}, c.svc.getImportRoles(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportRoles(), c.svc.getNewImportRoles(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)
	})

	t.Run("Okta apps have assignments, groups have assignments", func(t *testing.T) {
		c := initAccessListSync(t)

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addAppAssignments("app1", "1")
		c.client.addAppAssignments("app2", "1", "2")
		c.client.addGroupAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{"app1-reviewer"}, []string{"app1"}, owners),
			"app2":   newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
		}, c.svc.getImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessLists(), c.svc.getNewImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1":   newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app2/user1":   newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2":   newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"group1/user1": newAccessListMember(t, "group1", "user1", c.clock.Now()),
			"group1/user2": newAccessListMember(t, "group1", "user2", c.clock.Now()),
		}, c.svc.getImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessListMembers(), c.svc.getNewImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":            newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1"}}, nil),
			"app1-reviewer":   newReviewerRole(t, "app1"),
			"app2":            newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2"}}, nil),
			"app2-reviewer":   newReviewerRole(t, "app2"),
			"group1":          newRole(t, "group1", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
		}, c.svc.getImportRoles(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportRoles(), c.svc.getNewImportRoles(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)
	})

	t.Run("Okta apps have assignments, groups have assignments, apps and group filters added.", func(t *testing.T) {
		c := initAccessListSync(t)

		c.svc.groupFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}
		c.svc.appFilters = []*regexp.Regexp{
			regexp.MustCompile("^dev.*$"),
		}

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addAppAssignments("admin-app1", "1")
		c.client.addAppAssignments("dev-app2", "1", "2")
		c.client.addAppAssignments("dev-app3", "1", "2")
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
		}, c.svc.getImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessLists(), c.svc.getNewImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"dev-app2/user1":   newAccessListMember(t, "dev-app2", "user1", c.clock.Now()),
			"dev-app2/user2":   newAccessListMember(t, "dev-app2", "user2", c.clock.Now()),
			"dev-app3/user1":   newAccessListMember(t, "dev-app3", "user1", c.clock.Now()),
			"dev-app3/user2":   newAccessListMember(t, "dev-app3", "user2", c.clock.Now()),
			"dev-group2/user1": newAccessListMember(t, "dev-group2", "user1", c.clock.Now()),
			"dev-group2/user2": newAccessListMember(t, "dev-group2", "user2", c.clock.Now()),
			"dev-group3/user1": newAccessListMember(t, "dev-group3", "user1", c.clock.Now()),
			"dev-group3/user2": newAccessListMember(t, "dev-group3", "user2", c.clock.Now()),
		}, c.svc.getImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessListMembers(), c.svc.getNewImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"dev-app2":            newRole(t, "dev-app2", types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app2"}}, nil),
			"dev-app2-reviewer":   newReviewerRole(t, "dev-app2"),
			"dev-app3":            newRole(t, "dev-app3", types.Labels{eteleport.OktaAppIDLabel: []string{"dev-app3"}}, nil),
			"dev-app3-reviewer":   newReviewerRole(t, "dev-app3"),
			"dev-group2":          newRole(t, "dev-group2", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group2"}}),
			"dev-group2-reviewer": newReviewerRole(t, "dev-group2"),
			"dev-group3":          newRole(t, "dev-group3", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"dev-group3"}}),
			"dev-group3-reviewer": newReviewerRole(t, "dev-group3"),
		}, c.svc.getImportRoles(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportRoles(), c.svc.getNewImportRoles(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)
	})

	t.Run("previously existing apps and groups erased or updated, owners preserved", func(t *testing.T) {
		c := initAccessListSync(t)

		// Create a bunch of resources that the reconciler should clean up.
		_, err := c.ap.UpsertAccessList(ctx, newAccessList(t, "app1", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, []string{"some-other-owner"}))
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
			"app1":   newAccessList(t, "app1", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, []string{"some-other-owner"}),
			"app2":   newAccessList(t, "app2", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners),
			"group3": newAccessList(t, "group3", "blah", []string{"some-role-reviewer"}, []string{"some-role"}, owners),
		}, c.svc.getImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user-to-remove":   newAccessListMember(t, "app1", "user-to-remove", c.clock.Now()),
			"group3/user-to-remove": newAccessListMember(t, "group3", "user-to-remove", c.clock.Now()),
		}, c.svc.getImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app4": newRole(t, "app4", nil, nil),
		}, c.svc.getImportRoles(), cmpOpts...))

		c.client.addUserID("user1", "1")
		c.client.addUserID("user2", "2")
		c.client.addUserID("user-to-remove", "remove")
		c.client.addAppAssignments("app1", "1")
		c.client.addAppAssignments("app2", "1", "2")
		c.client.addGroupAssignments("group1", "1", "2")

		c.addApp(newAccessListSyncApp(t, "app1"))
		c.addApp(newAccessListSyncApp(t, "app2"))
		c.addGroup(newAccessListSyncGroup(t, "group1"))
		c.addGroup(newAccessListSyncGroup(t, "group2"))

		c.advanceAndWaitForSync()

		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessList{
			"app1":   newAccessList(t, "app1", "app label", []string{"app1-reviewer"}, []string{"app1"}, []string{"some-other-owner"}),
			"app2":   newAccessList(t, "app2", "app label", []string{"app2-reviewer"}, []string{"app2"}, owners),
			"group1": newAccessList(t, "group1", "group label", []string{"group1-reviewer"}, []string{"group1"}, owners),
		}, c.svc.getImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessLists(), c.svc.getNewImportAccessLists(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]*accesslist.AccessListMember{
			"app1/user1":   newAccessListMember(t, "app1", "user1", c.clock.Now()),
			"app2/user1":   newAccessListMember(t, "app2", "user1", c.clock.Now()),
			"app2/user2":   newAccessListMember(t, "app2", "user2", c.clock.Now()),
			"group1/user1": newAccessListMember(t, "group1", "user1", c.clock.Now()),
			"group1/user2": newAccessListMember(t, "group1", "user2", c.clock.Now()),
		}, c.svc.getImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportAccessListMembers(), c.svc.getNewImportAccessListMembers(), cmpOpts...))
		require.Empty(t, cmp.Diff(map[string]types.Role{
			"app1":            newRole(t, "app1", types.Labels{eteleport.OktaAppIDLabel: []string{"app1"}}, nil),
			"app1-reviewer":   newReviewerRole(t, "app1"),
			"app2":            newRole(t, "app2", types.Labels{eteleport.OktaAppIDLabel: []string{"app2"}}, nil),
			"app2-reviewer":   newReviewerRole(t, "app2"),
			"group1":          newRole(t, "group1", nil, types.Labels{eteleport.OktaGroupIDLabel: []string{"group1"}}),
			"group1-reviewer": newReviewerRole(t, "group1"),
		}, c.svc.getImportRoles(), cmpOpts...))
		require.Empty(t, cmp.Diff(c.svc.getImportRoles(), c.svc.getNewImportRoles(), cmpOpts...))

		verifyServiceMatchesBackend(t, c.ap, c.svc)
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
		eteleport.OktaAppIDLabel:      name,
	})
}

func newAccessListSyncAppLabelAppName(t *testing.T, name string) types.Application {
	t.Helper()

	return newAccessListSyncAppWithLabels(t, name, map[string]string{
		types.OriginLabel:             types.OriginOkta,
		types.OktaAppNameLabel:        name,
		types.OktaAppDescriptionLabel: "applink-name1",
		eteleport.OktaOrgURLLabel:     testOrgURL,
		eteleport.OktaAppIDLabel:      name,
	})
}

func newAccessListSyncGroupWithLabels(t *testing.T, name string, labels map[string]string) types.UserGroup {
	t.Helper()

	group, err := types.NewUserGroup(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	return group
}

func newAccessListSyncGroup(t *testing.T, name string) types.UserGroup {
	t.Helper()

	return newAccessListSyncGroupWithLabels(t, name, map[string]string{
		types.OriginLabel:               types.OriginOkta,
		types.OktaGroupNameLabel:        "group label",
		types.OktaGroupDescriptionLabel: "group description",
		eteleport.OktaOrgURLLabel:       testOrgURL,
		eteleport.OktaGroupIDLabel:      name,
	})
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
		AddedBy:    "okta-importer",
	})
	require.NoError(t, err)

	return member
}

func newReviewerRole(t *testing.T, grantRoleName string) types.Role {
	t.Helper()

	role, err := types.NewRole(grantRoleName+reviewerSuffix, types.RoleSpecV6{
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

	role, err := types.NewRole(name, types.RoleSpecV6{
		Allow: types.RoleConditions{
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

	accessLists := svc.getImportAccessLists()
	accessListMembers := svc.getImportAccessListMembers()
	roles := svc.getImportRoles()

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
