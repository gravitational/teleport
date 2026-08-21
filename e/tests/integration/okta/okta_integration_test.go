package okta

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func oktaUserLogin(u *okta.User) string {
	if u == nil {
		panic("okta user is nil")
	}
	if u.Profile == nil {
		panic("okta user profile is nil")
	}
	val, ok := (*u.Profile)["login"]
	if !ok {
		panic("login field not found")
	}
	var login string
	login, ok = val.(string)
	if !ok {
		panic("login field is not a string")
	}
	return login
}

// TestBasicAssignmentFlow tests the basic assignment flow.
// That tests the basic assignment flow for the apps groups imported by Okta integration
// making sure that removing/adding members to the access list will trigger the assignment/unassignment
// and the proper Okta API calls are made.
func TestBasicAssignmentFlow(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(2),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[0].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[1].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[4].Id)

	defaultOwner := fakeOkta.provisionedUsers[5]
	ownerLogin := oktaUserLogin(defaultOwner)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	tclCmd := sut.GetTCTL(t)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)

	beforeInstall := time.Now()

	err := tclCmd.Run(t.Context(),
		`plugins`, `install`, `okta`,
		`--org`, fakeOkta.URL(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		`--api-token="secret-okta-api-token`,
		"--owner="+ownerLogin)
	require.NoError(t, err)

	// TODO(smallinsky) Align timer when https://github.com/gravitational/teleport.e/issues/6558 issue is fixed
	// to test the flow with overlapping Okta sync.
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Minute,
		timeBetweenAssignmentProcessLoops: time.Minute,
	})

	waitForOktaSync(t, sut, withTimeout(time.Minute), withStep(time.Millisecond*100), withTimePoint(beforeInstall))
	// During init we have added 3 users to the group, so we should have 3 initial assignments created for those users.
	numOfInitialUserOktaAssignments := 3
	waitForPerUserOktaAssignments(t, assignmentWatcher, numOfInitialUserOktaAssignments)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaUserLogin(fakeOkta.provisionedUsers[0]))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		wantMembers := []*accesslist.AccessListMember{
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaUserLogin(fakeOkta.provisionedUsers[0])}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaUserLogin(fakeOkta.provisionedUsers[1])}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaUserLogin(fakeOkta.provisionedUsers[4])}}},
		}
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tclCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, fakeOkta.provisionedGroups[0].Id,
		}, &accessListMembers)
		assertResourcesByName(t, wantMembers, accessListMembers)
	}, time.Minute, time.Millisecond*100)
	err = tclCmd.Run(t.Context(),
		`acl`, `users`, `rm`, fakeOkta.provisionedGroups[0].Id, oktaUserLogin(fakeOkta.provisionedUsers[0]))
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.False(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[0].Id))
	}, time.Minute, time.Millisecond*250, "User %s is still assigned to group %s", fakeOkta.provisionedUsers[0].Id, fakeOkta.provisionedGroups[0].Id)

	err = tclCmd.Run(t.Context(),
		`acl`, `users`, `add`, fakeOkta.provisionedGroups[0].Id, oktaUserLogin(fakeOkta.provisionedUsers[2]))
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[2].Id))
	}, time.Minute, time.Millisecond*250, "User %s was never assigned to group %s", fakeOkta.provisionedUsers[2].Id, fakeOkta.provisionedGroups[0].Id)
}

