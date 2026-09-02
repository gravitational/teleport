package awsic

import (
	"context"
	"slices"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

// assertImportStatus defines a function signature for assertions about AWSICGroupImportStatus
// values. For use with assertImportStatus.
type groupImportStatusAssertion func(assert.TestingT, *types.AWSICGroupImportStatus) bool

// groupImportStatusCodeIs asserts that an AWSICGroupImportStatus has the supplied
// StatusCode value. For use with assertImportStatus,
func groupImportStatusCodeIs(code types.AWSICGroupImportStatusCode) groupImportStatusAssertion {
	return func(t assert.TestingT, status *types.AWSICGroupImportStatus) bool {
		return assert.Equal(t, code, status.StatusCode)
	}
}

// assertImportStatus asserts that the system AWSIC plugin resource has a valid
// GroupImportStatus block, and runs the supplied assertions on it. Takes an
// [assert.TestingT] rather than a [require.TestingT] in order to be usable
// inside a [require.EventuallyWithT] callback.
func assertImportStatus(ctx context.Context, t assert.TestingT, auth *auth.Server, assertions ...groupImportStatusAssertion) bool {
	p, err := auth.GetPlugin(ctx, types.PluginTypeAWSIdentityCenter, false /* no secrets */)
	if !assert.NoError(t, err) {
		return false
	}

	status := p.GetStatus()
	awsStatus := status.GetAwsIc()
	if !assert.NotNil(t, awsStatus) {
		return false
	}

	if !assert.NotNil(t, awsStatus.GroupImportStatus) {
		return false
	}

	for _, assertionFn := range assertions {
		if !assertionFn(t, awsStatus.GroupImportStatus) {
			return false
		}
	}

	return true
}

// assertSCIMUsers asserts that the SCIM service user list includes the supplied
// users by name, and ONLY those users. Takes an [assert.TestingT] rathe than a
// [require.TestingT] in order to be usable inside a [require.EventuallyWithT]
// callback.
func assertSCIMUsers(ctx context.Context, t assert.TestingT, client scimsdk.Client, expectedUsers ...string) bool {
	var scimUserNames []string
	for scimUser, err := range scimsdk.StreamUsers(ctx, client) {
		if !assert.NoError(t, err) {
			return false
		}
		scimUserNames = append(scimUserNames, scimUser.UserName)
	}
	return assert.ElementsMatch(t, expectedUsers, scimUserNames)
}

// requireSCIMUsers asserts that the SCIM service user list includes the supplied
// users by name, and ONLY those users. Aborts the test immediately on failure.
func requireSCIMUsers(ctx context.Context, t require.TestingT, client scimsdk.Client, expectedUsers ...string) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertSCIMUsers(ctx, t, client, expectedUsers...) {
		return
	}
	t.FailNow()
}

// assertSCIMUsersExist asserts that the SCIM service user list includes the supplied
// users by name. The test is not exclusive, meaning the SCIM service may have
// other users as well. Takes an [assert.TestingT] rather than a [require.TestingT]
// in order to be usable inside a [require.EventuallyWithT] callback.
func assertSCIMUsersExist(ctx context.Context, t assert.TestingT, client scimsdk.Client, expectedUsers ...string) bool {
	required := set.New(expectedUsers...)
	for scimUser, err := range scimsdk.StreamUsers(ctx, client) {
		if !assert.NoError(t, err) {
			return false
		}
		required.Remove(scimUser.UserName)
	}
	return assert.Empty(t, required)
}

// requireSCIMUsersExist asserts that the SCIM service user list includes the supplied
// users by name, immediately failing the test if the supplied users don't exist.
// The test is not exclusive, meaning the SCIM service may have other users as well.
func requireSCIMUsersExist(ctx context.Context, t require.TestingT, client scimsdk.Client, expectedUsers ...string) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertSCIMUsersExist(ctx, t, client, expectedUsers...) {
		return
	}
	t.FailNow()
}

func assertSCIMGroupsByDisplayName(ctx context.Context, t assert.TestingT, client scimsdk.Client, expectedDisplayNames ...string) bool {
	var displayNames []string
	for g, err := range scimsdk.StreamGroups(ctx, client) {
		if !assert.NoError(t, err) {
			return false
		}
		displayNames = append(displayNames, g.DisplayName)
	}
	return assert.ElementsMatch(t, expectedDisplayNames, displayNames)
}

