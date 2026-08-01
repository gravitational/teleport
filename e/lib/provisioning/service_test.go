package provisioning

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	scimconv "github.com/gravitational/teleport/e/lib/scim/conv"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/utils"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestDownstreamProvisioning(t *testing.T) {
	t.Parallel()

	// All of the subtests in this test are expected to run as a sequence. They
	// will fail if run independently or out of order.

	ctx := context.Background()
	pack := newPack(t)

	const (
		aliceUser         = "alice"
		bobUser           = "bob"
		aclID             = "test-access-list-1"
		aclTitle          = "Test Access List 1 Title"
		knownExternalUser = "external-user"
	)

	// Add user to the downstream system  that is known not to Teleport. This will be used for testing group
	// membership calculation later
	pack.scimMock.Users[knownExternalUser] = &scimsdk.User{
		ID:          uuid.New().String(),
		ExternalID:  knownExternalUser,
		UserName:    knownExternalUser,
		DisplayName: knownExternalUser,
	}

	t.Run("should provision teleport user to scim downstream", func(t *testing.T) {
		alice := pack.mustCreateTeleportUser(t, aliceUser)
		assertSCIMUserExists(t, pack.scimMock, aliceUser,
			withUserRevision(alice))

		bob := pack.mustCreateTeleportUser(t, bobUser)
		assertSCIMUserExists(t, pack.scimMock, bobUser,
			withUserRevision(bob))
	})

	t.Run("should provision access list to scim downstream", func(t *testing.T) {
		// WHEN I create an Access List containing a members
		//  a. known to only to the downstream system and marked in Teleport via
		//     membership origin == `OriginAWSIdentityCenter`
		//  b. known to both Teleport and the downstream system, and
		//  c. known to neither Teleport or the downstream system
		pack.mustCreateAccessList(t, aclID, aclTitle)
		pack.mustUpsertAccessListMember(t, aclID, aliceUser, accesslist.MembershipKindUser)
		pack.mustUpsertAccessListMemberWithNonExistentUserAccount(t, aclID, knownExternalUser, common.OriginAWSIdentityCenter)
		pack.mustUpsertAccessListMemberWithNonExistentUserAccount(t, aclID, "external-user2", "")
		pack.mustUpsertAccessListMember(t, aclID, bobUser, accesslist.MembershipKindUser)
		acl := pack.mustGetAccessList(t, aclID)

		// EXPECT that the Access List is provisioned downstream as a SCIM group
		// containing all members from categories (a) and (b), and none from
		// category (c) (i.e. external-user2), which is missing an identity center origin label is excluded.
		assertSCIMGroupExists(t, pack.scimMock, aclTitle,
			withMembers(aliceUser, bobUser, knownExternalUser),
			withAccessListRevision(acl))
	})

	t.Run("should de-provision scim group membership for member deleted from access list", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, aclID, aliceUser))
		assertSCIMGroupExists(t, pack.scimMock, aclTitle, withMemberCount(2))
	})

	t.Run("should de-provision scim group", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessList(ctx, aclID))
		assertSCIMGroupDoesntExist(t, pack.scimMock, aclTitle)
	})

	t.Run("should de-provision scim user", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteUser(ctx, aliceUser))
		assertSCIMUserDoesntExist(t, pack.scimMock, aliceUser)
	})
}

