package okta

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
)

// Proves that a Teleport editor user can't modify Access List members when bidirectional sync is
// disabled.
func Test_AccessList_readOnly_members(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create Okta users.
	const ghostEmail = "ghost@example.com"
	ghostUser := fakeOkta.CreateUser("ghost")

	const specterEmail = "specter@example.com"
	fakeOkta.CreateUser("specter")

	// Assign only user1.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, ghostUser.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	// 1. Create the integration with bidirectional sync disabled
	beforeCreateIntegrationTime := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		ReuseConnector: "okta-pre-created-test",
	})
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	// 2. Verify users (user1 - ghost) are synchronized
	var oktaUsers []types.User
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimePoint(beforeCreateIntegrationTime))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		users, err := sut.Teleport.Process.GetAuthServer().GetUsers(ctx, false /* withSecrets */)
		require.NoError(t, err)
		oktaUsers = oktaUsers[:0] // clear
		for _, u := range users {
			if v, _ := u.GetLabel("teleport.dev/origin"); v == "okta" {
				oktaUsers = append(oktaUsers, u)
			}
		}
		require.Len(t, oktaUsers, 1, "expected 1 Okta users in all_users = %v", users)
	}, time.Second*2, time.Millisecond*50)

	// 3. Remember the name of the AL

	var accessList *accesslist.AccessList
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		accessLists, err := authServer.GetAccessLists(ctx)
		require.NoError(t, err)
		require.Len(t, accessLists, 1)
		accessList = accessLists[0]
		require.NotEmpty(t, accessList.Spec.Title)
		require.Equal(t, fakeOkta.provisionedSAMLApp.Label, accessList.Spec.Title)
	}, time.Second*2, time.Millisecond*50)

	// 4. Verify members (user1 - ghost)

	var member1 *accesslist.AccessListMember

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, _, err := authServer.ListAccessListMembers(ctx, accessList.GetName(), 1000, "")
		require.NoError(t, err)
		require.Len(t, members, 1)
		member1 = members[0]
		require.Equal(t, ghostEmail, member1.GetName())
	}, time.Second*2, time.Millisecond*50)

	// 5. Prepare user's client and member2 (specter) struct

	aliceAccessListClient := accesslistv1.NewAccessListServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	member2, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: specterEmail,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList.GetName(),
			Name:       specterEmail,
			Joined:     time.Now(),
			Expires:    time.Now().Add(24 * time.Hour),
			Reason:     "tests reason",
			AddedBy:    "alice-admin",
		},
	)
	require.NoError(t, err)

	// 6. Try (and fail) to upsert member2 (specter)

	_, err = aliceAccessListClient.UpsertAccessListMember(ctx, &accesslistv1.UpsertAccessListMemberRequest{
		Member: conv.ToMemberProto(member2),
	})
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "adding and updating members not allowed")

	// 5. Try (and fail) to update member1 (ghost)

	_, err = aliceAccessListClient.UpdateAccessListMember(ctx, &accesslistv1.UpdateAccessListMemberRequest{
		Member: conv.ToMemberProto(member1),
	})
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "updating members not allowed")

	// 8. Try (and succeed) to upsert access list with the _already existing_ members

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1)},
	})
	require.NoError(t, err)

	// 9. Try (and fail) to upsert access list with _a new_ member (member1 exists, member2 added)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1), conv.ToMemberProto(member2)},
	})
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 10. Try (and fail) to upsert access list with _removed_ member (member1 removed)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{},
	})
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 11. Try (and fail) to upsert access list with _replaced_ (member1 removed, member2 added)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member2)},
	})
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 12. Enable bidirectional sync

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, &oktav1.UpdateIntegrationRequest{
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true, // enabled
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
	})

	// 13. Now adding a member should work

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, &accesslistv1.UpsertAccessListWithMembersRequest{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1), conv.ToMemberProto(member2)},
	})
	require.NoError(t, err)
}

// Verifies Okta changes to apps are reflected in Teleport when bidirectional sync is disabled.
func Test_AccessList_readOnly_pulls_from_Okta(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create Okta users.
	const ghostEmail = "ghost@example.com"
	ghostUser := fakeOkta.CreateUser("ghost")

	const specterEmail = "specter@example.com"
	specterUser := fakeOkta.CreateUser("specter")

	// Assign only user1.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, ghostUser.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	// 1. Create the integration with bidirectional sync disabled

	beforeCreationTime := time.Now()

	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		ReuseConnector: "okta-pre-created-test",
	})
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	waitForOktaSync(t, sut, withTimePoint(beforeCreationTime))

	// 2. Verify users (user1 - ghost) are synchronized

	var oktaUsers []types.User

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		users, err := sut.Teleport.Process.GetAuthServer().GetUsers(ctx, false /* withSecrets */)
		require.NoError(t, err)
		oktaUsers = oktaUsers[:0] // clear
		for _, u := range users {
			if v, _ := u.GetLabel("teleport.dev/origin"); v == "okta" {
				oktaUsers = append(oktaUsers, u)
			}
		}
		require.Len(t, oktaUsers, 1, "expected 1 Okta users in all_users = %v", users)
	}, time.Second*2, time.Millisecond*50)

	// 3. Remember the name of the AL

	var accessList *accesslist.AccessList

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		accessLists, err := authServer.GetAccessLists(ctx)
		require.NoError(t, err)
		require.Len(t, accessLists, 1)
		accessList = accessLists[0]
		require.NotEmpty(t, accessList.Spec.Title)
		require.Equal(t, fakeOkta.provisionedSAMLApp.Label, accessList.Spec.Title)
	}, time.Second*2, time.Millisecond*50)

	// 4. Verify members (user1 - ghost)

	var member1 *accesslist.AccessListMember

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, _, err := authServer.ListAccessListMembers(ctx, accessList.GetName(), 1000, "")
		require.NoError(t, err)
		require.Len(t, members, 1)
		member1 = members[0]
		require.Equal(t, ghostEmail, member1.GetName())
	}, time.Second*2, time.Millisecond*50)

	// 5. Assign user2 (specter) to the SAML app on the Okta side

	err = fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, specterUser.Id)
	require.NoError(t, err)

	// 6. Verify user2 (specter) is synchronized to the Access List

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, nextToken, err := authServer.ListAccessListMembers(ctx, accessList.GetName(), 1000, "")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Len(t, members, 2, "members = %v", members)
		for _, m := range members {
			require.True(t, m.GetName() == ghostEmail || m.GetName() == specterEmail, "member name = %q", m.GetName())
		}
	}, time.Second*2, time.Millisecond*50)
}
