package accesslist

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
)

func TestNewIneligibleStatusReconciler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
			require.NoError(t, err)
			for _, member := range members {
				require.Equal(t, "INELIGIBLE_STATUS_ELIGIBLE", member.Spec.IneligibleStatus)
			}
		}, 5*time.Second, 100*time.Millisecond)

		// Update the access list member with non-existent account and without identity center origin label.
		externalMemberWithoutOrigin := newAccessListMember(t, a3.GetName(), externalMember2, accesslist.MembershipKindUser, c.clock)
		createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, nil,
			[]*accesslist.AccessList{}, []*accesslist.AccessListMember{externalMemberWithoutOrigin})

		synctest.Wait()
		members, _, err := c.testEnv.accessLists.ListAllAccessListMembers(c.userCtx, 100, "")
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

		// Wait for the reconciler to update the status.
		synctest.Wait()
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
		synctest.Wait()
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

		// Wait for the reconciler to update the status.
		synctest.Wait()
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

		synctest.Wait()
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