func TestAccessListPredicate(t *testing.T) {
	t.Parallel()

	predicate := func(_ context.Context, acl *accesslist.AccessList) (bool, error) {
		return slices.Contains(acl.Spec.Grants.Traits["provision"], "true"), nil
	}

	pack := newPack(t, withAccessListPredicate(predicate))

	const aliceUser = "alice"
	pack.mustCreateTeleportUser(t, aliceUser)

	t.Run("only matching access lists are provisioned", func(t *testing.T) {
		const (
			aclIncludedID    = "provisioning-access-list-included"
			aclIncludedTitle = "Provisioning Included Access List"

			aclExcludedID    = "provisioning-access-list-excluded"
			aclExcludedTitle = "Provisioning Test Excluded Access List Title"
		)

		// Given an Access List that matches the ACL predicate
		acl := pack.mustCreateAccessListWithCleanup(t, aclIncludedID, aclIncludedTitle)
		acl.Spec.Grants.Traits = map[string][]string{"provision": {"true"}}
		pack.mustUpsertAccessList(t, acl)
		pack.mustUpsertAccessListMember(t, aclIncludedID, aliceUser, accesslist.MembershipKindUser)

		// Given another Access List that does NOT match the ACL predicate
		pack.mustCreateAccessListWithCleanup(t, aclExcludedID, aclExcludedTitle)
		pack.mustUpsertAccessListMember(t, aclIncludedID, aliceUser, accesslist.MembershipKindUser)

		// Expect that only the matching access list is provisioned
		assertSCIMGroupExists(t, pack.scimMock, aclIncludedTitle, withMemberCount(1))
		assertSCIMGroupDoesntExist(t, pack.scimMock, aclExcludedTitle)
	})

	t.Run("matching access lists are deprovisioned when the no longer match", func(t *testing.T) {
		const (
			aclIncludedID    = "deprovisioning-access-list-included"
			aclIncludedTitle = "De-provisioning Test Included Access List"
		)

		// Given an Access List that matches the ACL predicate
		acl := pack.mustCreateAccessListWithCleanup(t, aclIncludedID, aclIncludedTitle)
		acl.Spec.Grants.Traits = map[string][]string{"provision": {"true"}}
		pack.mustUpsertAccessList(t, acl)
		pack.mustUpsertAccessListMember(t, aclIncludedID, aliceUser, accesslist.MembershipKindUser)

		assertSCIMGroupExists(t, pack.scimMock, aclIncludedTitle, withMemberCount(1))

		// WHEN I update the access list so that it no longer matches the
		// predicate
		acl.Spec.Grants.Traits["provision"][0] = "not on your life"
		pack.mustUpsertAccessList(t, acl)

		// EXPECT that the downstream group is deleted
		assertSCIMGroupDoesntExist(t, pack.scimMock, aclIncludedTitle)
	})
}

func TestAccessListProvisioningEvents(t *testing.T) {
	t.Parallel()

	const (
		aliceUser      = "alice"
		allowedACLID   = "allowed"
		forbiddenACLID = "forbidden"
	)
	aclIDs := []string{allowedACLID, forbiddenACLID}

	// GIVEN A Provisioning system set up to record `OnPrincipalProvisioning`
	// and `OnPrincipalProvisioned` events, and to block the provisioning of a
	// "forbidden" Access List
	var provisioningCalls callMap
	recordPrincipalProvisioning := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST {
			provisioningCalls.logCall(p.GetSpec().GetPrincipalId())
			if p.GetSpec().GetPrincipalId() == forbiddenACLID {
				return trace.Wrap(ErrDoNotProvision)
			}
		}
		return nil
	}

	var provisionedCalls callMap
	recordProvisionedACL := func(_ context.Context, p *provisioningv1.PrincipalState) {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST {
			provisionedCalls.logCall(p.GetSpec().GetPrincipalId())
		}
	}
	pack := newPack(t,
		withOnProvisioningCallback(recordPrincipalProvisioning),
		withOnProvisionedCallback(recordProvisionedACL))

	// Given a nominal Teleport User...
	pack.mustCreateTeleportUser(t, aliceUser)

	// WHEN I create some Access Lists...
	for _, aclID := range aclIDs {
		_ = pack.mustCreateAccessListWithCleanup(t, aclID, "Access List: "+aclID)
		pack.mustUpsertAccessListMember(t, aclID, aliceUser, accesslist.MembershipKindUser)
	}

	// EXPECT that the "allowed" group was provisioned and populated downstream
	assertSCIMGroupExists(t, pack.scimMock, "Access List: allowed",
		withMembers(aliceUser))

	// EXPECT that the OnPrincipalProvisioning event callback was invoked on the
	// Access List at least once on all access lists
	for _, aclID := range aclIDs {
		eventualWithT(t, calledAtLeastNTimes(&provisioningCalls, aclID, 1))
		eventualWithT(t, calledAtLeastNTimes(&provisioningCalls, aclID, 1))
	}

	// EXPECT that the OnPrincipalProvisioned event callback is eventually called
	// on the ACL not blocked by the event callback
	eventualWithT(t, calledAtLeastNTimes(&provisionedCalls, allowedACLID, 1))

	// EXPECT that the OnPrincipalProvisioned event callback is never called
	// on the ACL blocked by the event callback
	require.Zero(t, provisionedCalls.count(forbiddenACLID),
		"Provisioned callback should not be invoked on forbidden ACL")

	// EXPECT that the "forbidden" group was not provisioned downstream
	assertSCIMGroupDoesntExist(t, pack.scimMock, "Access List: forbidden")

	// EXPECT that the "forbidden" Access list is still marked as stale
	forbiddenState := pack.getAccessListProvisioningState(t, forbiddenACLID)
	require.Equal(t,
		provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
		forbiddenState.GetStatus().GetProvisioningState())
}

