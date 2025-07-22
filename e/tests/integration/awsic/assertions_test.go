package awsic

import (
	"context"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/auth"
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
	// TODO(tcsc): handle multiple pages of users
	response, err := client.ListUsers(ctx)
	if !assert.NoError(t, err) {
		return false
	}

	scimUserNames := make([]string, len(response.Users))
	for i, scimUser := range response.Users {
		scimUserNames[i] = scimUser.UserName
	}

	return assert.ElementsMatch(t, expectedUsers, scimUserNames)
}
