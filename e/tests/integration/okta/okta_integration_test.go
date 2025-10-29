package okta

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// TestBasicAssignmentFlow tests the basic assignment flow.
// That tests the basic assignment flow for the apps groups imported by Okta integration
// making sure that removing/adding members to the access list will trigger the assignment/unassignment
// and the proper Okta API calls are made.
func TestBasicAssignmentFlow(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(1, 2, 7))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[0].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[4].Id)
	defaultOwner := oktaInfra.Users[5]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	beforeInstall := time.Now()

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, oktaApiClient.GetOrgUrl(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", defaultOwner.login()),
	})

	// Set time between imports to 1s
	authServer := sut.Teleport.Process.GetAuthServer()
	plugin, err := oktaplugin.Get(ctx, authServer.Plugins, true)
	require.NoError(t, err)
	plugin.Spec.GetOkta().SyncSettings.TimeBetweenImports = "1s"
	_, err = authServer.Plugins.UpdatePlugin(ctx, plugin)
	require.NoError(t, err)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(beforeInstall))
	waitForOktaFirstOktaAssignment(t, sut)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaInfra.Users[0])

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		wantMembers := []*accesslist.AccessListMember{
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[0].login()}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[1].login()}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[4].login()}}},
		}
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tclCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, oktaInfra.Groups[0].Id,
		}, &accessListMembers)
		assertResourcesByName(t, wantMembers, accessListMembers)
	}, time.Second*10, time.Millisecond*100)
	tclCmd.run(t, []string{
		`acl`, `users`, `rm`, oktaInfra.Groups[0].Id, oktaInfra.Users[0].login(),
	})
	oktaInfra.assertUserWasUnassignedFromOktaGroup(t, oktaInfra.Users[0].Id, oktaInfra.Groups[0].Id)

	tclCmd.run(t, []string{
		`acl`, `users`, `add`, oktaInfra.Groups[0].Id, oktaInfra.Users[2].login(),
	})
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[2].Id, oktaInfra.Groups[0].Id)
}

