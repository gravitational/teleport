package accesslist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestNewIneligibleStatusReconciler(t *testing.T) {
	c := initSvc(t)

	a1 := newAccessList(t, "1", c.clock)
	a2 := newAccessList(t, "2", c.clock)
	a3 := newAccessList(t, "3", c.clock)
	a4 := newAccessList(t, "4", c.clock)

	a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock)
	a1m3 := newAccessListMember(t, a1.GetName(), member3, accesslist.MembershipKindUser, c.clock)
	a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock)
	a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock)

	createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
		[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a1m3, a2m1, a3m1, a3m2})

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			assert.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
		}
	}, 5*time.Second, 100*time.Millisecond)

	// Update the access list member to be ineligible.
	user, err := c.testEnv.identity.GetUser(c.userCtx, member1, false /* withSecrets */)
	require.NoError(t, err)
	user.GetTraits()["mtrait1"] = nil
	_, err = c.testEnv.identity.UpdateUser(c.userCtx, user)
	require.NoError(t, err)

	// Wait for the reconciler to update the status.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Check that the access list member is now ineligible.
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.GetName() {
			case member1:
				assert.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", member.Spec.IneligibleStatus)
			default:
				assert.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}
	}, 5*time.Second, 100*time.Millisecond)

	// Update the access list member to be eligible.
	user, err = c.testEnv.identity.GetUser(c.userCtx, member1, false /* withSecrets */)
	require.NoError(t, err)
	user.GetTraits()["mtrait1"] = []string{"mvalue1", "mvalue2"}
	_, err = c.testEnv.identity.UpdateUser(c.userCtx, user)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			assert.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
		}
	}, 5*time.Second, 100*time.Millisecond)

	// Update access list to change the requirements.
	// All members of access list 1 should now be ineligible.
	a1.Spec.MembershipRequires.Traits["mtrait1"] = []string{"mvalue4", "mvalue3"}
	_, err = c.testEnv.accessLists.UpsertAccessList(c.userCtx, a1)
	require.NoError(t, err)

	// Wait for the reconciler to update the status.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Check that the access list member is now ineligible.
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.Spec.AccessList {
			case a1.GetName():
				assert.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", member.Spec.IneligibleStatus)
			default:
				assert.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}
	}, 5*time.Second, 100*time.Millisecond)

	// Update access list to change the requirements.
	// All members of access list 1 should now be eligible.
	a1.Spec.MembershipRequires.Traits["mtrait1"] = []string{"mvalue1", "mvalue2"}
	a1.Spec.OwnershipRequires.Traits["mtrait1"] = []string{"mvalue1"}
	_, err = c.testEnv.accessLists.UpsertAccessList(c.userCtx, a1)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		al, err := c.testEnv.accessLists.GetAccessList(c.userCtx, a1.GetName())
		require.NoError(t, err)
		assert.Len(t, al.Spec.Owners, 4)
		for _, owner := range al.Spec.Owners {
			assert.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", owner.IneligibleStatus)
		}
	}, 5*time.Second, 100*time.Millisecond)

	// Expire all access list members.
	c.clock.Advance(48 * time.Hour)
	// Wait for the reconciler to update the status.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			assert.Equal(t, "INELIGIBLE_STATUS_EXPIRED", member.Spec.IneligibleStatus)
		}
	}, 5*time.Second, 100*time.Millisecond)
}
