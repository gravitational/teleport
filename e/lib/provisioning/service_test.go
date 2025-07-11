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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestMain(m *testing.M) {
	// Almost every test in this package requires the Access List feature, so
	// we enable it once here, as opposed to using [modules.SetTestModules()] in
	// every test, which would also force each test to run in series.

	modules.SetModules(&modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})
	os.Exit(m.Run())
}

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
		pack.mustCreateTeleportUser(t, aliceUser)
		assertSCIMUserExists(t, pack.scimMock, aliceUser)
		pack.mustCreateTeleportUser(t, bobUser)
		assertSCIMUserExists(t, pack.scimMock, bobUser)
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

		// EXPECT that the Access List is provisioned downstream as a SCIM group
		// containing all members from categories (a) and (b), and none from
		// category (c) (i.e. external-user2), which is missing an identity center origin label is excluded.
		assertSCIMGroupExistsWithMembers(t, pack.scimMock, aclTitle, aliceUser, bobUser, knownExternalUser)
	})

	t.Run("should de-provision scim group membership for member deleted from access list", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, aclID, aliceUser))
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, aclTitle, 2)
	})

	t.Run("should de-provision scim group", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessList(ctx, aclID))
		assertSCIMGroupDoestExist(t, pack.scimMock, aclTitle)
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
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, aclIncludedTitle, 1)
		assertSCIMGroupDoestExist(t, pack.scimMock, aclExcludedTitle)
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

		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, aclIncludedTitle, 1)

		// WHEN I update the access list so that it no longer matches the
		// predicate
		acl.Spec.Grants.Traits["provision"][0] = "no on your life"
		pack.mustUpsertAccessList(t, acl)

		// EXPECT that the downstream group is deleted
		assertSCIMGroupDoestExist(t, pack.scimMock, aclIncludedTitle)
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
	recordProvisionedACL := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST {
			provisionedCalls.logCall(p.GetSpec().GetPrincipalId())
		}
		return nil
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
	assertSCIMGroupExistsWithMembers(t, pack.scimMock, "Access List: allowed", aliceUser)

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
	assertSCIMGroupDoestExist(t, pack.scimMock, "Access List: forbidden")

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
		assertSCIMUserExistAndIsActive(t, pack.scimMock, u)
	}
	for _, aclTitle := range accessLists {
		assertSCIMGroupExistsWithMembers(t, pack.scimMock, aclTitle, users...)
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
			assertSCIMGroupDoestExist(t, pack.scimMock, title)
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

		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, childACLTitle, 2)

		// Expect that the membership of the downstream group `Parent` is
		// expanded to include the two users from `Child`
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, parentACLTitle, 3)
	})

	t.Run("should re-provision access list when child list is updated", func(t *testing.T) {
		// Given the `Parent` and `Child` Access Lists created above,
		// When I remove `Bob` from the `Child` Access List...
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, childACLID, bobUser))

		// Expect that the membership of the downstream group `Parent` contracts
		// to reflect that Bob is no longer inherits membership of `Parent`
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, parentACLTitle, 2)
	})

	t.Run("should re-provision access list when member list deleted", func(t *testing.T) {
		// Given the `Parent` and `Child` Access Lists created above,
		// When I revoke `Child`'s membership of `Parent`...
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, parentACLID, childACLID))

		// Expect that the membership of the downstream group `Parent`contracts
		// to just  the one member of `Parent`
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, parentACLTitle, 1)
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
	assertSCIMUserExistAndIsActive(t, pack.scimMock, aliceUser)

	t.Run("should de-activate scim user in downstream when a user is locked in teleport", func(t *testing.T) {
		l := &types.LockV2{
			Metadata: types.Metadata{Name: lockName},
			Spec:     types.LockSpecV2{Target: types.LockTarget{User: aliceUser}},
		}
		require.NoError(t, pack.depsMock.UpsertLock(ctx, l))
		assertSCIMUserExistAndIsNotActive(t, pack.scimMock, aliceUser)
	})

	t.Run("should re-activate scim user in downstream when teleport user lock is deleted", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteLock(ctx, lockName))
		assertSCIMUserExistAndIsActive(t, pack.scimMock, aliceUser)
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
	assertSCIMUserExistAndIsActive(t, pack.scimMock, bobUser)
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
	recordUserProvisioned := func(_ context.Context, p *provisioningv1.PrincipalState) error {
		if p.GetSpec().GetPrincipalType() == provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER {
			userProvisionedCalls.logCall(p.GetSpec().GetPrincipalId())
		}
		return nil
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
		assertSCIMUserExistAndIsActive(t, pack.scimMock, u)
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
		assertSCIMUserExistAndIsActive(t, pack.scimMock, user)
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
	assertSCIMUserExistAndIsActive(t, pack.scimMock, designatedSurvivor)

	// EXPECT that all provisioning records are deleted from Teleport
	eventualWithT(t,
		func(collect *assert.CollectT) {
			assert.Empty(collect, pack.getProvisioningStates(t))
		})
}

