package igs

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

// TestAccessListMemberRBAC tests the permissions of an access list owner.
// Access List Owner without role RBAC:
// - should not be able to modify their membership properties
// - should not be able to add themselves as a member
// - If an owner is also a member, they should be able to add a new member
func TestAccessListOwnerPermissions(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	})

	sut := common.InitSUT(t, common.WithUser(t, "alice", "requester"))

	testAccessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title: "access list 1",
		Owners: []accesslist.Owner{
			{Name: "alice", IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()},
		},
		Grants: accesslist.Grants{Roles: []string{"access"}},
		Audit:  accesslist.Audit{NextAuditDate: sut.Clock.Now()},
	})
	require.NoError(t, err)
	ctx := context.Background()

	aclMember := []*accesslist.AccessListMember{
		mustCreateMember(t, testAccessList.GetName(), "alice"),
	}

	authServer := sut.Teleport.Process.GetAuthServer()
	_, _, err = authServer.AccessLists.UpsertAccessListWithMembers(ctx, testAccessList, aclMember)
	require.NoError(t, err)

	aliceTC := sut.GetClusterClientForUser(t, "alice")
	aliceAccessListClient := aliceTC.AuthClient.AccessListClient()

	t.Run("owner should not be able to modify their membership properties", func(t *testing.T) {
		acl, members := mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())

		require.True(t, isAccessListOwner(acl, "alice"))
		require.True(t, isAccessListMember(members, "alice"))

		idx := slices.IndexFunc(members, func(v *accesslist.AccessListMember) bool {
			return v.Spec.Name == "alice"
		})
		require.NotEqual(t, idx, -1)
		members[idx].Spec.Expires = sut.Clock.Now().Add(time.Hour * 24 * 10)

		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("owner that is also a member should be able to add a new member", func(t *testing.T) {
		acl, members := mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())

		require.True(t, isAccessListOwner(acl, "alice"))
		require.True(t, isAccessListMember(members, "alice"))

		members = append(members, mustCreateMember(t, acl.GetName(), "bob"))
		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.NoError(t, err)
	})

	t.Run("owner should not be able to add themselves as member", func(t *testing.T) {
		err = aliceAccessListClient.DeleteAccessListMember(ctx, testAccessList.GetName(), "alice")
		require.NoError(t, err)

		var acl *accesslist.AccessList
		var members []*accesslist.AccessListMember
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			acl, members = mustGetAccessListAndMembers(t, aliceAccessListClient, testAccessList.GetName())
			assert.True(collect, isAccessListOwner(acl, "alice"))
			assert.False(collect, isAccessListMember(members, "alice"))
		}, time.Second, time.Millisecond*100)

		members = append(members, mustCreateMember(t, testAccessList.GetName(), "alice"))
		_, _, err = aliceAccessListClient.UpsertAccessListWithMembers(ctx, acl, members)
		require.True(t, trace.IsAccessDenied(err))
	})
}
