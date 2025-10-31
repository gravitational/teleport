package accesslist

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
)

func TestNewIneligibleStatusReconciler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		waitForBatchWindow := func() {
			time.Sleep(batchWindow)
			synctest.Wait()
		}
		clock := clockwork.NewRealClock()
		c := initSvc(t, withClock(clock))
		synctest.Wait()

		a1 := newAccessList(t, "1", c.clock)
		a2 := newAccessList(t, "2", c.clock)
		a3 := newAccessList(t, "3", c.clock)
		a4 := newAccessList(t, "4", c.clock)

		expireMemberAfter := 24 * time.Hour
		memberExpires := c.clock.Now().Add(expireMemberAfter)
		a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock, withExpire(memberExpires))
		a1m2 := newAccessListMember(t, a1.GetName(), member2, accesslist.MembershipKindUser, c.clock, withExpire(memberExpires))
		a1m3 := newAccessListMember(t, a1.GetName(), member3, accesslist.MembershipKindUser, c.clock, withExpire(memberExpires))
		a2m1 := newAccessListMember(t, a2.GetName(), member1, accesslist.MembershipKindUser, c.clock, withExpire(memberExpires))
		a3m1 := newAccessListMember(t, a3.GetName(), member1, accesslist.MembershipKindUser, c.clock, withExpire(memberExpires))
		// origin label OriginAWSIdentityCenter for member with existing account should not bypass checkUserIsStillEligible.
		a3m2 := newAccessListMember(t, a3.GetName(), member2, accesslist.MembershipKindUser, c.clock, withOriginLabel(common.OriginAWSIdentityCenter), withExpire(memberExpires))
		externalMemberWithIdentityCenterOrigin := newAccessListMember(t, a3.GetName(), externalMember1, accesslist.MembershipKindUser, c.clock, withOriginLabel(common.OriginAWSIdentityCenter), withExpire(memberExpires))

		createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
			[]*accesslist.AccessList{a1, a2, a3, a4}, []*accesslist.AccessListMember{a1m1, a1m2, a1m3, a2m1, a3m1, a3m2, externalMemberWithIdentityCenterOrigin})

		waitForBatchWindow()
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
		}

		// Update the access list member with non-existent account and without identity center origin label.
		externalMemberWithoutOrigin := newAccessListMember(t, a3.GetName(), externalMember2, accesslist.MembershipKindUser, c.clock)
		createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
			[]*accesslist.AccessList{}, []*accesslist.AccessListMember{externalMemberWithoutOrigin})

		waitForBatchWindow()
		members, _, err = c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.Spec.Name {
			case externalMemberWithoutOrigin.GetName():
				require.Equal(t, "INELIGIBLE_STATUS_USER_NOT_EXIST", member.Spec.IneligibleStatus)
			default:
				require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}
		c.svc.accessLists.DeleteAccessListMember(c.userCtx, a3.GetName(), externalMemberWithoutOrigin.GetName())

		// Update the access list member to be ineligible.
		user, err := c.testEnv.identity.GetUser(c.userCtx, member1, false /* withSecrets */)
		require.NoError(t, err)
		user.GetTraits()["mtrait1"] = nil
		_, err = c.testEnv.identity.UpdateUser(c.userCtx, user)
		require.NoError(t, err)

		waitForBatchWindow()
		// Check that the access list member is now ineligible.
		members, _, err = c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.GetName() {
			case member1:
				require.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", member.Spec.IneligibleStatus)
			default:
				require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}

		// Update the access list member to be eligible.
		user, err = c.testEnv.identity.GetUser(c.userCtx, member1, false /* withSecrets */)
		require.NoError(t, err)
		user.GetTraits()["mtrait1"] = []string{"mvalue1", "mvalue2"}
		_, err = c.testEnv.identity.UpdateUser(c.userCtx, user)
		require.NoError(t, err)

		waitForBatchWindow()
		members, _, err = c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
		}

		// Update access list to change the requirements.
		// All members of access list 1 should now be ineligible.
		a1.Spec.MembershipRequires.Traits["mtrait1"] = []string{"mvalue4", "mvalue3"}
		_, err = c.testEnv.accessLists.UpsertAccessList(c.userCtx, a1)
		require.NoError(t, err)

		waitForBatchWindow()
		// Check that the access list member is now ineligible.
		members, _, err = c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.Spec.AccessList {
			case a1.GetName():
				require.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", member.Spec.IneligibleStatus)
			default:
				require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}

		// Update access list to change the requirements.
		// All members of access list 1 should now be eligible.
		a1.Spec.MembershipRequires.Traits["mtrait1"] = []string{"mvalue1", "mvalue2"}
		a1.Spec.OwnershipRequires.Traits["mtrait1"] = []string{"mvalue1"}
		_, err = c.testEnv.accessLists.UpsertAccessList(c.userCtx, a1)
		require.NoError(t, err)

		waitForBatchWindow()
		al, err := c.testEnv.accessLists.GetAccessList(c.userCtx, a1.GetName())
		require.NoError(t, err)
		require.Len(t, al.Spec.Owners, 4)
		for _, owner := range al.Spec.Owners {
			require.Equal(t, "INELIGIBLE_STATUS_MISSING_REQUIREMENTS", owner.IneligibleStatus)
		}

		time.Sleep(expireMemberAfter + time.Nanosecond)
		synctest.Wait()
		members, _, err = c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
		require.NoError(t, err)
		for _, member := range members {
			switch member.Spec.Name {
			case externalMemberWithIdentityCenterOrigin.GetName():
				require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			default:
				require.Equal(t, "INELIGIBLE_STATUS_EXPIRED", member.Spec.IneligibleStatus)
			}
		}
	})
}