type mockDeps struct {
	*local.AccessService
	*local.IdentityService
	*local.AccessListService
	services.DownstreamProvisioningStates
	types.Events
}

func newDepsMock(t *testing.T, clock clockwork.Clock) *mockDeps {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	identitySvc, err := local.NewIdentityService(b)
	require.NoError(t, err)

	aclSvc, err := local.NewAccessListService(b, clock)
	require.NoError(t, err)

	provStateSvc, err := local.NewProvisioningStateService(b)
	require.NoError(t, err)

	return &mockDeps{
		AccessService:                local.NewAccessService(b),
		Events:                       local.NewEventsService(b),
		IdentityService:              identitySvc,
		AccessListService:            aclSvc,
		DownstreamProvisioningStates: provStateSvc,
	}
}

type testPack struct {
	depsMock     *mockDeps
	scimMock     *scimsdk.ClientMock
	clock        clocki.FakeClock
	downstreamID services.DownstreamID
}

type sutOptions struct {
	downstreamID        services.DownstreamID
	scimClient          *scimsdk.ClientMock
	accessListPredicate AccessListPredicate
	userPredicate       identitycentercommon.UserFilterFunc
	onProvisioning      EventHandler
	onProvisioned       EventHandler
	onDeprovisioning    EventHandler
}

type sutOption func(*sutOptions)

func withAccessListPredicate(p AccessListPredicate) sutOption {
	return func(opts *sutOptions) {
		opts.accessListPredicate = p
	}
}

func withUserPredicate(fn func(types.User) bool) sutOption {
	return func(opts *sutOptions) {
		opts.userPredicate = fn
	}
}

func withOnProvisioningCallback(fn EventHandler) sutOption {
	return func(opts *sutOptions) {
		opts.onProvisioning = fn
	}
}

func withOnProvisionedCallback(fn EventHandler) sutOption {
	return func(opts *sutOptions) {
		opts.onProvisioned = fn
	}
}

func withOnDeprovisioningCallback(fn EventHandler) sutOption {
	return func(opts *sutOptions) {
		opts.onDeprovisioning = fn
	}
}