// TestNestedAclAssignment tests the assignment and sync of nested access lists.
// It ensures that members from nested access lists are flattened and added to the root access list in Okta,
// and that on reconciliation during sync, the nested lists are not flattened on the Teleport side.
func TestNestedAclAssignment(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(2),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[1].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[2].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[3].Id)

	defaultOwner := fakeOkta.provisionedUsers[0]
	ownerLogin := oktaUserLogin(defaultOwner)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	authServer := sut.Teleport.Process.GetAuthServer()
	tctlCmd := sut.GetTCTL(t)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)

	beforeInstall := time.Now()

	err := tctlCmd.Run(t.Context(),
		`plugins`, `install`, `okta`,
		`--org`, fakeOkta.URL(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		`--api-token="secret-okta-api-token"`,
		"--owner="+ownerLogin,
	)
	require.NoError(t, err)

	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Second,
		timeBetweenAssignmentProcessLoops: time.Second,
	})
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(beforeInstall))
	// During init we have added 3 users to the group, so we should have 3 initial assignments created for those users.
	numOfInitialUserOktaAssignments := 3
	waitForPerUserOktaAssignments(t, assignmentWatcher, numOfInitialUserOktaAssignments)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), ownerLogin)

	oktaSyncedList, err := authServer.GetAccessList(ctx, fakeOkta.provisionedGroups[0].Id)
	require.NoError(t, err)
	teleportList0 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-0", []string{ownerLogin}, accesslist.Grants{Roles: []string{"access"}}, []string{oktaUserLogin(fakeOkta.provisionedUsers[4])})
	teleportList1 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-1", []string{ownerLogin}, accesslist.Grants{Roles: []string{"editor"}}, []string{oktaUserLogin(fakeOkta.provisionedUsers[5])})
	teleportList2 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-2", []string{ownerLogin}, accesslist.Grants{Roles: []string{"reviewer"}}, []string{oktaUserLogin(fakeOkta.provisionedUsers[6])})

	// teleport list 1 and 2 are nested within teleport list 0
	nestedTeleportList1, err := authServer.AccessListsInternal.UpsertAccessListMember(ctx, mustCreateMember(t, teleportList0.GetName(), teleportList1.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)
	nestedTeleportList2, err := authServer.AccessListsInternal.UpsertAccessListMember(ctx, mustCreateMember(t, teleportList0.GetName(), teleportList2.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)

	// teleport list 0 is nested within okta-created list
	nestedTeleportList0, err := authServer.AccessListsInternal.UpsertAccessListMember(ctx, mustCreateMember(t, oktaSyncedList.GetName(), teleportList0.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100))

	// members from nested lists should be flattened + added to root in okta
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[4].Id))
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[5].Id))
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[6].Id))
	}, time.Second*30, time.Millisecond*250)

	// ensure membership on Teleport side isn't flattened after sync w/ okta
	assertAccessListMembers(t, ctx, sut, fakeOkta.provisionedGroups[0].Id, []string{
		// from root list
		oktaUserLogin(fakeOkta.provisionedUsers[1]),
		oktaUserLogin(fakeOkta.provisionedUsers[2]),
		oktaUserLogin(fakeOkta.provisionedUsers[3]),
		// nested list members
		nestedTeleportList0.GetName(),
	})

	// nested lists should contain their members
	assertAccessListMembers(t, ctx, sut, nestedTeleportList0.GetName(), []string{
		oktaUserLogin(fakeOkta.provisionedUsers[4]),
		nestedTeleportList1.GetName(),
		nestedTeleportList2.GetName(),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList1.GetName(), []string{
		oktaUserLogin(fakeOkta.provisionedUsers[5]),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList2.GetName(), []string{
		oktaUserLogin(fakeOkta.provisionedUsers[6]),
	})
}

// TestAccessRequest tests the access request flow. That tests the access request flow for the apps groups imported
// by Okta integration where the okta-requester role is used and the access list owner is able to review
// the access request and approve it.
func TestAccessRequest(t *testing.T) {
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(2),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.CreateApplicationGroupAssignment(fakeOkta.provisionedApps[0].Id, fakeOkta.provisionedGroups[0].Id)

	reviewer := fakeOkta.provisionedUsers[5]
	requester := fakeOkta.provisionedUsers[4]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
		common.WithModules(modulestest.EnterpriseModules()),
	)

	tctlCmd := sut.GetTCTL(t)

	beforeInstall := time.Now()

	err := tctlCmd.Run(t.Context(),
		`plugins`, `install`, `okta`,
		`--org`, fakeOkta.URL(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		`--api-token="secret-okta-api-token"`,
		"--owner="+oktaUserLogin(reviewer),
	)
	require.NoError(t, err)

	// Set time between imports to 1s
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Second,
		timeBetweenAssignmentProcessLoops: time.Second,
	})

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(beforeInstall))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var wantMembers []*accesslist.AccessListMember
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tctlCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, fakeOkta.provisionedGroups[0].Id,
		}, &accessListMembers)
		assertResourcesByName(t, wantMembers, accessListMembers)
	}, time.Second*10, time.Millisecond*100)

	tshRequester := &tshCommand{
		DataDir:   sut.DataDir,
		Listener:  sut.AuthListenerAddr,
		tshHome:   t.TempDir(),
		clock:     clockwork.NewRealClock(),
		proxyAddr: sut.ProxyAddr,
	}
	t.Run("requester requests access to an app", func(t *testing.T) {
		userGroups := fakeOkta.ListUserGroups(requester.Id)
		groupNames := make([]string, 0, len(userGroups))
		for _, v := range userGroups {
			groupNames = append(groupNames, v.Id)
		}

		tshRequester.mustLogin(t, oktaUserLogin(requester), append(groupNames, "Everyone"))
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			var apps []*types.AppV3
			mustRunTSHAndGetResultAs(t, tshRequester, []string{
				"app", "ls",
				"--insecure",
				"--format=json",
			}, &apps)
			require.Empty(t, apps)
		}, time.Second*10, time.Millisecond*100)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			appServers, err := sut.Teleport.Process.GetAuthServer().GetApplicationServers(ctx, "default")
			require.NoError(t, err)
			var appID string
			for _, v := range appServers {
				if v.GetApp().GetDescription() == fakeOkta.provisionedApps[0].Label {
					appID = v.GetApp().GetName()
					break
				}
			}
			require.NotEmpty(t, appID)

			err = tshRequester.run(t, []string{
				`request`, `create`, `--resource`, `/local-site/app/` + appID, `--nowait`,
			})
			require.NoError(t, err)
		}, time.Second*10, time.Millisecond*100)
	})

	var requestID string
	t.Run("reviewer reviews the access request", func(t *testing.T) {
		tshReviewer := &tshCommand{
			DataDir:   sut.DataDir,
			Listener:  sut.AuthListenerAddr,
			tshHome:   t.TempDir(),
			clock:     clockwork.NewRealClock(),
			proxyAddr: sut.ProxyAddr,
		}
		userGroups := fakeOkta.ListUserGroups(requester.Id)
		groupNames := make([]string, 0, len(userGroups))
		for _, v := range userGroups {
			groupNames = append(groupNames, v.Id)
		}

		tshReviewer.mustLogin(t, oktaUserLogin(reviewer), append(groupNames, "Everyone"))
		var accReqs []*types.AccessRequestV3
		mustRunTSHAndGetResultAs(t, tshReviewer, []string{
			`request`, `ls`, `--format=json`,
		}, &accReqs)
		require.Len(t, accReqs, 1)

		err := tshReviewer.run(t, []string{
			`request`, `review`, `--approve=` + accReqs[0].GetName(),
		})
		require.NoError(t, err)
		requestID = accReqs[0].GetName()
	})

	t.Run("requester is able to access the app", func(t *testing.T) {
		err := tshRequester.run(t, []string{
			`login`, `--request-id=` + requestID,
			`--insecure`,
		})
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			var apps []*types.AppV3
			mustRunTSHAndGetResultAs(t, tshRequester, []string{
				"app", "ls",
				"--insecure",
				"--format=json",
			}, &apps)
			want := []*types.AppV3{
				{Metadata: types.Metadata{Description: fakeOkta.provisionedApps[0].Label}},
			}
			assertResourcesByDesc(t, want, apps)
		}, time.Second*10, time.Millisecond*100)
	})
}

func assertAccessListMembers(t *testing.T, ctx context.Context, sut *common.SUT, accessListName string, want []string) {
	got, _, err := sut.Teleport.Process.GetAuthServer().ListAccessListMembers(ctx, accessListName, 0, "")
	require.NoError(t, err)

	var wantMembers []*accesslist.AccessListMember
	for _, name := range want {
		wantMembers = append(wantMembers, &accesslist.AccessListMember{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: name}}})
	}

	assertResourcesByName(t, wantMembers, got)
}