func calledAtLeastNTimes(calls *callMap, id string, minHitCount int) func(*assert.CollectT) {
	return func(collect *assert.CollectT) {
		assert.GreaterOrEqual(collect, calls.count(id), minHitCount)
	}
}

func TestAccessListDeprovisioningEvents(t *testing.T) {
	t.Parallel()

	const (
		aclSurvivorID = "surviving-test-acl"
		testLabel     = "test-label"
	)
	users := []string{"alice", "bob", "carol", "dave"}

	accessLists := map[string]string{
		"doomed-acl-1": "Doomed Access List",
		aclSurvivorID:  "Surviving Access List",
		"doomed-acl-2": "Another Doomed Access List",
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// GIVEN A Provisioning system rigged to record OnPrincipalDeprovisioning
	// callbacks, which will also suppress the de-provisioning of the "survivor"
	// Access List...
	var onDeprovisioningCallbacks callMap
	onPrincipalDeprovisioning := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() != provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST {
			return nil
		}
		principalID := p.GetSpec().GetPrincipalId()
		onDeprovisioningCallbacks.logCall(principalID)
		if principalID == aclSurvivorID {
			return ErrDoNotProvision
		}
		return nil
	}

	pack := newPack(t,
		withOnDeprovisioningCallback(onPrincipalDeprovisioning))

	// GIVEN a some Teleport users and Access Lists...
	for _, u := range users {
		pack.mustCreateTeleportUser(t, u)
	}
	for aclID, title := range accessLists {
		pack.mustCreateAccessList(t, aclID, title)
		for _, u := range users {
			pack.mustUpsertAccessListMember(t, aclID, u, accesslist.MembershipKindUser)
		}
	}

	// GIVEN that the user and access lists have all been provisioned downstream
	for _, u := range users {
		assertSCIMUserExists(t, pack.scimMock, u, withActiveState(true))
	}
	for _, aclTitle := range accessLists {
		assertSCIMGroupExists(t, pack.scimMock, aclTitle, withMembers(users...))
	}

	// WHEN I delete the Access Lists
	for aclID := range accessLists {
		require.NoError(t, pack.depsMock.DeleteAccessList(t.Context(), aclID))
	}

	// EXPECT that the OnPrincipalDeprovisioning callback was invoked for all
	// access lists
	expectedCalls := make(map[string]int, len(accessLists))
	for acl := range accessLists {
		expectedCalls[acl] = 1
	}
	eventualWithT(t,
		func(collect *assert.CollectT) {
			require.Equal(collect, expectedCalls, onDeprovisioningCallbacks.unwrap())
		})

	// EXPECT that all downstream groups have been de-provisioned, except the
	// designated survivor group whose de-provisioning we suppressed in the event
	// callback
	for aclID, title := range accessLists {
		if aclID == aclSurvivorID {
			// We are explicitly NOT testing the downstream group membership here,
			// as the group member list is indeterminate. For the purposes of this
			// test we are only stopping the downstream group being de-provisioned.
			// If we have already re-provisioned the group in response to the Access
			// List Membership deletion events caused by the Access List deletion,
			// then the groups will have fewer member than we expect.
			//
			// You can prevent this situation by suppressing re-provisioning the
			// group via the `onPrincipalProvisioning`, but thats out of scope
			// for this test
			assertSCIMGroupExists(t, pack.scimMock, title)
		} else {
			assertSCIMGroupDoesntExist(t, pack.scimMock, title)
		}
	}

	// EXPECT that all provisioning state records for all access lists have been
	// deleted from Teleport
	eventualWithT(t,
		func(collect *assert.CollectT) {
			for aclID := range accessLists {
				_, err := pack.depsMock.GetProvisioningState(t.Context(), pack.downstreamID, getIDForAccessListName(aclID))
				assert.True(collect, trace.IsNotFound(err), "Expected provisioning state for Access List %s to be deleted", aclID)
			}
		})
}