func newPack(t *testing.T, options ...sutOption) *testPack {
	defaultOpts := &sutOptions{
		downstreamID: "test-downstream",
		scimClient:   scimsdk.NewSCIMClientMock(),
		accessListPredicate: func(context.Context, *accesslist.AccessList) (bool, error) {
			return true, nil
		},
		userPredicate: identitycentercommon.UserPredicateFilter(nil),
	}
	for _, opt := range options {
		opt(defaultOpts)
	}

	// We can only legally call testify assertions from the main test thread, so
	// set up a mechanism for us to marshal the service goroutine's exit code
	// back here for asserting that it exited cleanly.
	errCh := make(chan error)
	t.Cleanup(func() {
		// If we have already failed, then there's no point in waiting around
		if t.Failed() {
			return
		}

		select {
		case err := <-errCh:
			require.NoError(t, err, "Service shutdown")
		case <-time.After(10 * time.Second):
			require.Fail(t, "Test cleanup timed out")
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()

	depsMock := newDepsMock(t, clock)
	svc, err := NewService(ServiceConfig{
		SCIMClient:                defaultOpts.scimClient,
		UsersCache:                depsMock,
		AccessListsCache:          depsMock,
		Locks:                     depsMock,
		StateSvc:                  depsMock,
		StateSvcCache:             depsMock,
		EventsClient:              depsMock,
		Clock:                     clock,
		DownstreamID:              defaultOpts.downstreamID,
		AccessListPredicate:       defaultOpts.accessListPredicate,
		UserPredicate:             defaultOpts.userPredicate,
		OnPrincipalProvisioning:   defaultOpts.onProvisioning,
		OnPrincipalProvisioned:    defaultOpts.onProvisioned,
		OnPrincipalDeprovisioning: defaultOpts.onDeprovisioning,
	})
	require.NoError(t, err)

	go func() {
		// ensure we marshal the error value back to the main test goroutine for
		// assertion
		errCh <- svc.Run(ctx)
	}()

	return &testPack{
		downstreamID: defaultOpts.downstreamID,
		depsMock:     depsMock,
		scimMock:     defaultOpts.scimClient,
		clock:        clock,
	}
}

func eventualWithT(t *testing.T, condition func(collect *assert.CollectT)) {
	const (
		defaultWaitFor = time.Second * 2
		defaultTick    = time.Millisecond * 50
	)
	require.EventuallyWithT(t, condition, defaultWaitFor, defaultTick)
}

func assertSCIMGroupExists(t *testing.T, client scimsdk.Client, groupDisplayName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetGroupByDisplayName(context.Background(), groupDisplayName)
		assert.NoError(collect, err)
	})
}

func assertSCIMGroupExistsWithMembers(t *testing.T, client scimsdk.Client, groupDisplayName string, members ...string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		acl, err := client.GetGroupByDisplayName(context.Background(), groupDisplayName)
		if assert.NoError(collect, err) {
			var actualMembers []string
			for _, am := range acl.Members {
				actualMembers = append(actualMembers, am.Display)
			}
			assert.ElementsMatch(collect, members, actualMembers)
		}
	})
}

func assertSCIMGroupExitsWithMembersLength(t *testing.T, client scimsdk.Client, groupDisplayName string, wantMembersLength int) {
	eventualWithT(t, func(collect *assert.CollectT) {
		acl, err := client.GetGroupByDisplayName(context.Background(), groupDisplayName)
		if assert.NoError(collect, err) {
			assert.Len(collect, acl.Members, wantMembersLength,
				"Group %q expected to have %d members", groupDisplayName, wantMembersLength)
		}
	})
}

func assertSCIMUserExists(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetUserByUserName(context.Background(), userName)
		assert.NoError(collect, err)
	})
}

func assertSCIMUserDoesntExist(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetUserByUserName(context.Background(), userName)
		assert.True(collect, trace.IsNotFound(err))
	})
}

func assertSCIMUserExistAndIsNotActive(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		out, err := client.GetUserByUserName(context.Background(), userName)
		if !assert.NoError(collect, err) {
			return
		}
		assert.False(collect, out.Active)
	})
}

func assertSCIMUserExistAndIsActive(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		out, err := client.GetUserByUserName(context.Background(), userName)
		if !assert.NoError(collect, err) {
			return
		}
		assert.True(collect, out.Active)
	})
}

func assertSCIMGroupDoestExist(t *testing.T, scimClient scimsdk.Client, displayName string) {
	// TODO: fix typo in name (missing `n`)
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := scimClient.GetGroupByDisplayName(context.Background(), displayName)
		assert.True(collect, trace.IsNotFound(err), "Downstream group %q should not exist", displayName)
	})
}