func createAccessListWithMembers(t *testing.T, ctx context.Context, sut *common.SUT, name string, owners []string, grants accesslist.Grants, members []string) *accesslist.AccessList {
	var accessListOwners []accesslist.Owner
	for _, owner := range owners {
		accessListOwners = append(accessListOwners, accesslist.Owner{
			Name:             owner,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindUser,
		})
	}
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title:  name,
		Owners: accessListOwners,
		Grants: grants,
		Audit:  accesslist.Audit{NextAuditDate: sut.Clock.Now()},
	})
	require.NoError(t, err)

	var accessListMembers []*accesslist.AccessListMember
	for _, member := range members {
		accessListMembers = append(accessListMembers, mustCreateMember(t, accessList.GetName(), member, accesslist.MembershipKindUser))
	}

	_, _, err = sut.Teleport.Process.GetAuthServer().AccessListsInternal.UpsertAccessListWithMembers(ctx, accessList, accessListMembers)
	require.NoError(t, err)

	return accessList
}

func mustCreateMember(t *testing.T, aclName, memberName string, memberType string) *accesslist.AccessListMember {
	clock := clockwork.NewRealClock()
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     aclName,
			Name:           memberName,
			Joined:         clock.Now(),
			AddedBy:        "added by",
			Expires:        clock.Now().Add(time.Hour * 24).UTC(),
			MembershipKind: memberType,
		},
	)
	require.NoError(t, err)
	return member
}

func TestOktaAccessRequestFlow(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.CreateApplicationGroupAssignment(fakeOkta.provisionedApps[0].Id, fakeOkta.provisionedGroups[0].Id)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	reviewer := fakeOkta.provisionedUsers[5]
	requester := fakeOkta.provisionedUsers[4]

	requesterLogin := oktaUserLogin(requester)
	reviewerLogin := oktaUserLogin(reviewer)

	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedApps[0].Id, fakeOkta.provisionedUsers[0].Id))

	for _, user := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))
	}

	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	start := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:            apiCredentials,
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        true,
		EnableAccessListSync:      true,
		EnableBidirectionalSync:   true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{reviewerLogin},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	updateOktaDelays(t, sut, delays{
		// TODO(smallinsky) Align timer when https://github.com/gravitational/teleport.e/issues/6558 issue is fixed
		// to test the flow with overlapping Okta sync.
		timeBetweenImports:                5 * time.Minute,
		timeBetweenAssignmentProcessLoops: 5 * time.Minute,
	})
	require.NoError(t, err)
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	auth := sut.Teleport.Process.GetAuthServer()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Verify reviewer user has okta-requester + 2 ACL reviewer roles
		s, err := auth.GetUserLoginState(ctx, reviewerLogin)
		require.NoError(t, err)
		require.Len(t, s.GetRoles(), 3)

		// Verify okta-requester role has necessary search_as_roles. It is
		// updated by Okta assignment processor async so if we don't ensure
		// this, we may get a race where the test tries to submit an access
		// request before okta-requester role was updated.
		oktaRequester, err := auth.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
		require.NoError(t, err)
		sar := oktaRequester.GetSearchAsRoles(types.Allow)
		require.NotEmpty(t, sar)
	}, time.Minute, time.Millisecond*100)

	t.Run("app access request", func(t *testing.T) {
		var app types.AppServer
		apps, err := auth.GetApplicationServers(t.Context(), "default")
		require.NoError(t, err)
		for _, v := range apps {
			if v.GetMetadata().Description != fakeOkta.provisionedSAMLApp.Name {
				app = v
				break
			}
		}

		appID, ok := app.GetLabel(eteleport.OktaAppIDLabel)
		require.True(t, ok)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			require.False(t, fakeOkta.IsUserAssignedToApplication(appID, requester.Id))
		}, time.Minute, time.Millisecond*250, "User %s was assigned to app %s", requester.Id, appID)

		assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requesterLogin)

		t.Run("delete app access request", func(t *testing.T) {
			accessRequestApp := createAccessRequest(t, sut, app.GetName(), types.KindApp, requesterLogin)
			approveAccessRequest(t, sut, accessRequestApp.GetName(), reviewerLogin)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.True(t, fakeOkta.IsUserAssignedToApplication(appID, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s was never assigned to app %s", requester.Id, appID)

			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requesterLogin)

			// TODO(smallinsky): remove dependency on locking access request.
			deleteAccessRequest(t, sut, accessRequestApp.GetName())

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.False(t, fakeOkta.IsUserAssignedToApplication(appID, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s is still assigned to app %s", requester.Id, appID)

			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requesterLogin)
		})

		t.Run("member was added to acl before access request was deleted", func(t *testing.T) {
			accessRequestApp := createAccessRequest(t, sut, app.GetName(), types.KindApp, requesterLogin)
			approveAccessRequest(t, sut, accessRequestApp.GetName(), reviewerLogin)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.True(t, fakeOkta.IsUserAssignedToApplication(appID, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s was never assigned to app %s", requester.Id, appID)

			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), reviewerLogin)

			mustAddAccessListMember(t, sut, app.GetName(), reviewerLogin)

			err = sut.Teleport.Process.GetAuthServer().DeleteAccessRequest(t.Context(), accessRequestApp.GetName())
			require.NoError(t, err)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.True(t, fakeOkta.IsUserAssignedToApplication(appID, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s was never assigned to app %s", requester.Id, appID)

			assertUserIsAccessListMember(ctx, t, sut, app.GetName(), reviewerLogin)
		})
	})

	t.Run("group access request", func(t *testing.T) {
		userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
		require.NoError(t, err)
		require.NotEmpty(t, userGroups)
		group := selectUserGroupByName(userGroups, fakeOkta.provisionedGroups[0].Id)
		require.NotNil(t, group)
		groupID := group.GetName()

		t.Run("delete group access request", func(t *testing.T) {
			accessRequest := createAccessRequest(t, sut, groupID, types.KindUserGroup, requesterLogin)

			assertUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
			approveAccessRequest(t, sut, accessRequest.GetName(), reviewerLogin)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s was never assigned to group %s", requester.Id, fakeOkta.provisionedGroups[0].Id)

			// TODO(smallinsky): remove dependency on locking access request.
			deleteAccessRequest(t, sut, accessRequest.GetName())

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.False(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))
			}, time.Minute, time.Millisecond*250, "User %s is still assigned to group %s", requester.Id, fakeOkta.provisionedGroups[0].Id)

			assertUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
		})
	})
}

func selectUserGroupByName(groups []types.UserGroup, name string) types.UserGroup {
	for _, group := range groups {
		if group.GetName() == name {
			return group
		}
	}
	return nil
}

