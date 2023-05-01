/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestAssignmentClient(t *testing.T) {
	ctx := context.Background()
	oktaClient := newTestClient()
	rateLimiter := rate.NewLimiter(rate.Inf, 1)
	assignmentClient := newAssignmentClient(oktaClient, rateLimiter)
	testGroup := "test-group"
	testApp := "test-app"
	testUser := "test-user@test.user"
	testOktaUserID := "okta-user-id"

	// No user present.
	_, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.ErrorIs(t, trace.NotFound("unable to find ID for user %s", testUser), err)

	_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.ErrorIs(t, trace.NotFound("unable to find ID for user %s", testUser), err)

	oktaClient.addUserID(testUser, testOktaUserID)

	// Refresh client so that the cache will retry the user.
	assignmentClient = newAssignmentClient(oktaClient, rateLimiter)

	// User present, but no apps/groups present.
	_, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.ErrorIs(t, err, trace.NotFound("assignments for app %s not found", testApp))

	_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.ErrorIs(t, err, trace.NotFound("assignments for group %s not found", testGroup))

	// Cache didn't populate for the apps/groups due to the failures, so we'll add in the apps/groups now.
	oktaClient.addApplicationToMapping(testApp)
	oktaClient.addGroupToMapping(testGroup)

	// Refresh client.
	assignmentClient = newAssignmentClient(oktaClient, rateLimiter)

	// User present, apps/groups present. This shouldn't fail, but the result of this should be cached.
	ok, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.NoError(t, err)
	require.False(t, ok)

	// Assign users to client, which will be ignored.
	oktaClient.assignUserToApplication(ctx, testOktaUserID, testApp)
	oktaClient.assignUserToGroup(ctx, testOktaUserID, testGroup)

	// User present, apps/groups present, using cached result.
	ok, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.NoError(t, err)
	require.False(t, ok)

	// Refresh caching client.
	assignmentClient = newAssignmentClient(oktaClient, rateLimiter)

	// Updated assignments should be used.
	ok, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.NoError(t, err)
	require.True(t, ok)

	// Unregister app should be reflected in the cache and in the backend client.
	require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))

	ok, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
	require.NoError(t, err)
	require.False(t, ok)

	require.Empty(t, cmp.Diff(map[string]map[string]bool{
		testApp: {},
	}, oktaClient.appsToUsers))

	// Unregister group should be reflected in the cache and in the backend client.
	require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))

	ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
	require.NoError(t, err)
	require.False(t, ok)

	require.Empty(t, cmp.Diff(map[string]map[string]bool{
		testGroup: {},
	}, oktaClient.groupsToUsers))
}
