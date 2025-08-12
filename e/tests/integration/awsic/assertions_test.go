package awsic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
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

func assertSCIMGroup(ctx context.Context, t assert.TestingT, client scimsdk.Client, displayName string, expectedMembers ...string) bool {
	g, err := client.GetGroupByDisplayName(ctx, displayName)
	if !assert.NoError(t, err, "Group with display name %q must exist", displayName) {
		return false
	}

	actualMembers := make([]string, len(g.Members))
	for i, member := range g.Members {
		actualMembers[i] = member.ExternalID
	}

	return assert.ElementsMatch(t, expectedMembers, actualMembers)
}

func requireSCIMGroup(ctx context.Context, t *testing.T, client scimsdk.Client, displayName string, expectedMembers ...string) {
	t.Helper()
	if assertSCIMGroup(ctx, t, client, displayName, expectedMembers...) {
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

// assertPrincipalAssignment asserts that an Identity Center Principal Assignment
// record exists for the supplied principal ID, nd runs the supplied assertions
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