// TestNestedAclAssignment tests the assignment and sync of nested access lists.
// It ensures that members from nested access lists are flattened and added to the root access list in Okta,
// and that on reconciliation during sync, the nested lists are not flattened on the Teleport side.
func TestNestedAclAssignment(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(1, 2, 7))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[2].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[3].Id)
	defaultOwner := oktaInfra.Users[0]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	authServer := sut.Teleport.Process.GetAuthServer()

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	beforeInstall := time.Now()

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, oktaApiClient.GetOrgUrl(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", defaultOwner.login()),
	})

	// Set time between imports to 1s
	plugin, err := oktaplugin.Get(ctx, authServer.Plugins, true)
	require.NoError(t, err)
	plugin.Spec.GetOkta().SyncSettings.TimeBetweenImports = "1s"
	_, err = authServer.Plugins.UpdatePlugin(ctx, plugin)
	require.NoError(t, err)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(beforeInstall))
	waitForOktaFirstOktaAssignment(t, sut)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), defaultOwner)

	oktaSyncedList, err := authServer.GetAccessList(ctx, oktaInfra.Groups[0].Id)
	require.NoError(t, err)
	teleportList0 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-0", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"access"}}, []string{oktaInfra.Users[4].login()})
	teleportList1 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-1", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"editor"}}, []string{oktaInfra.Users[5].login()})
	teleportList2 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-2", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"reviewer"}}, []string{oktaInfra.Users[6].login()})

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
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[4].Id, oktaInfra.Groups[0].Id)
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[5].Id, oktaInfra.Groups[0].Id)
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[6].Id, oktaInfra.Groups[0].Id)

	// ensure membership on Teleport side isn't flattened after sync w/ okta
	assertAccessListMembers(t, ctx, sut, oktaInfra.Groups[0].Id, []string{
		// from root list
		oktaInfra.Users[1].login(),
		oktaInfra.Users[2].login(),
		oktaInfra.Users[3].login(),
		// nested list members
		nestedTeleportList0.GetName(),
	})

	// nested lists should contain their members
	assertAccessListMembers(t, ctx, sut, nestedTeleportList0.GetName(), []string{
		oktaInfra.Users[4].login(),
		nestedTeleportList1.GetName(),
		nestedTeleportList2.GetName(),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList1.GetName(), []string{
		oktaInfra.Users[5].login(),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList2.GetName(), []string{
		oktaInfra.Users[6].login(),
	})
}

// TestAccessRequest tests the access request flow. That tests the access request flow for the apps groups imported
// by Okta integration where the okta-requester role is used and the access list owner is able to review
// the access request and approve it.
func TestAccessRequest(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(1, 2, 7))

	oktaInfra.createApplicationGroupAssignment(t, oktaInfra.Apps[0].Id, oktaInfra.Groups[0].Id)
	reviewer := oktaInfra.Users[5]
	requester := oktaInfra.Users[4]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	beforeInstall := time.Now()

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, oktaApiClient.GetOrgUrl(),
		`--saml-connector`, `okta-pre-created-test`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", reviewer.login()),
	})

	// Set time between imports to 1s
	authServer := sut.Teleport.Process.GetAuthServer()
	plugin, err := oktaplugin.Get(ctx, authServer.Plugins, true)
	require.NoError(t, err)
	plugin.Spec.GetOkta().SyncSettings.TimeBetweenImports = "1s"
	_, err = authServer.Plugins.UpdatePlugin(ctx, plugin)
	require.NoError(t, err)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(beforeInstall))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var wantMembers []*accesslist.AccessListMember
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tclCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, oktaInfra.Groups[0].Id,
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
		tshRequester.mustLogin(t, requester.login(), append(oktaInfra.getUserGroups(t, requester.Id), "Everyone"))
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
			err := tshRequester.run(t, []string{
				`request`, `create`, `--resource`, fmt.Sprintf(`/local-site/app/%s`, mustGetAppIDbyAppLabel(t, sut, oktaInfra)), `--nowait`,
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
		tshReviewer.mustLogin(t, reviewer.login(), append(oktaInfra.getUserGroups(t, requester.Id), "Everyone"))
		var accReqs []*types.AccessRequestV3
		mustRunTSHAndGetResultAs(t, tshReviewer, []string{
			`request`, `ls`, `--format=json`,
		}, &accReqs)
		require.Len(t, accReqs, 1)

		err := tshReviewer.run(t, []string{
			`request`, `review`, fmt.Sprintf(`--approve=%s`, accReqs[0].GetName()),
		})
		require.NoError(t, err)
		requestID = accReqs[0].GetName()
	})

	t.Run("requester is able to access the app", func(t *testing.T) {
		err := tshRequester.run(t, []string{
			`login`, fmt.Sprintf(`--request-id=%s`, requestID),
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
				{Metadata: types.Metadata{Description: oktaInfra.Apps[0].Label}},
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
	ctx := context.Background()
	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(1, 1, 7))
	oktaInfra.createApplicationGroupAssignment(t, oktaInfra.Apps[0].Id, oktaInfra.Groups[0].Id)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	reviewer := oktaInfra.Users[5]
	requester := oktaInfra.Users[4]

	_, _, err := oktaApiClient.AssignUserToApplication(t.Context(), oktaInfra.Apps[0].Id, okta.AppUser{Id: oktaInfra.Users[0].Id})
	require.NoError(t, err)

	oktaSAMLAppName := "trial-1234567_teleportsamlconnectorapp_1"
	samlApp := createOktaSAMLAPP(t, ctx, oktaApiClient, oktaSAMLAppName)
	for _, user := range oktaInfra.Users {
		_, _, err := oktaApiClient.AssignUserToApplication(t.Context(), samlApp.Id, okta.AppUser{Id: user.Id})
		require.NoError(t, err)
	}

	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	start := time.Now()
	_, err = oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:        durationpb.New(1 * time.Second),
		ApiCredentials:            apiCredentials,
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        true,
		EnableAccessListSync:      true,
		EnableBidirectionalSync:   true,
		AccessListSettings: &oktav1.AccessListSettings{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{reviewer.login()},
		},
		ReuseConnector: "okta-pre-created-test",
	})
	require.NoError(t, err)
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	auth := sut.Teleport.Process.GetAuthServer()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		s, err := auth.GetUserLoginState(ctx, reviewer.login())
		require.NoError(t, err)
		require.Len(t, s.GetRoles(), 3) // okta-requester + 2 ACL reviewer roles
	}, time.Second, time.Millisecond*100)

	t.Run("app access request", func(t *testing.T) {
		var app types.AppServer
		apps, err := auth.GetApplicationServers(t.Context(), "default")
		require.NoError(t, err)
		for _, v := range apps {
			if v.GetMetadata().Description != oktaSAMLAppName {
				app = v
				break
			}
		}

		appID, ok := app.GetLabel(eteleport.OktaAppIDLabel)
		require.True(t, ok)
		oktaInfra.assertUsersIsNotAssignedToOktaApp(t, requester.Id, appID)
		assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requester.login())

		t.Run("delete app access request", func(t *testing.T) {
			accessRequestApp := createAccessRequest(t, sut, app.GetName(), types.KindApp, requester.login())
			approveAccessRequest(t, sut, accessRequestApp.GetName(), reviewer.login())

			oktaInfra.assertUserWasAssignedToOktaApp(t, requester.Id, appID)
			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requester.login())

			err = auth.DeleteAccessRequest(t.Context(), accessRequestApp.GetName())
			require.NoError(t, err)
			oktaInfra.assertUsersIsNotAssignedToOktaApp(t, requester.Id, appID)
			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requester.login())
		})

		t.Run("member was added to acl before access request was deleted", func(t *testing.T) {
			accessRequestApp := createAccessRequest(t, sut, app.GetName(), types.KindApp, requester.login())
			approveAccessRequest(t, sut, accessRequestApp.GetName(), reviewer.login())

			oktaInfra.assertUserWasAssignedToOktaApp(t, requester.Id, appID)
			assertUserIsNotAccessListMember(ctx, t, sut, app.GetName(), requester.login())

			mustAddAccessListMember(t, sut, app.GetName(), requester.login())

			err = sut.Teleport.Process.GetAuthServer().DeleteAccessRequest(t.Context(), accessRequestApp.GetName())
			require.NoError(t, err)

			oktaInfra.assertUserWasAssignedToOktaApp(t, requester.Id, appID)
			assertUserIsAccessListMember(ctx, t, sut, app.GetName(), requester.login())
		})
	})

	t.Run("group access request", func(t *testing.T) {
		userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
		require.NoError(t, err)
		require.NotEmpty(t, userGroups)
		group := selectUserGroupByName(userGroups, oktaInfra.Groups[0].Id)
		require.NotNil(t, group)
		groupID := group.GetName()

		t.Run("delete group access request", func(t *testing.T) {
			accessRequest := createAccessRequest(t, sut, groupID, types.KindUserGroup, requester.login())

			assertUserIsNotAccessListMember(ctx, t, sut, groupID, requester.login())
			approveAccessRequest(t, sut, accessRequest.GetName(), reviewer.login())
			oktaInfra.assertUserWasAssignedToOktaGroup(t, requester.Id, oktaInfra.Groups[0].Id)

			err = auth.DeleteAccessRequest(t.Context(), accessRequest.GetName())
			require.NoError(t, err)

			oktaInfra.assertUserWasUnassignedFromOktaGroup(t, requester.Id, oktaInfra.Groups[0].Id)
			assertUserIsNotAccessListMember(ctx, t, sut, groupID, requester.login())
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
	}, 3*time.Second, 50*time.Millisecond, "User %s should not be a member of access list %s", user, acl)
}

func assertUserIsAccessListMember(ctx context.Context, t require.TestingT, sut *common.SUT, acl, user string) *accesslist.AccessListMember {
	var member *accesslist.AccessListMember
	var err error
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		member, err = sut.Teleport.Process.GetAuthServer().AccessListsInternal.GetAccessListMember(ctx, acl, user)
		require.NoError(t, err)
	}, 3*time.Second, 200*time.Millisecond)
	return member
}