// TestIneligibleStatusReconcilerFlowForNotExistingUsers verifies that the reconciler
// correctly handles access list members and owners that reference non-existent users.
//
// This is important to prevent continues ineligibility status updates when dealing with external user
// references that may not exist in Teleport's identity system.
func TestIneligibleStatusReconcilerFlowForNotExistingUsers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clock := clockwork.NewRealClock()
		c := initSvc(t, withClock(clock))
		synctest.Wait()

		// Helper to trigger reconciliation by waiting for the force reconcile duration
		waitForReconciliation := func() {
			time.Sleep(forceReconcileDuration * 2)
			synctest.Wait()
		}

		const (
			accessListName  = "test-access-list"
			nonExistentUser = "not_existing_user"
		)

		// Create an access list with no eligibility requirements and a non-existent owner
		accessList := newAccessList(t, accessListName, c.clock)
		accessList.Spec.OwnershipRequires = accesslist.Requires{}
		accessList.Spec.MembershipRequires = accesslist.Requires{}
		accessList.Spec.Owners = []accesslist.Owner{{Name: nonExistentUser}}

		memberForNonExistentUser := newAccessListMember(t, accessList.GetName(), nonExistentUser, accesslist.MembershipKindUser, c.clock)

		_, _, err := c.testEnv.accessLists.UpsertAccessListWithMembers(
			t.Context(),
			accessList,
			[]*accesslist.AccessListMember{memberForNonExistentUser},
		)
		require.NoError(t, err)

		waitForReconciliation()

		accessList, err = c.testEnv.accessLists.GetAccessList(t.Context(), accessListName)
		require.NoError(t, err)
		initialAccessListRevision := accessList.GetRevision()

		member, err := c.testEnv.accessLists.GetAccessListMember(t.Context(), accessList.GetName(), nonExistentUser)
		require.NoError(t, err)
		initialMemberRevision := member.GetRevision()

		verifyStatusAndRevisionsUnchanged := func() {
			list, err := c.testEnv.accessLists.GetAccessList(t.Context(), accessListName)
			require.NoError(t, err)
			require.Len(t, list.Spec.Owners, 1, "should have exactly one owner")
			require.Equal(t, "INELIGIBLE_STATUS_UNSPECIFIED", list.Spec.Owners[0].IneligibleStatus,
				"owner referencing non-existent user should have UNSPECIFIED status")
			require.Equal(t, initialAccessListRevision, list.GetRevision(),
				"access list revision should not change on subsequent reconciliations")

			m, err := c.testEnv.accessLists.GetAccessListMember(t.Context(), list.GetName(), nonExistentUser)
			require.NoError(t, err)
			require.Equal(t, "INELIGIBLE_STATUS_USER_NOT_EXIST", m.Spec.IneligibleStatus,
				"member referencing non-existent user should be marked as USER_NOT_EXIST")
			require.Equal(t, initialMemberRevision, m.GetRevision(),
				"member revision should not change on subsequent reconciliations")
		}

		// Run multiple reconciliation cycles to ensure stability
		// This verifies that the reconciler doesn't continuously update resources
		// when the ineligible status is already correctly set
		const reconciliationCycles = 5
		for i := 0; i < reconciliationCycles; i++ {
			waitForReconciliation()
			verifyStatusAndRevisionsUnchanged()
		}
	})
}