type userOption struct {
	labels map[string]string
}

type userOptionFn func(*userOption)

func withUserLabels(labels map[string]string) userOptionFn {
	return func(opts *userOption) {
		opts.labels = labels
	}
}

func (s *testPack) mustCreateTeleportUser(t *testing.T, name string, options ...userOptionFn) {
	opts := &userOption{}
	for _, opt := range options {
		opt(opts)
	}
	_, err := s.depsMock.CreateUser(context.Background(), &types.UserV2{
		Metadata: types.Metadata{
			Labels: opts.labels,
			Name:   name,
		},
	})
	require.NoError(t, err)
}

func (s *testPack) mustCreateAccessList(t *testing.T, name, title string) *accesslist.AccessList {
	acl := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: name},
		},
		Spec: accesslist.Spec{
			Owners: []accesslist.Owner{{Name: "access-list-owner"}},
			Grants: accesslist.Grants{Roles: []string{"role1"}},
			Title:  title,
		},
	}
	return s.mustUpsertAccessList(t, acl)
}

func (s *testPack) mustCreateAccessListWithCleanup(t *testing.T, name, title string) *accesslist.AccessList {
	acl := s.mustCreateAccessList(t, name, title)
	t.Cleanup(func() {
		err := s.depsMock.DeleteAccessList(context.Background(), acl.GetName())
		require.NoError(t, err)
	})
	return acl
}

func (s *testPack) mustUpsertAccessList(t *testing.T, acl *accesslist.AccessList) *accesslist.AccessList {
	acl, err := s.depsMock.UpsertAccessList(context.Background(), acl)
	require.NoError(t, err)
	return acl
}

func (s *testPack) mustUpsertAccessListMember(t *testing.T, accessList, memberName, memberKind string) {
	s.aclMember(t, accessList, memberName, memberKind, "" /* withOrigin */)
}

func (s *testPack) mustUpsertAccessListMemberWithNonExistentUserAccount(t *testing.T, accessList, memberName, origin string) {
	s.aclMember(t, accessList, memberName, accesslist.MembershipKindUser, origin)
}

func (s *testPack) aclMember(t *testing.T, accessList, memberName string, memberKind string, withOrigin string) {
	aclMember := &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: memberName},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList:     accessList,
			Name:           memberName,
			Joined:         s.clock.Now(),
			AddedBy:        "ut-test",
			MembershipKind: memberKind,
		},
	}
	if withOrigin != "" {
		aclMember.SetOrigin(withOrigin)
		aclMember.Metadata.Labels[ExternalIDLabel.String()] = memberName
	}
	_, err := s.depsMock.UpsertAccessListMember(context.Background(), aclMember)
	require.NoError(t, err)
}

func (s *testPack) getAccessListProvisioningState(t *testing.T, aclID string) *provisioningv1.PrincipalState {
	pps, err := s.depsMock.GetProvisioningState(context.Background(), s.downstreamID, getIDForAccessListName(aclID))
	require.NoError(t, err, "Principal Provisioning State for Access List %q must exist", aclID)
	return pps
}

func (s *testPack) getUserProvisioningState(t *testing.T, username string) *provisioningv1.PrincipalState {
	pps, err := s.depsMock.GetProvisioningState(context.Background(), s.downstreamID, getIDForUserName(username))
	require.NoError(t, err, "Principal Provisioning State for user %q must exist", username)
	return pps
}

func (s *testPack) getProvisioningStates(t *testing.T) []*provisioningv1.PrincipalState {
	var result []*provisioningv1.PrincipalState
	for pps, err := range allProvisioningStates(t.Context(), s.depsMock, s.downstreamID) {
		require.NoError(t, err)
		result = append(result, pps)
	}
	return result
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
