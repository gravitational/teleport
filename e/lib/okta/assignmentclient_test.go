package okta

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/teleport"
)

// purgeCache clears the assignmentClient caches, forcing the client to reload
// everything from the upstream Okta service. Used only in tests.
func (a *assignmentClient) purgeCache() {
	a.usersMu.Lock()
	a.users = nil
	a.usersMu.Unlock()

	a.appsMu.Lock()
	clear(a.apps)
	a.appsMu.Unlock()

	a.groupsMu.Lock()
	clear(a.groups)
	a.groupsMu.Unlock()
}

func TestAssignmentClient(t *testing.T) {
	ctx := context.Background()
	log := logrus.WithField(trace.Component, teleport.ComponentOkta)
	testGroup := "test-group"
	testApp := "test-app"
	testUser := "test-user@test.user"
	testOktaUserID := "okta-user-id"

	// Factory for creating an assignment client backed by a test Okta client
	// pre-configured with a user and some group and app memberships.
	testClientWithAssignments := func() (*testOktaClient, *assignmentClient) {
		oktaClient := newTestClient()
		oktaClient.usernamesToUserIDs = map[string]string{
			testUser: testOktaUserID,
		}
		oktaClient.appsToUsers = map[string]map[string]bool{
			testApp: {testOktaUserID: true},
		}
		oktaClient.groupsToUsers = map[string]map[string]bool{
			testGroup: {testOktaUserID: true},
		}
		oktaClient.appsToGroups = map[string][]string{
			testApp: {testGroup},
		}
		assignmentClient := newAssignmentClient(log, oktaClient)

		return oktaClient, assignmentClient
	}

	t.Run("no such user is an error", func(t *testing.T) {
		// Given an Okta system with no users or groups...
		assignmentClient := newAssignmentClient(log, newTestClient())

		// When I attempt to perform operations that require a given
		// user to exist, those operations will fail

		_, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.ErrorIs(t, trace.NotFound("unable to find ID for user %s", testUser), err)

		_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.ErrorIs(t, trace.NotFound("unable to find ID for user %s", testUser), err)
	})

	t.Run("user with no apps no groups", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user and
		// no apps or groups configured...
		oktaClient := newTestClient()
		oktaClient.usernamesToUserIDs = map[string]string{
			testUser: testOktaUserID,
		}
		assignmentClient := newAssignmentClient(log, oktaClient)

		// When I test that user's group and app memberships, the tests all
		// return NotFound because the Apps and Groups do not exist .

		_, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.ErrorIs(t, err, trace.NotFound("assignments for app %s not found", testApp))

		_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.ErrorIs(t, err, trace.NotFound("assignments for group %s not found", testGroup))
	})

	t.Run("assignments and caching", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user, one
		// group and one app configured, but the user is not assigned to either...
		oktaClient := newTestClient()
		oktaClient.usernamesToUserIDs = map[string]string{
			testUser: testOktaUserID,
		}
		oktaClient.appsToUsers = map[string]map[string]bool{
			testApp: {},
		}
		oktaClient.groupsToUsers = map[string]map[string]bool{
			testGroup: {},
		}
		assignmentClient := newAssignmentClient(log, oktaClient)

		// When we test for membership, the operations succeed and return `false`.
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)

		// When we manually make the user and group assignments in the back-end
		// test client and re-test the membership, expect that the
		// assignmentClient uses cached data rather than re-querying the back
		// end, and so still reports `false`.
		oktaClient.appsToUsers[testApp][testOktaUserID] = true
		oktaClient.groupsToUsers[testGroup][testOktaUserID] = true

		isAssigned, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)

		// When we purge the assignmentClient cache and retry the membership
		// tests, expect that the assignmentClient fetches new values from Okta
		// and so now reports `true`
		assignmentClient.purgeCache()

		isAssigned, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, isAssigned)
	})

	t.Run("reassignment", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user, one
		// group and one app configured, and the user is assigned to both...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When I attempt to disassociate a user from an app or group,
		// expect the operations to succeed and subsequent membership
		// tests to return `false`
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		ok, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, ok)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, ok)

		// Also expect that the change has been passed through to the okta
		// client
		require.Empty(t, oktaClient.groupsToUsers[testGroup])
		require.Empty(t, oktaClient.appsToUsers[testApp])

		// When I attempt to re-associate a user to an app or group,
		// expect the operations to succeed and subsequent membership
		// tests to return `true`

		require.NoError(t, assignmentClient.registerUserToApp(ctx, testUser, testApp))
		ok, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, ok)

		require.NoError(t, assignmentClient.registerUserToGroup(ctx, testUser, testGroup))
		ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("validation errors are not propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail with a oktaAPIValidationError
		oktaClient.unassignAppErr[testApp] = &oktaAPIValidationError{}
		oktaClient.unassignGroupErr[testGroup] = &oktaAPIValidationError{}

		// Expect that the oktaAPIValidationError is treated as a success, the
		// error is *NOT* propagated from the underlying Okta client, and the
		// user assignments are updated as requested
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)
	})

	t.Run("not found errors are not propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail with a NotFound error
		oktaClient.unassignAppErr[testApp] = trace.WithField(trace.NotFound("summary"), oktaErrorID, oktaErrCodeNotFoundException)
		oktaClient.unassignGroupErr[testGroup] = trace.WithField(trace.NotFound("summary"), oktaErrorID, oktaErrCodeNotFoundException)

		// Expect that the NotFound is treated as a success, the
		// error is *NOT* propagated from the underlying Okta client, and the
		// user assignments are updated as requested
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)
	})

	t.Run("unassignment errors are propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail...
		oktaClient.unassignAppErr[testApp] = trace.BadParameter("bad parameter")
		oktaClient.unassignGroupErr[testGroup] = trace.BadParameter("bad parameter")

		// Expect that general errors are propagated from the underlying Okta
		// client, and that the assignments have not been affected.

		require.Error(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, isAssigned)

		require.Error(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, isAssigned)
	})
}