// TestIneligibleStatusReconcilerBatcher triggers multiple changes within the batch window
// and test the StateOverloaded state where no reconciliation action taken till the events thought settle down.
func TestIneligibleStatusReconcilerBatcher(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clock := clockwork.NewRealClock()
		c := initSvc(t, withClock(clock))

		userService := c.testEnv.identity
		synctest.Wait()

		a1 := newAccessList(t, "1", c.clock)
		a1.Spec.MembershipRequires = accesslist.Requires{
			Traits: map[string][]string{"want_trait": {}},
		}
		a1m1 := newAccessListMember(t, a1.GetName(), member1, accesslist.MembershipKindUser, c.clock)

		createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
			[]*accesslist.AccessList{a1}, []*accesslist.AccessListMember{a1m1})

		assertMemberIllegalityStatus := func(wantStatus string) {
			m, err := c.testEnv.accessLists.GetAccessListMember(t.Context(), a1.GetName(), member1)
			require.NoError(t, err)
			require.Equal(t, wantStatus, m.Spec.IneligibleStatus)
		}

		emitUserChange := func(changeFn func(u types.User)) {
			u, err := userService.GetUser(t.Context(), member1, false)
			require.NoError(t, err)
			if changeFn != nil {
				changeFn(u)
			}
			_, err = userService.UpsertUser(t.Context(), u)
			require.NoError(t, err)
		}

		// Eligibility reconsider haven't run yet, status should be empty.
		assertMemberIllegalityStatus("")

		// Trigger an eligibility by string time exceeding batch window.
		// All changes to user within the batch window should be coalesced
		// and only one reconciliation should be performed
		time.Sleep(batchWindow)
		synctest.Wait()
		assertMemberIllegalityStatus("INELIGIBLE_STATUS_MISSING_REQUIREMENTS")

		// Triggers multiple changes to the user within the batch window.
		// No action should be taken yet since many changes triggers height load mode
		// that waits will events to settle down.
		for range maxBatchQueueSize {
			emitUserChange(func(u types.User) {
				u.SetTraits(nil)
			})
		}
		time.Sleep(batchWindow)
		synctest.Wait()
		assertMemberIllegalityStatus("INELIGIBLE_STATUS_MISSING_REQUIREMENTS")

		// The events thought settled down and is below the HighLoadThreshold.
		// This should trigger the eligibility reconciliation.
		for range maxBatchQueueSize / 3 {
			emitUserChange(func(u types.User) {
				u.SetTraits(map[string][]string{
					"want_trait": {"some_value"},
				})
			})
		}

		time.Sleep(batchWindow)
		synctest.Wait()
		assertMemberIllegalityStatus("INELIGIBLE_STATUS_ELIGIBLE")
	})
}