// TestOktaAccessRequestWithSCIMOktaSync tests access requests when SCIM Okta sync is enabled.
// When an access request is approved, the user should be added to the corresponding Okta group.
// However, the user should not be synced back as an access list member via SCIM Group update,
// as this would escalate // their privileges to long-term access.
// Even after the access request expires, the user would
// still remain a member of the access list.
func TestOktaAccessRequestWithSCIMOktaSync(t *testing.T) {
	ctx := t.Context()
	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(1, 1, 7))
	oktaInfra.createApplicationGroupAssignment(t, oktaInfra.Apps[0].Id, oktaInfra.Groups[0].Id)

	reviewer := oktaInfra.Users[5]
	requester := oktaInfra.Users[4]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	start := time.Now()
	scimToken := createAndWaitForOktaIntegration(t, sut, oktaApiClient, witAccessListSettings(&oktav1.AccessListSettings{
		GroupFilters: []string{"group-*"},
		AppFilters:   []string{"app-*"},
		DefaultOwner: []string{reviewer.login()},
	}), withEnableFullSync())
	scimClient := createSCIMClient(t, sut, scimToken)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		s, err := sut.Teleport.Process.GetAuthServer().GetUserLoginState(ctx, reviewer.login())
		require.NoError(t, err)
		require.Len(t, s.GetRoles(), 2) // okta-requester + 2 ACL reviewer roles
	}, time.Second, time.Millisecond*100)

	auth := sut.Teleport.Process.GetAuthServer()
	userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, userGroups)
	group := selectUserGroupByName(userGroups, oktaInfra.Groups[0].Id)
	require.NotNil(t, group)
	groupID := group.GetName()

	accessRequest := createAccessRequest(t, sut, groupID, types.KindUserGroup, requester.login())

	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requester.login())
	approveAccessRequest(t, sut, accessRequest.GetName(), reviewer.login())

	g, err := scimClient.GetGroup(ctx, oktaInfra.Groups[0].Id)
	require.NoError(t, err)
	g.Members = append(g.Members, &scimsdk.GroupMember{
		ExternalID: requester.login(),
	})
	_, err = scimClient.UpdateGroup(t.Context(), g)
	require.NoError(t, err)

	oktaInfra.assertUserWasAssignedToOktaGroup(t, requester.Id, oktaInfra.Groups[0].Id)
	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requester.login())

	err = auth.DeleteAccessRequest(t.Context(), accessRequest.GetName())
	require.NoError(t, err)

	oktaInfra.assertUserWasUnassignedFromOktaGroup(t, requester.Id, oktaInfra.Groups[0].Id)
	assertUserIsNotAccessListMember(ctx, t, sut, groupID, requester.login())
}