func deleteAccessRequest(t *testing.T, sut *common.SUT, accessRequestName string) {
	// TODO(smallinsky): remove after https://github.com/gravitational/teleport.e/issues/8118 is addressed.
	//
	// In the Okta access request flow, revocation on deletion relies on handling an op.Delete event.
	// If the service isn’t fully started, the plugin restarts, or leader election is still in progress,
	// the handler may miss that delete event.
	//
	// In that case, Okta assignments may not be revoked immediately on access request deletion, and
	// will instead be revoked later when the access request expires (AccessRequest expiration is propagate as
	// Okta assignment cleanup time)
	lockAccessRequest(t, sut, accessRequestName)

	err := sut.Teleport.Process.GetAuthServer().DeleteAccessRequest(t.Context(), accessRequestName)
	require.NoError(t, err)
}

func lockAccessRequest(t *testing.T, sut *common.SUT, accessRequestName string) {
	lock, err := types.NewLock(accessRequestName, types.LockSpecV2{Target: types.LockTarget{AccessRequest: accessRequestName}})
	require.NoError(t, err)
	err = sut.Teleport.Process.GetAuthServer().Services.UpsertLock(t.Context(), lock)
	require.NoError(t, err)
}

func mustAddAccessListMember(t *testing.T, sut *common.SUT, aclName, memberName string) {
	acl, err := sut.Teleport.Process.GetAuthServer().GetAccessList(t.Context(), aclName)
	require.NoError(t, err)
	members, _, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.ListAccessListMembers(t.Context(), aclName, 0, "")
	require.NoError(t, err)
	members = append(members, mustCreateMember(t, aclName, memberName, accesslist.MembershipKindUser))
	_, _, err = sut.Teleport.Process.GetAuthServer().AccessListsInternal.UpsertAccessListWithMembers(t.Context(), acl, members)
	require.NoError(t, err)
}

// assertUserIsNotAccessListMember is deprecated.
//
// Deprecated: use requireUserIsNotAccessListMember and/or waitForResourceDeletion
func assertUserIsNotAccessListMember(ctx context.Context, t require.TestingT, sut *common.SUT, acl, user string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.GetAccessListMember(ctx, acl, user)
		require.True(t, trace.IsNotFound(err))
	}, time.Minute, 50*time.Millisecond, "User %s should not be a member of access list %s", user, acl)
}

func requireUserIsNotAccessListMember(ctx context.Context, t *testing.T, sut *common.SUT, acl, user string) {
	t.Helper()
	_, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.GetAccessListMember(ctx, acl, user)
	require.True(t, trace.IsNotFound(err))
}

func assertUserIsAccessListMember(ctx context.Context, t require.TestingT, sut *common.SUT, acl, user string) *accesslist.AccessListMember {
	var member *accesslist.AccessListMember
	var err error
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		member, err = sut.Teleport.Process.GetAuthServer().AccessListsInternal.GetAccessListMember(ctx, acl, user)
		require.NoError(t, err)
	}, time.Minute, 500*time.Millisecond)
	return member
}

// TestOktaAccessRequestWithSCIMOktaSync tests access requests when SCIM Okta sync is enabled.
// When an access request is approved, the user should be added to the corresponding Okta group.
// However, the user should not be synced back as an access list member via SCIM Group update,
// as this would escalate // their privileges to long-term access.
// Even after the access request expires, the user would
// still remain a member of the access list.
func TestOktaAccessRequestWithSCIMOktaSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const groupCount = 1

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(groupCount),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.CreateApplicationGroupAssignment(fakeOkta.provisionedApps[0].Id, fakeOkta.provisionedGroups[0].Id)

	reviewer := fakeOkta.provisionedUsers[5]
	requester := fakeOkta.provisionedUsers[4]
	reviewerLogin := oktaUserLogin(reviewer)
	requesterLogin := oktaUserLogin(requester)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	userLoginStateWatcher := sut.NewResourceWatcher(t, types.KindUserLoginState)
	roleWatcher := sut.NewResourceWatcher(t, types.KindRole)
	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta, withAccessListSettings(oktav1.AccessListSettings_builder{
		GroupFilters: []string{"group-*"},
		AppFilters:   []string{"app-*"},
		DefaultOwner: []string{reviewerLogin},
	}.Build()), withEnableFullSync())
	scimClient := createSCIMClient(t, sut, scimToken)

	// If the reviewer's UserLoginState has okta-requester and roles that means, both the User
	// and Access Lists are synced.
	waitForResource(t, userLoginStateWatcher, func(uls *userloginstate.UserLoginState) bool {
		return uls.GetName() == reviewerLogin && len(uls.GetRoles()) == 1+groupCount // okta-requester + ACL reviewer role for each group (default owner)
	})

	auth := sut.Teleport.Process.GetAuthServer()
	userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, userGroups)
	group := selectUserGroupByName(userGroups, fakeOkta.provisionedGroups[0].Id)
	require.NotNil(t, group)
	groupID := group.GetName()

	// Wait for the "okta-requester" role to be updated with search_as_roles, otherwise we can
	// hit an error like: Resource Access Requests require usable "search_as_roles", none found
	// for user "user-4@example.com"
	waitForResource(t, roleWatcher, func(r types.Role) bool {
		return r.GetName() == teleport.SystemOktaRequesterRoleName && len(r.GetSearchAsRoles(types.Allow)) == groupCount
	})

	accessRequest := createAccessRequest(t, sut, groupID, types.KindUserGroup, requesterLogin)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	requireUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
	approveAccessRequest(t, sut, accessRequest.GetName(), reviewerLogin)

	g, err := scimClient.GetGroup(ctx, fakeOkta.provisionedGroups[0].Id)
	require.NoError(t, err)
	g.Members = append(g.Members, &scimsdk.GroupMember{
		ExternalID: requesterLogin,
	})
	_, err = scimClient.UpdateGroup(t.Context(), g)
	require.NoError(t, err)

	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetCleanupTime().Equal(accessRequest.GetAccessExpiry()) &&
			a.GetStatus() == constants.OktaAssignmentStatusSuccessful &&
			assignmentHasTarget(a, constants.OktaAssignmentTargetGroup, fakeOkta.provisionedGroups[0].Id)
	})
	require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))

	requireUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)

	// TODO(kopiczko): Replace with a regular auth.DeleteAccessRequest call. currently there is
	// a race between AccessRequestReconciler.OnLogin and .onDelete. Details here
	// https://github.com/gravitational/teleport.e/issues/8118#issuecomment-5367514561. So the
	// lock has to be created to make sure the corresponding okta_assignment is cleaned up.
	deleteAccessRequest(t, sut, accessRequest.GetName())

	// https://github.com/gravitational/teleport.e/issues/8654
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.IsFinalized() &&
			assignmentHasTarget(a, constants.OktaAssignmentTargetGroup, fakeOkta.provisionedGroups[0].Id)
	})
	require.False(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))

	requireUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
}

