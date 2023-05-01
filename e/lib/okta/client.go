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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
)

const (
	oktaErrorID          = "errorId"
	oktaUserProfileLogin = "login"
)

var _ oktaClient = (*wrappedClient)(nil)

// wrappedClient is a client type that is backed by an Okta SDK client.
type wrappedClient struct {
	client     *okta.Client
	oktaOrgURL string
}

// iterateGroups will iterate over the list of all Okta groups.
func (w *wrappedClient) iterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	// The default page size is 10000 here, which is fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-groups
	oktaGroups, resp, err := w.client.Group.ListGroups(ctx, query.NewQueryParams())
	for {
		if err != nil {
			return trace.Wrap(oktaErrToTrace(err), "error when iterating through groups")
		}

		for _, oktaGroup := range oktaGroups {
			if err := fn(oktaGroup); err != nil {
				return trace.Wrap(err)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &oktaGroups)
	}

	return nil
}

// iterateApps will iterate over the list of all Okta applications.
func (w *wrappedClient) iterateApps(ctx context.Context, fn func(okta.App) error) error {
	// The default for application listing is 20 per page. Here we'll bump it
	// to the max of 200 per page to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-applications
	oktaApps, resp, err := w.client.Application.ListApplications(ctx, query.NewQueryParams(
		query.WithLimit(200), // Max size.
	))
	for {
		if err != nil {
			return trace.Wrap(oktaErrToTrace(err), "error when iterating through apps")
		}

		for _, oktaApp := range oktaApps {
			if err := fn(oktaApp); err != nil {
				return trace.Wrap(err)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &oktaApps)
	}

	return nil
}

// getGroupAssignments will return the list of users assigned to a group.
func (w *wrappedClient) getGroupAssignments(ctx context.Context, groupID string) ([]string, error) {
	var userIDs []string

	// The default number of users here is 1000, which will be fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-group-members
	groupUsers, resp, err := w.client.Group.ListGroupUsers(ctx, groupID, query.NewQueryParams())

	for {
		if err != nil {
			return nil, trace.Wrap(oktaErrToTrace(err), "error when getting group user assignments")
		}

		for _, groupUser := range groupUsers {
			userIDs = append(userIDs, groupUser.Id)
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groupUsers)
	}

	return userIDs, nil
}

// getAppAssignments will return the list of users assigned to an app.
func (w *wrappedClient) getAppAssignments(ctx context.Context, appID string) ([]string, error) {
	var userIDs []string

	// We'll use the max page size of 500 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-users-assigned-to-application
	appUsers, resp, err := w.client.Application.ListApplicationUsers(ctx, appID, query.NewQueryParams(
		query.WithLimit(500),
	))

	for {
		if err != nil {
			return nil, trace.Wrap(oktaErrToTrace(err), "error when getting application user assignments")
		}

		for _, appUser := range appUsers {
			userIDs = append(userIDs, appUser.Id)
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &appUsers)
	}

	return userIDs, nil
}

// listUsers will return a mapping of usernames to user IDs from Okta.
func (w *wrappedClient) listUsers(ctx context.Context) (map[string]string, error) {
	usernameToUserID := map[string]string{}

	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/users/#list-users
	users, resp, err := w.client.User.ListUsers(ctx, query.NewQueryParams(
		query.WithLimit(200),
	))

	for {
		if err != nil {
			return nil, trace.Wrap(oktaErrToTrace(err), "error while listing users")
		}

		for _, user := range users {
			profile := user.Profile
			if profile != nil {
				usernameToUserID[fmt.Sprintf("%s", (*profile)[oktaUserProfileLogin])] = user.Id
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &users)
	}

	return usernameToUserID, nil
}

// assignUserToGroup will assign the given user to the group.
func (w *wrappedClient) assignUserToGroup(ctx context.Context, userID, groupId string) error {
	if _, err := w.client.Group.AddUserToGroup(ctx, groupId, userID); err != nil {
		return oktaErrToTrace(err)
	}

	return nil
}

// unassignUserFromGroup will unassign the given user from the group.
func (w *wrappedClient) unassignUserFromGroup(ctx context.Context, userID, groupId string) error {
	if _, err := w.client.Group.RemoveUserFromGroup(ctx, groupId, userID); err != nil {
		return oktaErrToTrace(err)
	}

	return nil
}

// assignUserToApplication will assign the given user to the application.
func (w *wrappedClient) assignUserToApplication(ctx context.Context, username, applicationId string) error {
	user, _, err := w.client.User.GetUser(ctx, username)
	if err != nil {
		return oktaErrToTrace(err)
	}

	appUser := okta.AppUser{
		Id: user.Id,
	}
	if _, _, err := w.client.Application.AssignUserToApplication(ctx, applicationId, appUser); err != nil {
		return oktaErrToTrace(err)
	}

	return nil
}

// unassignUserFromApplication will unassign the given user from the application.
func (w *wrappedClient) unassignUserFromApplication(ctx context.Context, username, applicationId string) error {
	user, _, err := w.client.User.GetUser(ctx, username)
	if err != nil {
		return oktaErrToTrace(err)
	}

	// Unlike groups, deleting a non-existent application user will produce an error from the Okta API.
	if _, err := w.client.Application.DeleteApplicationUser(ctx, applicationId, user.Id, query.NewQueryParams()); err != nil {
		return oktaErrToTrace(err)
	}

	return nil
}

// getOrgURL will return the org URL for the client.
func (w *wrappedClient) orgURL() string {
	return w.oktaOrgURL
}

const (
	// Okta error constants are not housed within the SDK, so we'll need to refer to the
	// documentation directly and define our own..
	// https://developer.okta.com/docs/reference/error-codes/
	oktaErrCodeAuthenticationException   = "E0000004"
	oktaErrCodeInvalidSessionException   = "E0000005"
	oktaErrCodeAccessDeniedException     = "E0000006"
	oktaErrCodeResourceNotFoundException = "E0000007"
	oktaErrCodeNotFoundException         = "E0000008"
)

// oktaErrToTrace takes Okta errors and converts them into appropriate trace equivalents.
func oktaErrToTrace(err error) error {
	oktaErr, ok := err.(*okta.Error)

	// If this is not an Okta error, just wrap the error and return it.
	if !ok {
		return trace.Wrap(err)
	}

	switch oktaErr.ErrorCode {
	case oktaErrCodeAuthenticationException, oktaErrCodeInvalidSessionException, oktaErrCodeAccessDeniedException:
		return trace.WithField(trace.AccessDenied(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	case oktaErrCodeResourceNotFoundException, oktaErrCodeNotFoundException:
		return trace.WithField(trace.NotFound(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	default:
		// If we don't have a more specific error to provide, just wrap the error and return it.
		return trace.WithField(trace.BadParameter(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	}
}
