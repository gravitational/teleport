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
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/modules"
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
	waitForOktaFirstOktaAssignment(t, sut)
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
	waitForOktaFirstOktaAssignment(t, sut)
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

	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
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
	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:            apiCredentials,
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        true,
		EnableAccessListSync:      true,
		EnableBidirectionalSync:   true,
		AccessListSettings: &oktav1.AccessListSettings{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{reviewerLogin},
		},
		ReuseConnector: "okta-pre-created-test",
	})
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

func assertUserIsNotAccessListMember(ctx context.Context, t require.TestingT, sut *common.SUT, acl, user string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := sut.Teleport.Process.GetAuthServer().AccessListsInternal.GetAccessListMember(ctx, acl, user)
		require.True(t, trace.IsNotFound(err))
	}, time.Minute, 50*time.Millisecond, "User %s should not be a member of access list %s", user, acl)
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

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(1),
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

	start := time.Now()
	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta, witAccessListSettings(&oktav1.AccessListSettings{
		GroupFilters: []string{"group-*"},
		AppFilters:   []string{"app-*"},
		DefaultOwner: []string{reviewerLogin},
	}), withEnableFullSync())
	scimClient := createSCIMClient(t, sut, scimToken)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		s, err := sut.Teleport.Process.GetAuthServer().GetUserLoginState(ctx, reviewerLogin)
		require.NoError(t, err)
		require.Len(t, s.GetRoles(), 2) // okta-requester + 2 ACL reviewer roles
	}, time.Second, time.Millisecond*100)

	auth := sut.Teleport.Process.GetAuthServer()
	userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, userGroups)
	group := selectUserGroupByName(userGroups, fakeOkta.provisionedGroups[0].Id)
	require.NotNil(t, group)
	groupID := group.GetName()

	accessRequest := createAccessRequest(t, sut, groupID, types.KindUserGroup, requesterLogin)

	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
	approveAccessRequest(t, sut, accessRequest.GetName(), reviewerLogin)

	g, err := scimClient.GetGroup(ctx, fakeOkta.provisionedGroups[0].Id)
	require.NoError(t, err)
	g.Members = append(g.Members, &scimsdk.GroupMember{
		ExternalID: requesterLogin,
	})
	_, err = scimClient.UpdateGroup(t.Context(), g)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))
	}, time.Second*10, time.Millisecond*250, "User %s was never assigned to group %s", requester.Id, fakeOkta.provisionedGroups[0].Id)

	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)

	err = auth.DeleteAccessRequest(t.Context(), accessRequest.GetName())
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.True(t, fakeOkta.UserAssignedGroup(fakeOkta.provisionedGroups[0].Id, requester.Id))
	}, time.Second*10, time.Millisecond*250, "User %s was never assigned to group %s", requester.Id, fakeOkta.provisionedGroups[0].Id)

	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requesterLogin)
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
	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true,
		AccessListSettings: &oktav1.AccessListSettings{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{"alice-admin"},
		},
		ReuseConnector: "okta-pre-created-test",
	})
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
	for range iterCount {
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
		}, time.Minute, time.Millisecond*30)
	}
}