func TestOktaAssignmentRaceCheck(t *testing.T) {
	ctx := context.Background()
	oktaAPIClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaAPIClient, withAppsGroupsUsersCount(0, 1, 7))

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	reviewer := oktaInfra.Users[5]
	requester := oktaInfra.Users[4]

	oktaSAMLAppName := "trial-1234567_teleportsamlconnectorapp_1"
	samlApp := createOktaSAMLAPP(t, ctx, oktaAPIClient, oktaSAMLAppName)
	for _, user := range oktaInfra.Users {
		_, _, err := oktaAPIClient.AssignUserToApplication(t.Context(), samlApp.Id, okta.AppUser{Id: user.Id})
		require.NoError(t, err)
	}

	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	start := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:      durationpb.New(1 * time.Second),
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true,
		AccessListSettings: &oktav1.AccessListSettings{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{reviewer.login()},
		},
		ReuseConnector: "okta-pre-created-test",
	})
	require.NoError(t, err)
	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100), withTimePoint(start))

	auth := sut.Teleport.Process.GetAuthServer()

	userGroups, _, err := auth.ListUserGroups(t.Context(), 0, "")
	require.NoError(t, err)
	require.NotEmpty(t, userGroups)
	group := selectUserGroupByName(userGroups, oktaInfra.Groups[0].Id)
	require.NotNil(t, group)

	const iterCount = 10
	for range iterCount {
		oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, requester.Id)
		m := assertUserIsAccessListMember(ctx, t, sut, group.GetName(), requester.login())
		require.Equal(t, "okta-service", m.Spec.AddedBy)
		oktaInfra.removeUserFromGroup(t, oktaInfra.Groups[0].Id, requester.Id)
		assertUserIsNotAccessListMember(ctx, t, sut, group.GetName(), requester.login())
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assignments, _, err := sut.Teleport.Process.GetAuthServer().Okta.ListOktaAssignments(ctx, 0, "")
			require.NoError(t, err)
			require.Empty(t, assignments)
		}, time.Second*5, time.Millisecond*30)
	}

	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, requester.Id)
	assertUserIsAccessListMember(ctx, t, sut, group.GetName(), requester.login())
	for range iterCount {
		oktaInfra.removeUserFromGroup(t, oktaInfra.Groups[0].Id, requester.Id)
		assertUserIsNotAccessListMember(ctx, t, sut, group.GetName(), requester.login())
		oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, requester.Id)
		assertUserIsAccessListMember(ctx, t, sut, group.GetName(), requester.login())
	}
	oktaInfra.removeUserFromGroup(t, oktaInfra.Groups[0].Id, requester.Id)
	assertUserIsNotAccessListMember(ctx, t, sut, group.GetName(), requester.login())
}