func TestOktaAssignmentRaceCheck(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(0),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	for _, user := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))
	}

	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	start := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})
	require.NoError(t, err)
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	memberID := fakeOkta.provisionedUsers[4].Id
	memberLogin := oktaUserLogin(fakeOkta.provisionedUsers[4])
	groupID := fakeOkta.provisionedGroups[0].Id

	const iterCount = 10
	for i := range iterCount {
		fakeOkta.AddUserToGroup(groupID, memberID)
		m := assertUserIsAccessListMember(ctx, t, sut, groupID, memberLogin)
		require.Equal(t, "okta-service", m.Spec.AddedBy)

		fakeOkta.RemoveUserFromGroup(groupID, memberID)
		assertUserIsNotAccessListMember(ctx, t, sut, groupID, memberLogin)

		// TODO(smallinsky): Remove this check when https://github.com/gravitational/teleport.e/issues/6558 is fixed.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assignments, _, err := sut.Teleport.Process.GetAuthServer().Okta.ListOktaAssignments(ctx, 0, "")
			require.NoError(t, err)
			require.Empty(t, assignments)
		}, time.Minute, time.Millisecond*500, "iteration = %d", i+1)
	}
}

// TestCleanupAssignmentFilter tests that the cleanupAssignmentFilter prevents the access list
// sync from re-adding a member whose Okta assignment is pending cleanup.
//
// Scenario:
//  1. User is a member of an Okta group (synced as access list member)
//  2. User is removed from the access list in Teleport (cleanup_time is set on OktaAssignment)
//  3. The assignment processor tries to remove the user from the Okta group but fails
//     (simulated via fakeOktaServer returning HTTP 429 on DELETE)
//  4. Access list sync runs and sees the user still in the Okta group (stale data)
//  5. The cleanupAssignmentFilter prevents the sync from re-adding the user
//  6. Once the Okta API error clears, the assignment processor completes the cleanup
func TestCleanupAssignmentFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withUserCount(1),
		withAppCount(0),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	for _, user := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))
	}
	memberID := fakeOkta.provisionedUsers[0].Id
	memberLogin := oktaUserLogin(fakeOkta.provisionedUsers[0])
	groupID := fakeOkta.provisionedGroups[0].Id

	// Add user to the Okta group so the sync picks them up as an access list member.
	fakeOkta.AddUserToGroup(groupID, memberID)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	createAndWaitForOktaIntegration(t, sut, fakeOkta, withAccessListSettings(oktav1.AccessListSettings_builder{
		GroupFilters: []string{"group-*"},
		AppFilters:   []string{"app-*"},
		DefaultOwner: []string{"alice-admin"},
	}.Build()), withEnableFullSync())
	// Wait for the per-user OktaAssignment to be created before proceeding.
	//
	// If we delete the access list member before any OktaAssignment exists,
	// the staleOktaMemberFilter (which prevents the sync from re-adding
	// deleted members) has no assignment to compare against, so subsequent
	// sync cycles re-add the member.
	//
	// This is a low-risk production race: it only triggers when a member is
	// removed from an access list within the brief window before the first
	// OktaAssignment is created for a user. The impact is negligible
	// because if applies only to first sync cycle.
	//
	// TODO(smallinsky): After addressing https://github.com/gravitational/teleport.e/issues/8654
	// the access list sync and assignments creation during first sync can be synchronized.
	waitForPerUserOktaAssignments(t, assignmentWatcher, 1)

	auth := sut.Teleport.Process.GetAuthServer()

	// Wait for the user to appear as access list member.
	assertUserIsAccessListMember(ctx, t, sut, groupID, memberLogin)

	// Block the assignment processor from removing the user from the Okta group
	// by making the DELETE endpoint return 429 (Too Many Requests).
	// This simulates Okta rate limiting the removal request.
	fakeOkta.SetRemoveUserFromGroupOverwrite(func(_, _ string) error {
		return trace.LimitExceeded("rate limited")
	})

	// Effectively disable backoff for failed targets.
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Second,
		timeBetweenAssignmentProcessLoops: time.Second,
		targetProcessingBackoffStep:       time.Second,
		targetProcessingBackoffMax:        time.Second,
	})

	// Remove user from the access list in Teleport. This triggers the user monitor
	// to set cleanup_time on the OktaAssignment.
	err := auth.AccessListsInternal.DeleteAccessListMember(ctx, groupID, memberLogin)
	require.NoError(t, err)

	// Wait for at least one more access list sync cycle. The sync will see the user
	// still in the Okta group, but the cleanupAssignmentFilter must prevent re-addition.
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	// Check if a user is still assigned to the Okta group,
	// which means the sync did not remove them due to the pending cleanup
	// because of the API error.
	require.True(t, fakeOkta.UserAssignedGroup(groupID, memberID))

	// Assert the user was NOT re-added as an access list member.
	assertUserIsNotAccessListMember(ctx, t, sut, groupID, memberLogin)

	// Clear the overwrite so the assignment processor can complete the cleanup.
	fakeOkta.SetRemoveUserFromGroupOverwrite(nil)

	// Verify the user is eventually removed from the Okta group.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.False(t, fakeOkta.UserAssignedGroup(groupID, memberID))
	}, time.Minute, time.Millisecond*250)
}