func assertAccessLists(ctx context.Context, t assert.TestingT, lister iciter.AccessListLister, expectedTitles ...string) bool {
	var accessListTitles []string
	for acl, err := range iciter.AllAccessLists(ctx, lister) {
		if !assert.NoError(t, err) {
			return false
		}
		accessListTitles = append(accessListTitles, acl.Spec.Title)
	}
	return assert.ElementsMatch(t, expectedTitles, accessListTitles)
}

// scimGroupAssertion is the signature for functions that assert the properties
// of a [scimsdk.Group].
type scimGroupAssertion func(assert.TestingT, *scimsdk.Group) bool

// hasMembers returns a [scimGroupAssertion] that asserts that the group has a
// specific member list.
func hasMembers(expectedMembers ...string) scimGroupAssertion {
	return func(t assert.TestingT, g *scimsdk.Group) bool {
		actualMembers := make([]string, len(g.Members))
		for i, member := range g.Members {
			actualMembers[i] = member.ExternalID
		}
		return assert.ElementsMatch(t, expectedMembers, actualMembers)
	}
}

// assertSCIMGroup asserts the existence of a SCIM group with a given display name,
// and runs the supplied assertions on it. Returns after the first failed assertion.
// Takes an [assert.TestingT] rather than a [require.TestingT] in order to be usable
// inside a [require.EventuallyWithT] callback.
func assertSCIMGroup(ctx context.Context, t assert.TestingT, client scimsdk.Client, displayName string, assertions ...scimGroupAssertion) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	g, err := client.GetGroupByDisplayName(ctx, displayName)
	if !assert.NoError(t, err, "Group with display name %q must exist", displayName) {
		return false
	}

	for _, assertionFn := range assertions {
		if !assertionFn(t, g) {
			return false
		}
	}

	return true
}

// requireSCIMGroup asserts the existence of a SCIM group with a given display name,
// and runs the supplied assertions on it. Immediately fails the test if any
// assertions fail.
func requireSCIMGroup(ctx context.Context, t require.TestingT, client scimsdk.Client, displayName string, assertions ...scimGroupAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertSCIMGroup(ctx, t, client, displayName, assertions...) {
		return
	}
	t.FailNow()
}

// principalAssignmentAssertion is the signature for functions that assert properties
// of an Identity Center Principal Assignment record
type principalAssignmentAssertion func(assert.TestingT, *identitycenterv1.PrincipalAssignment) bool

// hasUserPrincipalID asserts that a Principal Assignment Record has the appropriate name
// to be associated with a given user.
func hasUserPrincipalID(username string) principalAssignmentAssertion {
	id := principal.GetIDForUserName(username)
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		return assert.Equal(t, string(id), pa.GetMetadata().GetName())
	}
}

// hasProvisioningState returns a [principalAssignmentAssertion] that asserts
// a Principal Assignment's ProvisioningState status field
func hasProvisioningState(s identitycenterv1.ProvisioningState) principalAssignmentAssertion {
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		return assert.Equal(t, s.String(), pa.GetStatus().GetProvisioningState().String())
	}
}

func hasAnyExternalID(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
	return assert.NotEmpty(t, pa.GetSpec().GetExternalId(), "Expected a non-empty ExternalID")
}

// hasExternalID returns a [principalAssignmentAssertion] that asserts the value of a
// Principal Assignment record's external ID
func hasExternalID(expectedID string) principalAssignmentAssertion {
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		return assert.Equal(t, expectedID, pa.GetSpec().GetExternalId())
	}
}

// hasAccountAssignment returns a [principalAssignmentAssertion] asserting
// that the Principal Assignment Record has a specific set of account assignments
// recorded against it
func hasAccountAssignment(ps, accountID string) principalAssignmentAssertion {
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		idx := slices.IndexFunc(
			pa.GetStatus().GetAssignments(),
			func(asmt *identitycenterv1.AccountAssignmentRef) bool {
				return asmt.GetAccountId() == accountID && asmt.GetPermissionSetArn() == ps
			})
		return assert.NotEqual(t, -1, idx, "No such account assignment found")
	}
}

// hasNoAccountAssignments asserts that the target principal has no Account
// Assignments recorded against them
func hasNoAccountAssignments(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
	return assert.Empty(t, pa.GetStatus().GetAssignments(),
		"Principal must have no account assignments")
}

func principalAssignment(assertions ...principalAssignmentAssertion) func(*identitycenterv1.PrincipalAssignment) bool {
	return func(pa *identitycenterv1.PrincipalAssignment) bool {
		var collector common.CollectT
		for _, assertion := range assertions {
			if !assertion(&collector, pa) {
				return false
			}
		}
		return true
	}
}

