package awsic

import (
	"context"
	"slices"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/assert"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
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

// assertSCIMUsersExist asserts that the SCIM service user list includes the supplied
// users by name. The test is not exclusive, meaning the SCIM service may have
// other users as well Takes an [assert.TestingT] rathe than a [require.TestingT]
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
func requireSCIMGroup(ctx context.Context, t *testing.T, client scimsdk.Client, displayName string, assertions ...scimGroupAssertion) {
	t.Helper()
	if assertSCIMGroup(ctx, t, client, displayName, assertions...) {
		return
	}
	t.FailNow()
}

// principalAssignmentAssertion is the signature for functions that assert properties
// of an Identity Center Principal Assignment record
type principalAssignmentAssertion func(assert.TestingT, *identitycenterv1.PrincipalAssignment) bool

// hasProvisioningState returns a [principalAssignmentAssertion] that asserts
// a Principal Assignment's ProvisioningState status field
func hasProvisioningState(s identitycenterv1.ProvisioningState) principalAssignmentAssertion {
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		return assert.Equal(t, s, pa.GetStatus().GetProvisioningState())
	}
}

// hasExternalID returns a [principalAssignmentAssertion] that asserts the value of a
// Principal Assignment record's external ID
func hasExternalID(expectedID string) principalAssignmentAssertion {
	return func(t assert.TestingT, pa *identitycenterv1.PrincipalAssignment) bool {
		return assert.Equal(t, expectedID, pa.GetSpec().GetExternalId())
	}
}

// assertPrincipalAssignment asserts that an Identity Center Principal Assignment
// record exists for the supplied principal ID, and runs the supplied assertions
// on it. Returns after the first failed assertion. Takes an [assert.TestingT]
// rather than a [require.TestingT] in order to be usable inside a
// [require.EventuallyWithT] callback.
func assertPrincipalAssignment(ctx context.Context, t assert.TestingT, getter services.IdentityCenterPrincipalAssignments, id services.PrincipalAssignmentID, assertions ...principalAssignmentAssertion) bool {
	state, err := getter.GetPrincipalAssignment(ctx, id)
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

type icUserAssertion func(context.Context, assert.TestingT, icsdk.Client, *icsdk.User) bool

func hasAccountAssignments(expected ...*icsdk.Assignment) icUserAssertion {
	return func(ctx context.Context, t assert.TestingT, client icsdk.Client, user *icsdk.User) bool {
		assignments, err := client.ListAssignments(ctx, user.ID, ssoadmintypes.PrincipalTypeUser)
		if !assert.NoError(t, err) {
			return false
		}
		return assert.ElementsMatch(t, expected, assignments)
	}
}

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