// TestOktaAssignmentFailedCleanupProcessing verifies there that the assignmentProcessor creates
// okta_assignment related audit events only when targets are truly processed or cleaned up. For
// that to happen the timer-based loop is disabled by setting timeBetweenAssignmentProcessLoops to
// 1h (otherwise re-processing would generate more audit events). This by extension verifies there
// are no races or unnecessary updates for okta_assignment resources during their lifecycle. One
// example of such unnecessary update that was fixed was updating the cleanup time to the current
// time by UserAssignmentCreator for the assignments already scheduled for cleanup.
func TestOktaAssignmentFailedCleanupProcessing(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const usersLen = 10

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(usersLen),
		withAppCount(0),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	for _, user := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))
	}

	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	start := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	updateOktaDelays(t, sut, delays{
		// We don't want timer-based processing to intertwine as it may cause extra cleanup
		// audit events if re-processing kicks-in during EventuallyWithT.
		timeBetweenAssignmentProcessLoops: 1 * time.Hour,
	})
	require.NoError(t, err)
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	users := mustGetOktaUsers(t, sut)
	require.Len(t, users, usersLen)

	// Make sure we have 1 lists synced for each group.
	accessLists := mustGetAccessLists(t, sut)
	require.Len(t, accessLists, 1)
	accessList := accessLists[0]

	// Make sure there are no Okta assignments related in the backend events yet.
	assignmentEvents := mustGetEventsFrom(t, sut, start, events.OktaAssignmentProcessEvent, events.OktaAssignmentCleanupEvent)
	require.Empty(t, assignmentEvents)

	start = time.Now()
	mustUpsertAccessListMember(t, sut, accessList, users[0])
	mustUpsertAccessListMember(t, sut, accessList, users[1])

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		processEvents := mustGetEventsFrom(t, sut, start, events.OktaAssignmentProcessEvent)
		// TODO(kopiczko): the check here should be `require.Len(t, processEvents, 2, "len(processEvents) = %d", len(processEvents))` when https://github.com/gravitational/teleport.e/issues/8654 is addressed
		require.GreaterOrEqual(t, len(processEvents), 2, "len(processEvents) = %d", len(processEvents))
		require.LessOrEqual(t, len(processEvents), 4, "len(processEvents) = %d", len(processEvents))

		cleanupEvents := mustGetEventsFrom(t, sut, start, events.OktaAssignmentCleanupEvent)
		// TODO(kopiczko): the check here should be `require.Empty(t, cleanupEvents, "len(cleanupEvents) = %d", len(cleanupEvents))` when https://github.com/gravitational/teleport.e/issues/8654 is addressed
		require.GreaterOrEqual(t, len(cleanupEvents), 0, "len(cleanupEvents) = %d", len(cleanupEvents))
		require.LessOrEqual(t, len(cleanupEvents), 2, "len(cleanupEvents) = %d", len(cleanupEvents))
	}, 1*time.Minute, 500*time.Millisecond)

	fakeOkta.SetRemoveUserFromGroupOverwrite(func(_, _ string) error {
		return trace.LimitExceeded("assignment cleanup audit events test: failing to remove the user from group on the Okta side")
	})

	mustDeleteAccessListMember(t, sut, accessList, users[0])
	mustDeleteAccessListMember(t, sut, accessList, users[1])
	for _, u := range users[2:] {
		mustUpsertAccessListMember(t, sut, accessList, u)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		processEvents := mustGetEventsFrom(t, sut, start, events.OktaAssignmentProcessEvent)
		// TODO(kopiczko): the check here should be `require.Len(t, processEvents, len(users), "len(processEvents) = %d", len(processEvents))` when https://github.com/gravitational/teleport.e/issues/8654 is addressed
		require.GreaterOrEqual(t, len(processEvents), len(users), "len(processEvents) = %d", len(processEvents))
		require.LessOrEqual(t, len(processEvents), len(users)*2, "len(processEvents) = %d", len(processEvents))

		cleanupEvents := mustGetEventsFrom(t, sut, start, events.OktaAssignmentCleanupEvent)
		// TODO(kopiczko): the check here should be `require.Len(t, cleanupEvents, 2, "len(cleanupEvents) = %d", len(cleanupEvents))` when https://github.com/gravitational/teleport.e/issues/8654 is addressed
		require.GreaterOrEqual(t, len(cleanupEvents), 2, "len(cleanupEvents) = %d", len(cleanupEvents))
		require.LessOrEqual(t, len(cleanupEvents), 4, "len(cleanupEvents) = %d", len(cleanupEvents))
	}, 1*time.Minute, 500*time.Millisecond)
}