func TestAccessListProvisioningNested(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pack := newPack(t)

	const (
		aliceUser = "alice"
		bobUser   = "bob"
		carolUser = "carol"

		parentACLID    = "parent-acl"
		parentACLTitle = "Parent ACL title"

		childACLID    = "child-acl"
		childACLTitle = "Child ACL title"
	)

	t.Run("should provision nested access list members to scim downstream", func(t *testing.T) {
		// Given an Access List `Parent` containing "alice"
		pack.mustCreateTeleportUser(t, aliceUser)
		pack.mustCreateAccessList(t, parentACLID, parentACLTitle)
		pack.mustUpsertAccessListMember(t, parentACLID, aliceUser, accesslist.MembershipKindUser)

		// Given a second Access List `Child` containing "bob" and "carol"
		pack.mustCreateAccessList(t, childACLID, childACLTitle)
		pack.mustCreateTeleportUser(t, bobUser)
		pack.mustUpsertAccessListMember(t, childACLID, bobUser, accesslist.MembershipKindUser)
		pack.mustCreateTeleportUser(t, carolUser)
		pack.mustUpsertAccessListMember(t, childACLID, carolUser, accesslist.MembershipKindUser)

		// When I make the `Child` Access List a member of `Parent`
		pack.mustUpsertAccessListMember(t, parentACLID, childACLID, accesslist.MembershipKindList)

		assertSCIMGroupExists(t, pack.scimMock, childACLTitle, withMemberCount(2))

		// Expect that the membership of the downstream group `Parent` is
		// expanded to include the two users from `Child`
		assertSCIMGroupExists(t, pack.scimMock, parentACLTitle, withMemberCount(3))
	})

	t.Run("should re-provision access list when child list is updated", func(t *testing.T) {
		// Given the `Parent` and `Child` Access Lists created above,
		// When I remove `Bob` from the `Child` Access List...
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, childACLID, bobUser))

		// Expect that the membership of the downstream group `Parent` contracts
		// to reflect that Bob is no longer inherits membership of `Parent`
		assertSCIMGroupExists(t, pack.scimMock, parentACLTitle, withMemberCount(2))
	})

	t.Run("should re-provision access list when member list deleted", func(t *testing.T) {
		// Given the `Parent` and `Child` Access Lists created above,
		// When I revoke `Child`'s membership of `Parent`...
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, parentACLID, childACLID))

		// Expect that the membership of the downstream group `Parent`contracts
		// to just  the one member of `Parent`
		assertSCIMGroupExists(t, pack.scimMock, parentACLTitle, withMemberCount(1))
	})
}

func TestUserProvisioningActivationDeactivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pack := newPack(t)

	const (
		aliceUser = "alice"
		lockName  = "lock-1"
	)

	pack.mustCreateTeleportUser(t, aliceUser)
	assertSCIMUserExists(t, pack.scimMock, aliceUser, withActiveState(true))

	t.Run("should de-activate scim user in downstream when a user is locked in teleport", func(t *testing.T) {
		l := &types.LockV2{
			Metadata: types.Metadata{Name: lockName},
			Spec:     types.LockSpecV2{Target: types.LockTarget{User: aliceUser}},
		}
		require.NoError(t, pack.depsMock.UpsertLock(ctx, l))
		assertSCIMUserExists(t, pack.scimMock, aliceUser, withActiveState(false))
	})

	t.Run("should re-activate scim user in downstream when teleport user lock is deleted", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteLock(ctx, lockName))
		assertSCIMUserExists(t, pack.scimMock, aliceUser, withActiveState(true))
	})
}