func waitForAllPrincipalAssignments(t *testing.T, watcher types.Watcher, selectors ...func(*identitycenterv1.PrincipalAssignment) bool) []*identitycenterv1.PrincipalAssignment {
	return common.WaitForAllResource153PutEvents(t, watcher, selectors...)
}

// waitForPrincipalAssignment waits for a put event on the supplied watcher that
// matches the supplied predicates.
func waitForPrincipalAssignment(t *testing.T, watcher types.Watcher, assertions ...principalAssignmentAssertion) *identitycenterv1.PrincipalAssignment {
	results := waitForAllPrincipalAssignments(t, watcher, principalAssignment(assertions...))
	return results[0]
}

// waitForPrincipalAssignmentDeletion waits for a delete event on the supplied
// watcher that affects the given principal.
func waitForPrincipalAssignmentDeletion(t *testing.T, watcher types.Watcher, id services.PrincipalAssignmentID) {
	common.WaitForDeleteEvent(t, watcher, func(r types.Resource) bool {
		return r.GetKind() == types.KindIdentityCenterPrincipalAssignment &&
			r.GetName() == string(id)
	})
}

// assertPrincipalAssignment asserts that an Identity Center Principal Assignment
// record exists for the supplied principal ID, and runs the supplied assertions
// on it. Returns after the first failed assertion. Takes an [assert.TestingT]
// rather than a [require.TestingT] in order to be usable inside a
// [require.EventuallyWithT] callback.
func assertPrincipalAssignment(
	ctx context.Context,
	t assert.TestingT,
	getter services.IdentityCenterPrincipalAssignments,
	id services.PrincipalAssignmentID,
	assertions ...principalAssignmentAssertion,
) (bool, *identitycenterv1.PrincipalAssignment) {
	state, err := getter.GetPrincipalAssignment(ctx, id)
	if !assert.NoError(t, err) {
		return false, nil
	}

	for _, assertion := range assertions {
		if !assertion(t, state) {
			return false, nil
		}
	}

	return true, state
}

// requirePrincipalAssignment asserts that an Identity Center Principal Assignment
// record exists for the supplied principal ID, and runs the supplied assertions
// on it.
func requirePrincipalAssignment(
	ctx context.Context,
	t require.TestingT,
	getter services.IdentityCenterPrincipalAssignments,
	id services.PrincipalAssignmentID,
	assertions ...principalAssignmentAssertion,
) *identitycenterv1.PrincipalAssignment {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if passed, state := assertPrincipalAssignment(ctx, t, getter, id, assertions...); passed {
		return state
	}
	t.FailNow()
	return nil // This should never be hit due to above call to `FailNow`
}

type scimProvisioningStateAssertion func(assert.TestingT, *provisioningv1.PrincipalState) bool

func hasSCIMUserPrincipalID(username string) scimProvisioningStateAssertion {
	id := provisioning.GetIDForUserName(username)
	return func(t assert.TestingT, ps *provisioningv1.PrincipalState) bool {
		return assert.Equal(t, string(id), ps.GetMetadata().GetName())
	}
}

func hasSCIMProvisioningState(s provisioningv1.ProvisioningState) scimProvisioningStateAssertion {
	return func(t assert.TestingT, ps *provisioningv1.PrincipalState) bool {
		return assert.Equal(t, s.String(), ps.GetStatus().GetProvisioningState().String())
	}
}

func hasSCIMExternalID(expected string) scimProvisioningStateAssertion {
	return func(t assert.TestingT, ps *provisioningv1.PrincipalState) bool {
		return assert.Equal(t, expected, ps.GetStatus().GetExternalId())
	}
}

func hasSCIMErrorMatching(pattern string) scimProvisioningStateAssertion {
	return func(t assert.TestingT, ps *provisioningv1.PrincipalState) bool {
		return assert.Regexp(t, pattern, ps.GetStatus().GetError(), "Error must match regex")
	}
}

func scimProvisioningState(assertions ...scimProvisioningStateAssertion) func(*provisioningv1.PrincipalState) bool {
	return func(s *provisioningv1.PrincipalState) bool {
		var collector common.CollectT
		for _, assertion := range assertions {
			if !assertion(&collector, s) {
				return false
			}
		}
		return true
	}
}

func waitForAllSCIMProvisioningStates(t *testing.T, watcher types.Watcher, selectors ...func(*provisioningv1.PrincipalState) bool) []*provisioningv1.PrincipalState {
	return common.WaitForAllResource153PutEvents[*provisioningv1.PrincipalState](t, watcher, selectors...)
}