// TestOktaAssignmentTargetReprocessingBackoff verifies that target reprocessing backoff is applied for failed targets.
func TestOktaAssignmentTargetReprocessingBackoff(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(withUserCount(2), withGroupCount(1))
	t.Cleanup(fakeOkta.Stop)

	owner := fakeOkta.provisionedUsers[0]
	ownerLogin := oktaUserLogin(owner)

	user := fakeOkta.provisionedUsers[1]
	userLogin := oktaUserLogin(user)

	group := fakeOkta.provisionedGroups[0].Id
	fakeOkta.AddUserToGroup(group, owner.Id)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)

	tctlCmd := sut.GetTCTL(t)
	require.NoError(t, tctlCmd.Run(
		t.Context(),
		`plugins`, `install`, `okta`,
		`--org`, fakeOkta.URL(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		`--api-token="secret-okta-api-token`,
		"--owner="+ownerLogin,
	))

	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Hour,
		timeBetweenAssignmentProcessLoops: time.Second,
		targetProcessingBackoffStep:       5 * time.Second,
		targetProcessingBackoffMax:        5 * time.Second,
	})

	waitForOktaSync(t, sut, withTimeout(time.Minute), withStep(time.Millisecond*100), withTimePoint(time.Now()))
	waitForPerUserOktaAssignments(t, assignmentWatcher, 1)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), ownerLogin)

	fakeOkta.setAssignUserToGroupOverwrite(func(_, _ string) error {
		return trace.LimitExceeded("rate limited")
	})

	require.NoError(t, tctlCmd.Run(ctx, `acl`, `users`, `add`, group, userLogin))

	var initialAssignmentTransition time.Time
	var initialTargetProcessed time.Time

	// Initial assignment processing.
	require.EventuallyWithT(t, func(tc *assert.CollectT) {
		assignment := mustGetAssignmentForUser(tc, sut, userLogin)
		require.NotEqual(tc, constants.OktaAssignmentStatusProcessing, assignment.GetStatus())

		targets := assignment.GetTargets()
		require.Len(tc, targets, 1)

		status := targets[0].GetStatus()
		require.NotNil(tc, status)
		require.Equal(tc, int32(1), status.FailureCount)

		initialAssignmentTransition = assignment.GetLastTransition()
		initialTargetProcessed = status.LastProcessed
	}, time.Minute, 100*time.Millisecond)

	// Wait for the assignment to be reprocessed while the target isn't reprocessed,
	// i.e. target skipped reprocessing within backoff.
	require.EventuallyWithT(t, func(tc *assert.CollectT) {
		assignment := mustGetAssignmentForUser(tc, sut, userLogin)
		require.Equal(tc, constants.OktaAssignmentStatusFailed, assignment.GetStatus())

		targets := assignment.GetTargets()
		require.Len(tc, targets, 1)

		status := targets[0].GetStatus()
		require.NotNil(tc, status)

		// If the last transition time of the assignment is after the initial failed assignment transition time,
		// and the last processed time of the target is the same as the initial failed processed time,
		// then the assignment has been reprocessed but skipped reprocessing the target, indicating it's in backoff.
		require.True(tc, assignment.GetLastTransition().After(initialAssignmentTransition))
		require.True(tc, status.LastProcessed.Equal(initialTargetProcessed))
	}, time.Minute, 100*time.Millisecond)

	// Wait for the assignment to be reprocessed and the target to be reprocessed,
	// i.e. target reprocessed outside of backoff.
	require.EventuallyWithT(t, func(tc *assert.CollectT) {
		assignment := mustGetAssignmentForUser(tc, sut, userLogin)
		require.Equal(tc, constants.OktaAssignmentStatusFailed, assignment.GetStatus())

		targets := assignment.GetTargets()
		require.Len(tc, targets, 1)

		status := targets[0].GetStatus()
		require.NotNil(tc, status)

		// If the last transition time of the assignment is after the initial failed assignment transition time,
		// and the last processed time of the target is after the initial failed processed time,
		// then the assignment has been reprocessed and the target has been reprocessed, indicating backoff has expired.
		require.True(tc, assignment.GetLastTransition().After(initialAssignmentTransition))
		require.True(tc, status.LastProcessed.After(initialTargetProcessed))
	}, time.Minute, 100*time.Millisecond)
}

// TestOktaAssignmentProcessingSuspendedUser verifies the lifecycle of suspending
// then reactivating a user in Okta.
// 1. Active: assignments processed
// 2. Suspended: cleanup assignments processed, provision assignments skipped
// 3. Active: assignments processed
func TestOktaAssignmentProcessingSuspendedUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(withUserCount(2), withGroupCount(2), withSAMLApp())
	t.Cleanup(fakeOkta.Stop)

	owner := fakeOkta.provisionedUsers[0]
	ownerLogin := oktaUserLogin(owner)

	user := fakeOkta.provisionedUsers[1]
	userLogin := oktaUserLogin(user)

	group1 := fakeOkta.provisionedGroups[0].Id
	fakeOkta.AddUserToGroup(group1, owner.Id)

	group2 := fakeOkta.provisionedGroups[1].Id

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
		common.WithUser(t, "alice-admin", "editor"),
	)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	authServer := sut.Teleport.Process.GetAuthServer()

	createAndWaitForOktaIntegration(t, sut, fakeOkta,
		withAccessListSettings(oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{ownerLogin},
		}.Build()), withEnableFullSync())

	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Hour,
		timeBetweenAssignmentProcessLoops: time.Second,
		targetProcessingBackoffStep:       time.Second,
		targetProcessingBackoffMax:        time.Second,
	})

	waitForOktaSync(t, sut, withTimeout(time.Minute), withStep(time.Millisecond*100), withTimePoint(time.Now()))
	waitForPerUserOktaAssignments(t, assignmentWatcher, 1)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), ownerLogin)

	staticAssignment := mustGetAssignmentForUser(t, sut, ownerLogin)

	// Provision assignment for active user is processed.
	mustAddAccessListMember(t, sut, group1, userLogin)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == userLogin && a.GetStatus() == constants.OktaAssignmentStatusSuccessful
	})
	require.True(t, fakeOkta.UserAssignedGroup(group1, user.Id))
	userAssignment := mustGetAssignmentForUser(t, sut, userLogin)

	// Suspend the user.
	require.NoError(t, fakeOkta.SuspendUser(user.Id))

	// Cleanup assignment for suspended user is processed and finalized.
	require.NoError(t, authServer.DeleteAccessListMember(ctx, group1, userLogin))
	common.WaitForDeleteEvent(t, assignmentWatcher, func(r types.Resource) bool {
		return r.GetName() == userAssignment.GetName()
	})
	require.False(t, fakeOkta.UserAssignedGroup(group1, user.Id))

	// Provision assignment for suspended user is not processed.
	mustAddAccessListMember(t, sut, group2, userLogin)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == userLogin && a.GetStatus() == constants.OktaAssignmentStatusPending
	})
	// Happy path is pending->processing->successful/failed.
	// Target assignment is still pending and user remains in Okta group.
	waitForNAssignmentTransitions(t, assignmentWatcher, sut.Clock.Now(), staticAssignment.GetName(), 3)
	userAssignment = mustGetAssignmentForUser(t, sut, userLogin)
	require.Equal(t, constants.OktaAssignmentStatusPending, userAssignment.GetStatus())
	require.False(t, fakeOkta.UserAssignedGroup(group2, user.Id))

	// Provision assignment for reactivated user is processed.
	fakeOkta.ActivateUser(user.Id)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == userLogin && a.GetStatus() == constants.OktaAssignmentStatusSuccessful
	})
	require.True(t, fakeOkta.UserAssignedGroup(group2, user.Id))
}