func TestUserProvisioningPredicate(t *testing.T) {
	t.Parallel()

	const (
		aliceUser = "alice"
		bobUser   = "bob"
	)
	oktaOrigin := map[string]string{common.OriginLabel: types.OriginOkta}
	userFilters := []*types.AWSICUserSyncFilter{
		{Labels: map[string]string{common.OriginLabel: "unknown"}},
		// filters are OR-ed if any filter matches the user is included.
		{Labels: oktaOrigin},
	}

	// GIVEN a test provisioning system that only provisions users with the Okta
	// origin label...
	pack := newPack(t,
		withUserPredicate(identitycentercommon.UserPredicateFilter(userFilters)),
	)

	// WHEN I create two Teleport users, one with the target Origin label and one
	// without
	pack.mustCreateTeleportUser(t, aliceUser)
	pack.mustCreateTeleportUser(t, bobUser, withUserLabels(oktaOrigin))

	// EXPECT that the user of interest is created in the downstream SCIM server,
	// AND that the other user is not
	assertSCIMUserExists(t, pack.scimMock, bobUser, withActiveState(true))
	assertSCIMUserDoesntExist(t, pack.scimMock, aliceUser)
}

func TestUserProvisioningEvents(t *testing.T) {
	t.Parallel()

	allUsers := []string{"alice", "bob", "carol", "dave"}
	allowedUsers := allUsers[0:3]
	forbiddenUsers := allUsers[3:]

	var userProvisioningCalls callMap
	recordUserProvisioning := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
			pid := p.GetSpec().GetPrincipalId()
			userProvisioningCalls.logCall(pid)
			if !slices.Contains(allowedUsers, pid) {
				return trace.Wrap(ErrDoNotProvision)
			}
		}
		return nil
	}

	var userProvisionedCalls callMap
	recordUserProvisioned := func(_ context.Context, p *provisioningv1.PrincipalState) {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
			userProvisionedCalls.logCall(p.GetSpec().GetPrincipalId())
		}
	}

	// GIVEN a test provisioning system with provisioning event callbacks that
	// record principals they are invoked on.
	pack := newPack(t,
		withOnProvisioningCallback(recordUserProvisioning),
		withOnProvisionedCallback(recordUserProvisioned),
	)

	// WHEN I create some Teleport users...
	for _, username := range allUsers {
		pack.mustCreateTeleportUser(t, username)
	}

	// EXPECT that the `OnProvisioning` event callback is invoked on all users
	eventualWithT(t,
		func(collect *assert.CollectT) {
			assert.Equal(collect, len(allUsers), userProvisioningCalls.len())
			for _, u := range allUsers {
				assert.GreaterOrEqual(collect, userProvisioningCalls.count(u), 1,
					"OnProvisioning event must be invoked for user %q", u)
			}
		})

	// Expect that all users NOT suppressed by the OnProvisioning callback
	// exist in downstream system
	for _, u := range allowedUsers {
		assertSCIMUserExists(t, pack.scimMock, u, withActiveState(true))
	}
	for _, u := range forbiddenUsers {
		assertSCIMUserDoesntExist(t, pack.scimMock, u)
	}

	// EXPECT that the `OnProvisioned` event was invoked for each user EXCEPT for
	// "dave", who we suppressed with the event callback
	eventualWithT(t,
		func(collect *assert.CollectT) {
			assert.Equal(collect, len(allowedUsers), userProvisionedCalls.len())
			for _, u := range allowedUsers {
				assert.GreaterOrEqual(collect, userProvisionedCalls.count(u), 1,
					"OnProvisioned event must be invoked for user %q", u)
			}
		})

	// EXPECT that the forbidden users provisioning state is still marked as STALE
	for _, u := range forbiddenUsers {
		s := pack.getUserProvisioningState(t, u)
		require.Equal(t,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
			s.GetStatus().GetProvisioningState())
	}
}