func waitForSCIMProvisioningState(t *testing.T, watcher types.Watcher, assertions ...scimProvisioningStateAssertion) *provisioningv1.PrincipalState {
	results := common.WaitForAllResource153PutEvents[*provisioningv1.PrincipalState](t, watcher, scimProvisioningState(assertions...))
	return results[0]
}

func waitForSCIMProvisioningStateDeletion(t *testing.T, watcher types.Watcher, id services.ProvisioningStateID) {
	common.WaitForDeleteEvent(t, watcher, func(r types.Resource) bool {
		return r.GetKind() == types.KindProvisioningPrincipalState &&
			r.GetName() == string(id)
	})
}

func assertSCIMProvisioningState(ctx context.Context, t assert.TestingT, getter services.ProvisioningStates, id services.ProvisioningStateID, assertions ...scimProvisioningStateAssertion) bool {
	state, err := getter.GetProvisioningState(ctx, identitycenter.IdentityCenterDownstreamID, id)
	if !assert.NoError(t, err) {
		return false
	}

	for _, assertion := range assertions {
		if !assertion(t, state) {
			return false
		}
	}

	return true
}

func requireSCIMProvisioningState(ctx context.Context, t require.TestingT, getter services.ProvisioningStates, id services.ProvisioningStateID, assertions ...scimProvisioningStateAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertSCIMProvisioningState(ctx, t, getter, id, assertions...) {
		return
	}
	t.FailNow()
}

type roleAssertion func(assert.TestingT, types.Role) bool

func hasAllowAccountAssignments(expected ...types.IdentityCenterAccountAssignment) roleAssertion {
	return func(t assert.TestingT, role types.Role) bool {
		rv6, ok := role.(*types.RoleV6)
		if !assert.True(t, ok, "unexpected role type %T", role) {
			return false
		}
		return assert.ElementsMatch(t, expected, rv6.Spec.Allow.AccountAssignments)
	}
}

func withRoleLabel(key, expectedValue string) roleAssertion {
	return func(t assert.TestingT, role types.Role) bool {
		actualValue, ok := role.GetLabel(key)
		return assert.True(t, ok, "Label %q must be set", key) && assert.Equal(t, expectedValue, actualValue)
	}
}

func withRoleSubkind(s string) roleAssertion {
	return func(t assert.TestingT, role types.Role) bool {
		return assert.Equal(t, s, role.GetSubKind())
	}
}

// assertRole asserts that the names Teleport role exists, and runs the supplied
// assertions on it.  Takes an [assert.TestingT] rather than a [require.TestingT]
// in order to be usable inside a [require.EventuallyWithT] callback.
func assertRole(ctx context.Context, t assert.TestingT, rolesSvc services.RoleGetter, roleName string, roleAssertions ...roleAssertion) bool {
	r, err := rolesSvc.GetRole(ctx, roleName)
	if !assert.NoError(t, err) {
		return false
	}
	for _, assertionFn := range roleAssertions {
		if !assertionFn(t, r) {
			return false
		}
	}
	return true
}

// assertRole asserts that the names Teleport role exists, and runs the supplied
// assertions on it.
func requireRole(ctx context.Context, t require.TestingT, rolesSvc services.RoleGetter, roleName string, roleAssertions ...roleAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertRole(ctx, t, rolesSvc, roleName, roleAssertions...) {
		return
	}
	t.FailNow()
}

type icUserAssertion func(context.Context, assert.TestingT, icsdk.Client, *icsdk.User) bool

// hasAccountAssignments asserts that the target user has the supplied account
// assignments in AWS, and ONLY those assignments.
func hasAccountAssignments(expected ...*icsdk.Assignment) icUserAssertion {
	return func(ctx context.Context, t assert.TestingT, client icsdk.Client, user *icsdk.User) bool {
		assignments, err := client.ListAssignments(ctx, user.ID, ssoadmintypes.PrincipalTypeUser)
		if !assert.NoError(t, err) {
			return false
		}
		return assert.ElementsMatch(t, expected, assignments)
	}
}

func icUserAccountAssignment(ps, account string) *icsdk.Assignment {
	return &icsdk.Assignment{
		AccountID:        account,
		PermissionSetARN: ps,
		PrincipalType:    ssoadmintypes.PrincipalTypeUser,
	}
}

