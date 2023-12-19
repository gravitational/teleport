package okta

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	oktaErrorID          = "errorId"
	oktaUserProfileLogin = "login"
)

var _ oktaClient = (*wrappedClient)(nil)

// wrappedClient is a wrapper around an Okta SDK client that provides
// higher-level operations and for interacting with an Okta server
// over using the basic SDK client (e.g. result pagination)
type wrappedClient struct {
	log              *logrus.Entry
	client           *okta.Client
	oktaOrgURL       string
	pluginStatusSink common.StatusSink
}

// iterateUsers iterates over all users in the Okta system, invoking the
// supplied function for every user. The user callback may return the
// `stopIteration` error to signal that it doesn't want any more users.
func (w *wrappedClient) iterateUsers(ctx context.Context, fn func(*okta.User) error) error {
	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/users/#list-users
	users, resp, err := w.client.User.ListUsers(ctx, query.NewQueryParams(
		query.WithLimit(200),
	))

	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error while iterating over okta users")
		}

		for _, user := range users {
			if err = fn(user); err != nil {
				if err == stopIteration {
					break
				}
				return trace.Wrap(w.oktaErrToTrace(ctx, err), "error while inspecting okta user %s", user.Id)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &users)
	}

	return nil
}

// iterateGroups will iterate over the list of all Okta groups, invoking the
// supplied function for every group record. The callback may return the
// `stopIteration` error to signal that it doesn't want any more groups.
func (w *wrappedClient) iterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	// The default page size is 10000 here, but that seems to be beyond what the HTTP client built
	// into the go Okta client can handle, so I'm limiting it to 200.
	// https://developer.okta.com/docs/reference/api/groups/#list-groups
	oktaGroups, resp, err := w.client.Group.ListGroups(ctx, query.NewQueryParams(
		query.WithLimit(200), // A smaller page size so that we don't blow up the go Okta HTTP client.
	))
	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when iterating through groups")
		}

		for _, oktaGroup := range oktaGroups {
			if err := fn(oktaGroup); err != nil {
				if err == stopIteration {
					break
				}
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

// iterateApps will iterate over the list of all Okta applications. The callback
// may return the `stopIteration` error to signal that it doesn't want any more
// apps.
func (w *wrappedClient) iterateApps(ctx context.Context, fn func(okta.App) error) error {
	// The default for application listing is 20 per page. Here we'll bump it
	// to the max of 200 per page to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-applications
	oktaApps, resp, err := w.client.Application.ListApplications(ctx, query.NewQueryParams(
		query.WithLimit(200), // Max size.
	))
	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when iterating through apps")
		}

		for _, oktaApp := range oktaApps {
			if err := fn(oktaApp); err != nil {
				if err == stopIteration {
					break
				}
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
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting group user assignments")
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
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application user assignments")
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

// getAppGroups will return the list of groups an application belongs to.
func (w *wrappedClient) getAppGroups(ctx context.Context, appID string) ([]string, error) {
	var groupIDs []string

	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-groups-assigned-to-application
	groups, resp, err := w.client.Application.ListApplicationGroupAssignments(ctx, appID, query.NewQueryParams(
		query.WithLimit(200),
	))

	for {
		if err != nil {
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application groups")
		}

		for _, group := range groups {
			groupIDs = append(groupIDs, group.Id)
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groups)
	}

	return groupIDs, nil
}

// listUsers will return a mapping of usernames to user IDs from Okta.
func (w *wrappedClient) listUsers(ctx context.Context) (map[string]string, error) {
	usernameToUserID := map[string]string{}

	err := w.iterateUsers(ctx, func(user *okta.User) error {
		profile := user.Profile
		if profile != nil {
			usernameToUserID[fmt.Sprintf("%s", (*profile)[oktaUserProfileLogin])] = user.Id
		}
		return nil
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return usernameToUserID, nil
}

// assignUserToGroup will assign the given user to the group.
func (w *wrappedClient) assignUserToGroup(ctx context.Context, userID, groupId string) error {
	if _, err := w.client.Group.AddUserToGroup(ctx, groupId, userID); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// unassignUserFromGroup will unassign the given user from the group.
func (w *wrappedClient) unassignUserFromGroup(ctx context.Context, userID, groupId string) error {
	if _, err := w.client.Group.RemoveUserFromGroup(ctx, groupId, userID); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// assignUserToApplication will assign the given user to the application.
func (w *wrappedClient) assignUserToApplication(ctx context.Context, username, applicationId string) error {
	user, _, err := w.client.User.GetUser(ctx, username)
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	appUser := okta.AppUser{
		Id: user.Id,
	}
	if _, _, err := w.client.Application.AssignUserToApplication(ctx, applicationId, appUser); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// assignGroupToApplicationByID assigns the given group to the given application
// using the OktaGroupID, rather than the group name as in other methods.
func (w *wrappedClient) assignGroupToApplicationByID(ctx context.Context, groupId, applicationId string) error {
	body := okta.ApplicationGroupAssignment{}
	if _, _, err := w.client.Application.CreateApplicationGroupAssignment(ctx, applicationId, groupId, body); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// unassignUserFromApplication will unassign the given user from the application.
func (w *wrappedClient) unassignUserFromApplication(ctx context.Context, username, applicationId string) error {
	user, _, err := w.client.User.GetUser(ctx, username)
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	// Unlike groups, deleting a non-existent application user will produce an error from the Okta API.
	if _, err := w.client.Application.DeleteApplicationUser(ctx, applicationId, user.Id, query.NewQueryParams()); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

func (w *wrappedClient) createApplication(ctx context.Context, application okta.App) (okta.App, error) {
	app, _, err := w.client.Application.CreateApplication(ctx, application, nil)
	if err != nil {
		return nil, w.oktaErrToTrace(ctx, err)
	}
	return app, nil
}

func (w *wrappedClient) orgName(ctx context.Context) (string, error) {
	settings, _, err := w.client.OrgSetting.GetOrgSettings(ctx)
	if err != nil {
		return "", w.oktaErrToTrace(ctx, err)
	}
	return settings.CompanyName, nil
}

// getOrgURL will return the org URL for the client.
func (w *wrappedClient) orgURL() string {
	return w.oktaOrgURL
}

func (w *wrappedClient) doHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	requester := w.client.CloneRequestExecutor()
	for _, contentType := range accept {
		requester.WithAccept(contentType)
	}

	req, err := requester.NewRequest(method, "", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.URL = url

	resp, err := requester.Do(ctx, req, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize); err != nil {
		return nil, trace.Wrap(err)
	} else {
		return body, nil
	}
}

const (
	// Okta error constants are not housed within the SDK, so we'll need to refer to the
	// documentation directly and define our own..
	// https://developer.okta.com/docs/reference/error-codes/
	oktaErrCodeAPIValidationException        = "E0000001"
	oktaErrCodeAuthenticationException       = "E0000004"
	oktaErrCodeInvalidSessionException       = "E0000005"
	oktaErrCodeAccessDeniedException         = "E0000006"
	oktaErrCodeResourceNotFoundException     = "E0000007"
	oktaErrCodeNotFoundException             = "E0000008"
	oktaErrCodeInvalidTokenProvidedException = "E0000011"
)

// oktaAPIValidationError is a validation error.
type oktaAPIValidationError struct {
	errorID string
	summary string
}

func (o oktaAPIValidationError) Error() string {
	return fmt.Sprintf("%s: %s", o.errorID, o.summary)
}

// oktaErrToTrace takes Okta errors and converts them into appropriate trace equivalents.
func (w *wrappedClient) oktaErrToTrace(ctx context.Context, err error) error {
	oktaErr, ok := err.(*okta.Error)

	// If this is not an Okta error, just wrap the error and return it.
	if !ok {
		return trace.Wrap(err)
	}

	switch oktaErr.ErrorCode {
	case oktaErrCodeAuthenticationException, oktaErrCodeInvalidSessionException, oktaErrCodeInvalidTokenProvidedException:
		reportPluginStatus(ctx, w.log, w.pluginStatusSink, types.PluginStatusCode_UNAUTHORIZED)
		return trace.WithField(trace.AccessDenied(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	case oktaErrCodeAccessDeniedException:
		return trace.WithField(trace.AccessDenied(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	case oktaErrCodeResourceNotFoundException, oktaErrCodeNotFoundException:
		return trace.WithField(trace.NotFound(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	case oktaErrCodeAPIValidationException:
		return &oktaAPIValidationError{errorID: oktaErr.ErrorId, summary: oktaErr.ErrorSummary}
	default:
		// If we don't have a more specific error to provide, just wrap the error and return it.
		return trace.WithField(trace.BadParameter(oktaErr.ErrorSummary), oktaErrorID, oktaErr.ErrorId)
	}
}