func TestUserDeprovisioningEvents(t *testing.T) {
	t.Parallel()

	users := []string{"alice", "bob", "carol"}
	designatedSurvivor := "bob"

	var eventCalls callMap
	onPrincipalDeprovisioning := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
			principalID := p.GetSpec().GetPrincipalId()
			eventCalls.logCall(principalID)
			if principalID == designatedSurvivor {
				return ErrDoNotProvision
			}
		}
		return nil
	}

	// GIVEN a test provisioning system with several Teleport users provisioned
	// into the downstream system
	pack := newPack(t, withOnDeprovisioningCallback(onPrincipalDeprovisioning))
	for _, username := range users {
		pack.mustCreateTeleportUser(t, username)
	}
	for _, user := range users {
		assertSCIMUserExists(t, pack.scimMock, user, withActiveState(true))
	}

	// WHEN I delete the teleport users...
	for _, username := range users {
		pack.depsMock.DeleteUser(context.Background(), username)
	}

	// EXPECT that the de-provisioning callback is eventually invoked on all
	// users
	eventualWithT(t,
		func(collect *assert.CollectT) {
			assert.Equal(collect, len(users), eventCalls.len())
			for _, u := range users {
				assert.Equal(collect, 1, eventCalls.count(u), "Expected exactly one even callback on user %q", u)
			}
		})

	// EXPECT that all users *except* the designated surviving user have been
	// deprovisioned from the downstream system
	for _, u := range users {
		if u != designatedSurvivor {
			assertSCIMUserDoesntExist(t, pack.scimMock, u)
		}
	}
	assertSCIMUserExists(t, pack.scimMock, designatedSurvivor, withActiveState(true))

	// EXPECT that all provisioning records are deleted from Teleport
	eventualWithT(t,
		func(collect *assert.CollectT) {
			assert.Empty(collect, pack.getProvisioningStates(t))
		})
}

