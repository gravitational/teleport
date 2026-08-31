package okta

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
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

	app := fakeOkta.CreateBasicApp("read-only-members-app")

	// Assign only user1.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, ghostUser.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(app.Id, ghostUser.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)
	accessListWatcher := sut.NewResourceWatcher(t, types.KindAccessList)
	memberWatcher := sut.NewResourceWatcher(t, types.KindAccessListMember)
	pluginWatcher := sut.NewResourceWatcher(t, types.KindPlugin)

	// 1. Create the integration with bidirectional sync disabled
	beforeCreateIntegrationTimePoint := time.Now()
	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	// 2. Verify users (user1 - ghost) are synchronized
	waitForUserSync(t, pluginWatcher, beforeCreateIntegrationTimePoint)
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimePoint(beforeCreateIntegrationTimePoint))

	waitForResourceCount(t, userWatcher, 1, func(u types.User) bool {
		return u.Origin() == types.OriginOkta
	})

	// 3. Remember the name of the AL
	waitForAccessListSync(t, pluginWatcher, beforeCreateIntegrationTimePoint)
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimePoint(beforeCreateIntegrationTimePoint))

	accessLists := waitForResourceCount(t, accessListWatcher, 1, func(*accesslist.AccessList) bool {
		return true
	})
	accessList := accessLists[0]
	require.NotEmpty(t, accessList.Spec.Title)
	require.Equal(t, app.Label, accessList.Spec.Title)

	// 4. Verify members (user1 - ghost)

	member1 := waitForResource(t, memberWatcher, func(m *accesslist.AccessListMember) bool {
		return m.Spec.AccessList == accessList.GetName()
	})
	require.Equal(t, ghostEmail, member1.GetName())

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

	_, err = aliceAccessListClient.UpsertAccessListMember(ctx, accesslistv1.UpsertAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(member2),
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "adding and updating members not allowed")

	// 5. Try (and fail) to update member1 (ghost)

	_, err = aliceAccessListClient.UpdateAccessListMember(ctx, accesslistv1.UpdateAccessListMemberRequest_builder{
		Member: conv.ToMemberProto(member1),
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "updating members not allowed")

	// 8. Try (and succeed) to upsert access list with the _already existing_ members

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1)},
	}.Build())
	require.NoError(t, err)

	// 9. Try (and fail) to upsert access list with _a new_ member (member1 exists, member2 added)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1), conv.ToMemberProto(member2)},
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 10. Try (and fail) to upsert access list with _removed_ member (member1 removed)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{},
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 11. Try (and fail) to upsert access list with _replaced_ (member1 removed, member2 added)

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member2)},
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")

	// 12. Enable bidirectional sync

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true, // enabled
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
	}.Build())

	// 13. Now adding a member should work

	_, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, accesslistv1.UpsertAccessListWithMembersRequest_builder{
		AccessList: conv.ToProto(accessList),
		Members:    []*accesslistv1.Member{conv.ToMemberProto(member1), conv.ToMemberProto(member2)},
	}.Build())
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

	app := fakeOkta.CreateBasicApp("read-only-pulls-app")

	// Assign only user1.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, ghostUser.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(app.Id, ghostUser.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)
	accessListWatcher := sut.NewResourceWatcher(t, types.KindAccessList)
	memberWatcher := sut.NewResourceWatcher(t, types.KindAccessListMember)
	pluginWatcher := sut.NewResourceWatcher(t, types.KindPlugin)

	// 1. Create the integration with bidirectional sync disabled

	beforeCreationTime := time.Now()

	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	waitForUserSync(t, pluginWatcher, beforeCreationTime)
	waitForAccessListSync(t, pluginWatcher, beforeCreationTime)
	waitForOktaSync(t, sut, withTimePoint(beforeCreationTime))

	// 2. Verify users (user1 - ghost) are synchronized

	waitForResourceCount(t, userWatcher, 1, func(u types.User) bool {
		return u.Origin() == types.OriginOkta
	})

	// 3. Remember the name of the AL

	accessLists := waitForResourceCount(t, accessListWatcher, 1, func(*accesslist.AccessList) bool {
		return true
	})
	accessList := accessLists[0]
	require.NotEmpty(t, accessList.Spec.Title)
	require.Equal(t, app.Label, accessList.Spec.Title)

	// 4. Verify members (user1 - ghost)

	member1 := waitForResource(t, memberWatcher, func(m *accesslist.AccessListMember) bool {
		return m.Spec.AccessList == accessList.GetName()
	})
	require.Equal(t, ghostEmail, member1.GetName())

	// 5. Assign user2 (specter) to the SAML app for user sync and the app on the Okta side

	beforeAssignmentTimePoint := time.Now()
	err = fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, specterUser.Id)
	require.NoError(t, err)
	err = fakeOkta.AssignUserToApplication(app.Id, specterUser.Id)
	require.NoError(t, err)

	// 6. Verify user2 (specter) is synchronized to the Access List
	waitForAccessListSync(t, pluginWatcher, beforeAssignmentTimePoint)
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimePoint(beforeAssignmentTimePoint))

	waitForResource(t, memberWatcher, func(m *accesslist.AccessListMember) bool {
		return m.Spec.AccessList == accessList.GetName() && m.GetName() == specterEmail
	})
	members := mustListAccessListMembers(t, sut, accessList.GetName())
	require.Len(t, members, 2, "members = %v", members)
	for _, m := range members {
		require.True(t, m.GetName() == ghostEmail || m.GetName() == specterEmail, "member name = %q", m.GetName())
	}
}