// TestOktaAssignmentProcessingRemovedUser verifies the end-to-end process of removing
// a user from Okta (active -> deactivated -> removed) and the assignment processing at each point:
//
// 1. Active: assignments processed
// 2. Deactivated: cleanup assignments processed, provision assignments skipped
// 3. Removed: assignments deleted
func TestOktaAssignmentProcessingRemovedUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(withUserCount(3), withGroupCount(2), withSAMLApp())
	t.Cleanup(fakeOkta.Stop)

	owner := fakeOkta.provisionedUsers[0]
	ownerLogin := oktaUserLogin(owner)

	user1 := fakeOkta.provisionedUsers[1]
	user1Login := oktaUserLogin(user1)

	user2 := fakeOkta.provisionedUsers[2]
	user2Login := oktaUserLogin(user2)

	group1 := fakeOkta.provisionedGroups[0].Id
	fakeOkta.AddUserToGroup(group1, owner.Id)

	group2 := fakeOkta.provisionedGroups[1].Id

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
		common.WithUser(t, "alice-admin", "editor"),
	)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	authServer := sut.Teleport.Process.GetAuthServer()

	createAndWaitForOktaIntegration(t, sut, fakeOkta,
		withAccessListSettings(oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{ownerLogin},
		}.Build()), withEnableFullSync())

	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                time.Hour,
		timeBetweenAssignmentProcessLoops: time.Second,
		targetProcessingBackoffStep:       time.Second,
		targetProcessingBackoffMax:        time.Second,
	})

	waitForOktaSync(t, sut, withTimeout(time.Minute), withStep(time.Millisecond*100), withTimePoint(time.Now()))
	waitForPerUserOktaAssignments(t, assignmentWatcher, 1)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), ownerLogin)

	staticAssignment := mustGetAssignmentForUser(t, sut, ownerLogin)

	// Provision assignments for active user1 and user2 are processed successfully and users added to Okta group.
	mustAddAccessListMember(t, sut, group1, user1Login)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == user1Login && a.GetStatus() == constants.OktaAssignmentStatusSuccessful
	})
	require.True(t, fakeOkta.UserAssignedGroup(group1, user1.Id))
	user1Assignment := mustGetAssignmentForUser(t, sut, user1Login)

	mustAddAccessListMember(t, sut, group1, user2Login)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == user2Login && a.GetStatus() == constants.OktaAssignmentStatusSuccessful
	})
	require.True(t, fakeOkta.UserAssignedGroup(group1, user2.Id))
	user2Assignment := mustGetAssignmentForUser(t, sut, user2Login)

	// Cleanup assignment for deactivated user1 is finalized and user removed from Okta group.
	require.NoError(t, fakeOkta.DeactivateUser(user1.Id))
	require.NoError(t, authServer.DeleteAccessListMember(ctx, group1, user1Login))
	common.WaitForDeleteEvent(t, assignmentWatcher, func(r types.Resource) bool {
		return r.GetName() == user1Assignment.GetName()
	})
	require.False(t, fakeOkta.UserAssignedGroup(group1, user1.Id))

	// Provision assignment for deactivated user1 is skipped and user remains in Okta group.
	mustAddAccessListMember(t, sut, group2, user1Login)
	waitForResource(t, assignmentWatcher, func(a types.OktaAssignment) bool {
		return a.GetUser() == user1Login && a.GetStatus() == constants.OktaAssignmentStatusPending
	})
	// 3 transitions of static assignment guarantees at least 1 full cycle for user assignment.
	// Happy path is pending->processing->successful/failed.
	// Target assignment is still pending and user remains in Okta group.
	waitForNAssignmentTransitions(t, assignmentWatcher, sut.Clock.Now(), staticAssignment.GetName(), 3)
	user1Assignment = mustGetAssignmentForUser(t, sut, user1Login)
	require.Equal(t, constants.OktaAssignmentStatusPending, user1Assignment.GetStatus())
	require.False(t, fakeOkta.UserAssignedGroup(group2, user1.Id))

	// Provision assignment for removed user1 is deleted.
	require.NoError(t, fakeOkta.RemoveUser(user1.Id))
	common.WaitForDeleteEvent(t, assignmentWatcher, func(r types.Resource) bool {
		return r.GetName() == user1Assignment.GetName()
	})

	// Cleanup assignment for removed user2 is removed.
	require.NoError(t, fakeOkta.RemoveUser(user2.Id))
	require.NoError(t, authServer.DeleteAccessListMember(ctx, group1, user2Login))
	common.WaitForDeleteEvent(t, assignmentWatcher, func(r types.Resource) bool {
		return r.GetName() == user2Assignment.GetName()
	})
}

// access list sync propagates group membership changes even when bidirectional sync
// (the Teleport -> Okta assignment processor) is disabled.
//
// When bidirectional sync is disabled, the flow  still creates OktaAssignment
// records. However, because the assignment processor loop is not running, the
// ongoing assignments filter should be skipped during access list sync.
func TestAccessListSyncWithBidirectionalSyncDisabled(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withUserCount(1),
		withAppCount(1),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	for _, u := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, u.Id))
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedApps[0].Id, u.Id))
	}

	memberID := fakeOkta.provisionedUsers[0].Id
	memberLogin := oktaUserLogin(fakeOkta.provisionedUsers[0])
	groupID := fakeOkta.provisionedGroups[0].Id

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	// Start with access-list sync enabled but bidirectional sync (assignment processor)
	// disabled. Only the Okta -> Teleport direction is active.
	createAndWaitForOktaIntegration(t, sut, fakeOkta,
		withAccessListSettings(oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{},
			DefaultOwner: []string{"alice-admin"},
		}.Build()),
		withAccessListSyncEnabledNoBidirectional(),
	)
	waitForPerUserOktaAssignments(t, assignmentWatcher, 1)

	assertUserIsNotAccessListMember(ctx, t, sut, groupID, memberLogin)

	fakeOkta.AddUserToGroup(groupID, memberID)

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)
	assertUserIsAccessListMember(ctx, t, sut, groupID, memberLogin)
	fakeOkta.RemoveUserFromGroup(groupID, memberID)

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)
	assertUserIsNotAccessListMember(ctx, t, sut, groupID, memberLogin)
}

// waitForNAssignmentTransitions waits for the assignment to transition n times.
// When provided with a 'static' assignment, it can be used to wait for n iterations of the assignment processor loop.
func waitForNAssignmentTransitions(t *testing.T, watcher types.Watcher, since time.Time, assignmentName string, n int) {
	t.Helper()

	for range n {
		waitForResource(t, watcher, func(a types.OktaAssignment) bool {
			if a.GetName() != assignmentName || !a.GetLastTransition().After(since) {
				return false
			}

			since = a.GetLastTransition()
			return true
		})
	}
}