func TestMissingDownstreamPrincipal(t *testing.T) {
	t.Parallel()

	pack := newPack(t)

	t.Run("User", func(t *testing.T) {
		t.Parallel()

		t.Run("Deleted", func(t *testing.T) {
			t.Parallel()
			watcher := mustCreateWatcher(t, pack.depsMock.Events, types.KindProvisioningPrincipalState)

			// GIVEN a Teleport User that has been provisioned to the downstream system
			// and been given a downstream-supplied External ID
			pack.mustCreateTeleportUser(t, "alice")
			initialState := waitForPrincipalState(t, watcher,
				withProvisioningStateID(GetIDForUserName("alice")),
				not(withEmptyExternalID))
			initialUser, err := pack.scimMock.GetUserByUserName(t.Context(), "alice")
			require.NoError(t, err, "Test user must have been created in downstream system")
			require.True(t, initialUser.Active, "Test user must be active in downstream system")

			initialExternalID := initialState.GetStatus().GetExternalId()

			// WHEN a user is manually deleted from the downstream SCIM system
			// (simulating external deletion outside of Teleport's control)
			pack.scimMock.DeleteUser(t.Context(), initialExternalID)

			// AND an update is triggered for the user in Teleport (e.g., by modifying a field)
			pack.mustUpdateTeleportUser(t, "alice", func(u types.User) {
				u.SetStaticLabels(map[string]string{"test": "label"})
			})

			// EXPECT that the user is automatically re-provisioned to the downstream
			// system and has a new external ID recorded in their Teleport provisioning
			// record.
			waitForPrincipalState(t, watcher,
				withProvisioningStateID(GetIDForUserName("alice")),
				not(withEmptyExternalID),
				not(withExternalID(initialExternalID)))
			recreatedUser, err := pack.scimMock.GetUserByUserName(t.Context(), "alice")
			require.NoError(t, err, "Test user must have been re-created in downstream system")
			require.True(t, recreatedUser.Active, "Test user must be active in downstream system")
		})

		t.Run("Recreated", func(t *testing.T) {
			t.Parallel()
			watcher := mustCreateWatcher(t, pack.depsMock.Events, types.KindProvisioningPrincipalState)

			// GIVEN a Teleport user that has been provisioned to the downstream system
			// and been given a downstream-supplied External ID
			pack.mustCreateTeleportUser(t, "bob")
			initialState := waitForPrincipalState(t, watcher,
				withProvisioningStateID(GetIDForUserName("bob")),
				not(withEmptyExternalID))
			initialUser, err := pack.scimMock.GetUserByUserName(t.Context(), "bob")
			require.NoError(t, err, "Test user must have been created in downstream system")
			require.True(t, initialUser.Active, "Test user must be active in downstream system")
			initialExternalID := initialState.GetStatus().GetExternalId()

			// WHEN the downstream user is assigned a new ID in order to simulate a
			// user being deleted and recreated on the downstream system, outside of
			// Teleport's control
			const newExternalID = "recreated-bob"
			pack.scimMock.Mu.Lock()
			bob := pack.scimMock.Users[initialExternalID]
			bob.ID = newExternalID
			pack.scimMock.Users[newExternalID] = bob
			delete(pack.scimMock.Users, initialExternalID)
			pack.scimMock.Mu.Unlock()

			// AND an update is triggered for the user in Teleport (e.g., by modifying a field)
			pack.mustUpdateTeleportUser(t, "bob", func(u types.User) {
				u.SetStaticLabels(map[string]string{"test": "label"})
			})

			// EXPECT that the provisioning system has re-adopted user under their
			// new ID
			waitForPrincipalState(t, watcher,
				withProvisioningStateID(GetIDForUserName("bob")),
				not(withEmptyExternalID),
				withExternalID(newExternalID))
		})
	})

	t.Run("Group", func(t *testing.T) {
		t.Parallel()

		userNames := []string{"carol", "dave", "erica", "fred"}

		for _, name := range userNames {
			pack.mustCreateTeleportUser(t, name)
		}

		for _, name := range userNames {
			assertSCIMUserExists(t, pack.scimMock, name, withActiveState(true))
		}

		t.Run("Deleted", func(t *testing.T) {
			const (
				aclID    = "delete-test-access-list"
				aclTitle = "Delete test access list"
			)
			t.Parallel()
			watcher := mustCreateWatcher(t, pack.depsMock.Events, types.KindProvisioningPrincipalState)

			// GIVEN a Teleport Access List that has been provisioned to the downstream
			// system as a SCIM group, and had it's external ID recorded in Teleport
			pack.mustCreateAccessList(t, aclID, aclTitle)
			for _, name := range userNames {
				pack.mustUpsertAccessListMember(t, aclID, name, accesslist.MembershipKindUser)
			}

			initialState := waitForPrincipalState(t, watcher,
				withProvisioningStateID(getIDForAccessListName(aclID)),
				not(withEmptyExternalID),
				withProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
			)
			initialExternalID := initialState.GetStatus().GetExternalId()

			// WHEN I manually delete the downstream SCIM group to simulate
			// a Group being deleted outside of Teleport's control
			pack.scimMock.Mu.Lock()
			delete(pack.scimMock.Groups, initialExternalID)
			pack.scimMock.Mu.Unlock()

			// AND an update is triggered for the Access List in Teleport that
			// will force a provisioning event
			pack.mustDeleteAccessListMember(t, aclID, userNames[0])

			// EXPECT that the downstream group is automatically re-provisioned into the
			// downstream system with an updated member list, and has its new External ID
			// recorded by Teleport
			waitForPrincipalState(t, watcher,
				withProvisioningStateID(getIDForAccessListName(aclID)),
				not(withEmptyExternalID),
				not(withExternalID(initialExternalID)),
				withProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
			)
			assertSCIMGroupExists(t, pack.scimMock, aclTitle, withMembers(userNames[1:]...))
		})

		t.Run("Recreated", func(t *testing.T) {
			const (
				aclID    = "recreate-test-access-list"
				aclTitle = "Recreate test access list"
			)
			t.Parallel()
			watcher := mustCreateWatcher(t, pack.depsMock.Events, types.KindProvisioningPrincipalState)

			// GIVEN a Teleport Access List that has been provisioned to the downstream
			// system as a SCIM group, and had it's external ID recorded in Teleport
			pack.mustCreateAccessList(t, aclID, aclTitle)
			for _, name := range userNames {
				pack.mustUpsertAccessListMember(t, aclID, name, accesslist.MembershipKindUser)
			}
			initialState := waitForPrincipalState(t, watcher,
				withProvisioningStateID(getIDForAccessListName(aclID)),
				not(withEmptyExternalID),
				withProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
			)
			initialExternalID := initialState.GetStatus().GetExternalId()
			assertSCIMGroupExists(t, pack.scimMock, aclTitle, withMembers(userNames...))

			// WHEN I manually assign a new ID to the downstream group to simulate
			// a group being deleted and re-created outside of Teleport's
			// control
			const newExternalID = "recreated-group-id"
			pack.scimMock.Mu.Lock()
			g := pack.scimMock.Groups[initialExternalID]
			g.ID = newExternalID
			pack.scimMock.Groups[newExternalID] = g
			delete(pack.scimMock.Groups, initialExternalID)
			pack.scimMock.Mu.Unlock()

			// AND an update is triggered for the Access List in Teleport that
			// will force a provisioning event
			pack.mustDeleteAccessListMember(t, aclID, userNames[0])

			// EXPECT that the downstream group is automatically re-adopted by
			// the Teleport SCIM provisioner
			waitForPrincipalState(t, watcher,
				withProvisioningStateID(getIDForAccessListName(aclID)),
				withExternalID(newExternalID),
				withProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
			)
		})
	})
}