// assertICUser asserts that a user with the given name exists in the Identity Center
// instance, and runs the supplied assertions on it. The function will return on
// the first failed assertion.
func assertICUser(ctx context.Context, t assert.TestingT, client icsdk.Client, username string, assertions ...icUserAssertion) bool {
	users, err := client.ListUsers(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	i := slices.IndexFunc(users, func(u *icsdk.User) bool { return u.UserName == username })
	if !assert.NotEqual(t, -1, i, "User %q should exist in Identity Center", username) {
		return false
	}
	user := users[i]
	for _, assertionFn := range assertions {
		if !assertionFn(ctx, t, client, user) {
			return false
		}
	}
	return true
}

// requireICUser  asserts that a user with the given name exists in the Identity Center
// instance and runs the supplied assertions on it, immediately failing the test
// if any assertion fails.
func requireICUser(ctx context.Context, t require.TestingT, client icsdk.Client, username string, assertions ...icUserAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertICUser(ctx, t, client, username, assertions...) {
		return
	}
	t.FailNow()
}

func assertNoICUser(ctx context.Context, t assert.TestingT, client icsdk.Client, username string) bool {
	users, err := client.ListUsers(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	i := slices.IndexFunc(users, func(u *icsdk.User) bool { return u.UserName == username })
	return assert.Equal(t, -1, i, "User %q should not exist in Identity Center", username)
}

func requireNoICUser(ctx context.Context, t require.TestingT, client icsdk.Client, username string) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertNoICUser(ctx, t, client, username) {
		return
	}
	t.FailNow()
}

type icGroupAssertion func(context.Context, assert.TestingT, icsdk.Client, *icsdk.Group) bool

func hasGroupAccountAssignments(expected ...*icsdk.Assignment) icGroupAssertion {
	return func(ctx context.Context, t assert.TestingT, client icsdk.Client, group *icsdk.Group) bool {
		assignments, err := client.ListAssignments(ctx, group.ID, ssoadmintypes.PrincipalTypeGroup)
		if !assert.NoError(t, err) {
			return false
		}
		return assert.ElementsMatch(t, expected, assignments)
	}
}

func icGroupAccountAssignment(ps, account string) *icsdk.Assignment {
	return &icsdk.Assignment{
		AccountID:        account,
		PermissionSetARN: ps,
		PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
	}
}

func withMembers(expectedMembers ...string) icGroupAssertion {
	return func(ctx context.Context, t assert.TestingT, client icsdk.Client, group *icsdk.Group) bool {
		groupMembers, err := client.ListGroupMemberships(ctx, group.ID)
		if !assert.NoError(t, err) {
			return false
		}

		users, err := client.ListUsers(ctx)
		if !assert.NoError(t, err) {
			return false
		}

		userLookup := make(map[string]*icsdk.User)
		for _, u := range users {
			userLookup[u.ID] = u
		}

		var actualMembers []string
		for _, gm := range groupMembers {
			user, ok := userLookup[gm.MemberID]
			if !assert.True(t, ok, "No user with ID %q in mock Identity Center Data", gm.MemberID) {
				return false
			}
			actualMembers = append(actualMembers, user.UserName)
		}

		return assert.ElementsMatch(t, expectedMembers, actualMembers)
	}
}

// assertICGroup asserts that a group with the given displayName exists in the
// Identity Center instance, and runs the supplied assertions on it. The function
// will return on the first failed assertion.
func assertICGroup(ctx context.Context, t assert.TestingT, client icsdk.Client, displayName string, assertions ...icGroupAssertion) bool {
	groups, err := client.ListGroups(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	i := slices.IndexFunc(groups, func(g *icsdk.Group) bool { return g.DisplayName == displayName })
	if !assert.NotEqual(t, -1, i, "Group with display name %q should exist in Identity Center", displayName) {
		return false
	}
	group := groups[i]
	for _, assertionFn := range assertions {
		if !assertionFn(ctx, t, client, group) {
			return false
		}
	}
	return true
}

// requireICGroup asserts that a group with the given display title exists in the
// Identity Center instance and runs the supplied assertions on it, immediately
// failing the test if any assertion fails.
func requireICGroup(ctx context.Context, t require.TestingT, client icsdk.Client, displayName string, assertions ...icGroupAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertICGroup(ctx, t, client, displayName, assertions...) {
		return
	}
	t.FailNow()
}

func assertNoICGroup(ctx context.Context, t assert.TestingT, client icsdk.Client, displayName string) bool {
	groups, err := client.ListGroups(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	i := slices.IndexFunc(groups, func(g *icsdk.Group) bool { return g.DisplayName == displayName })
	return assert.Equal(t, -1, i, "Group %q should not exist in Identity Center", displayName)
}

func requireNoICGroup(ctx context.Context, t require.TestingT, client icsdk.Client, displayName string) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertNoICGroup(ctx, t, client, displayName) {
		return
	}
	t.FailNow()
}