func eventualWithT(t *testing.T, condition func(collect *assert.CollectT)) {
	const (
		defaultWaitFor = time.Second * 2
		defaultTick    = time.Millisecond * 50
	)
	require.EventuallyWithT(t, condition, defaultWaitFor, defaultTick)
}

type scimGroupAssertion func(assert.TestingT, *scimsdk.Group) bool

func withAccessListRevision(acl *accesslist.AccessList) scimGroupAssertion {
	expected := scimconv.VersionAsETag(acl.GetRevision())
	return func(t assert.TestingT, g *scimsdk.Group) bool {
		return assert.NotNil(t, g.Meta, "SCIM Group missing metadata") &&
			assert.Equal(t, expected, g.Meta.Version,
				"SCIM Group version must match Teleport Access List revision")
	}
}

func withMembers(expected ...string) scimGroupAssertion {
	return func(t assert.TestingT, g *scimsdk.Group) bool {
		actual := sliceutils.Map(g.Members, (*scimsdk.GroupMember).GetDisplay)
		return assert.ElementsMatch(t, expected, actual,
			"Group member lists must match")
	}
}

func withMemberCount(n int) scimGroupAssertion {
	return func(t assert.TestingT, g *scimsdk.Group) bool {
		return assert.Len(t, g.Members, n, "Group must have exactly %d members", n)
	}
}

func assertSCIMGroupExists(t *testing.T, client scimsdk.Client, groupDisplayName string, assertions ...scimGroupAssertion) {
	eventualWithT(t, func(collect *assert.CollectT) {
		g, err := client.GetGroupByDisplayName(context.Background(), groupDisplayName)
		if !assert.NoError(collect, err) {
			return
		}
		for _, assertion := range assertions {
			// Run all the assertions to collect as much information as possible
			// during test failures
			assertion(collect, g)
		}
	})
}

type scimUserAssertion func(assert.TestingT, *scimsdk.User) bool

func withUserRevision(user types.User) scimUserAssertion {
	expected := scimconv.VersionAsETag(user.GetRevision())
	return func(t assert.TestingT, user *scimsdk.User) bool {
		return assert.NotNil(t, user.Meta, "User resource missing metadata") &&
			assert.Equal(t, expected, user.Meta.Version, "Unexpected user revision")
	}
}

func withActiveState(expected bool) scimUserAssertion {
	return func(t assert.TestingT, user *scimsdk.User) bool {
		return assert.Equal(t, expected, user.Active, "Expected User to have Active state %v", expected)
	}
}

func assertSCIMUserExists(t *testing.T, client scimsdk.Client, userName string, assertions ...scimUserAssertion) {
	eventualWithT(t, func(collect *assert.CollectT) {
		user, err := client.GetUserByUserName(context.Background(), userName)
		if !assert.NoError(collect, err) {
			return
		}
		for _, assertion := range assertions {
			// run all assertions to collect as much information as possible
			// about any failures
			assertion(collect, user)
		}
	})
}

func assertSCIMUserDoesntExist(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetUserByUserName(context.Background(), userName)
		assert.True(collect, trace.IsNotFound(err))
	})
}

func assertSCIMGroupDoesntExist(t *testing.T, scimClient scimsdk.Client, displayName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := scimClient.GetGroupByDisplayName(context.Background(), displayName)
		assert.True(collect, trace.IsNotFound(err), "Downstream group %q should not exist", displayName)
	})
}

type callMap utils.SyncMap[string, int]

func (m *callMap) logCall(call string) {
	(*utils.SyncMap[string, int])(m).Write(
		func(innerMap map[string]int) {
			innerMap[call] = innerMap[call] + 1
		})
}

func (m *callMap) count(call string) int {
	if n, ok := (*utils.SyncMap[string, int])(m).Load(call); ok {
		return n
	}
	return 0
}

func (m *callMap) len() int {
	return (*utils.SyncMap[string, int])(m).Len()
}

func (m *callMap) unwrap() map[string]int {
	return (*utils.SyncMap[string, int])(m).Clone()
}
