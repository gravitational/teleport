package okta

import (
	"context"
	"errors"
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

// Static assertion that the wrappedClient type implements OktaClient
var _ OktaClient = (*wrappedClient)(nil)

// wrappedClient is a wrapper around an Okta SDK client that provides
// higher-level operations and for interacting with an Okta server
// over using the basic SDK client (e.g. result pagination)
type wrappedClient struct {
	log              *logrus.Entry
	client           *okta.Client
	oktaOrgURL       string
	pluginStatusSink common.StatusSink
}

// getCurrentUser fetches the Okta profile of the user represented by the API token.
func (w *wrappedClient) getCurrentUser(ctx context.Context) (*okta.User, error) {
	// Fetch the current Okta user, as per
	//   https://developer.okta.com/docs/reference/api/users/#get-current-user
	me, _, err := w.client.User.GetUser(ctx, "me")
	if err != nil {
		return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error getting current user")
	}
	return me, nil
}

// iterateUsers iterates over all users in the Okta system, invoking the
// supplied function for every user. The user callback may return the
// `stopIteration` error to signal that it doesn't want any more users.
func (w *wrappedClient) iterateUsers(ctx context.Context, fn func(*okta.User) error, paramsOpt ...query.ParamOptions) error {
	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/users/#list-users
	paramsOpt = append(paramsOpt, query.WithLimit(200))
	users, resp, err := w.client.User.ListUsers(ctx, query.NewQueryParams(paramsOpt...))

	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error while iterating over okta users")
		}

		for _, user := range users {
			if err = fn(user); err != nil {
				if errors.Is(err, errStopIteration) {
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
				if errors.Is(err, errStopIteration) {
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
				if errors.Is(err, errStopIteration) {
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

// iterateAppUsers will iterate over the list of all Okta users assigned to
// a given app. The supplied iterator callback may return stopIteration to
// signal that it does not want to continue receiving users. All other
// non-nil return values are considered an error and will be propagated to
// the caller.
func (w *wrappedClient) iterateAppUsers(ctx context.Context, appID oktaAppID, fn func(*okta.AppUser) error) error {

	// We'll use the max page size of 500 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-users-assigned-to-application
	appUsers, resp, err := w.client.Application.ListApplicationUsers(ctx, string(appID), query.NewQueryParams(
		query.WithLimit(500),
	))

	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application user assignments")
		}

		for _, appUser := range appUsers {
			if err := fn(appUser); err != nil {
				if errors.Is(err, errStopIteration) {
					break
				}
				return trace.Wrap(err)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &appUsers)
	}

	return nil
}

// getGroupAssignments will return the list of users assigned to a group.
func (w *wrappedClient) getGroupAssignments(ctx context.Context, groupID oktaGroupID) ([]oktaUserID, error) {
	var userIDs []oktaUserID

	// The default number of users here is 1000, which will be fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-group-members
	groupUsers, resp, err := w.client.Group.ListGroupUsers(ctx, string(groupID), query.NewQueryParams())

	for {
		if err != nil {
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting group user assignments")
		}

		for _, groupUser := range groupUsers {
			userIDs = append(userIDs, oktaUserID(groupUser.Id))
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groupUsers)
	}

	return userIDs, nil
}

// getAppAssignments will return the list of users assigned to an app.
func (w *wrappedClient) getAppAssignments(ctx context.Context, appID oktaAppID) ([]appAssignment, error) {
	var assignments []appAssignment
	err := w.iterateAppUsers(ctx, appID, func(appUser *okta.AppUser) error {
		assignments = append(assignments, appAssignment{
			userID: appUser.Id,
			scope:  appAssignmentScope(appUser.Scope),
		})
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return assignments, nil
}

// getAppGroups will return the list of groups an application belongs to.
func (w *wrappedClient) getAppGroups(ctx context.Context, appID oktaAppID) ([]oktaGroupID, error) {
	var groupIDs []oktaGroupID

	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-groups-assigned-to-application
	groups, resp, err := w.client.Application.ListApplicationGroupAssignments(ctx, string(appID), query.NewQueryParams(
		query.WithLimit(200),
	))

	for {
		if err != nil {
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application groups")
		}

		for _, group := range groups {
			groupIDs = append(groupIDs, oktaGroupID(group.Id))
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groups)
	}

	return groupIDs, nil
}

// listUsers will return a mapping of usernames to user IDs from Okta.
func (w *wrappedClient) listUsers(ctx context.Context, paramOpt ...query.ParamOptions) (map[userName]oktaUserID, error) {
	usernameToUserID := map[userName]oktaUserID{}
	err := w.iterateUsers(ctx, func(user *okta.User) error {
		log := w.log.WithField("okta_user_id", user.Id)

		profile := user.Profile
		if profile == nil {
			log.Warn("Malformed Okta user: missing user profile")
			return nil
		}

		login, ok := (*profile)[oktaUserProfileLogin]
		if !ok {
			log.Warn("Malformed Okta user profile: missing " + oktaUserProfileLogin)
			return nil
		}

		// Use Sprintf to coerce the "login" profile attribute into a
		// string. It _should_ already be one, but the Okta profile
		// gives no such guarantee.
		n := fmt.Sprintf("%s", login)
		usernameToUserID[userName(n)] = oktaUserID(user.Id)
		return nil
	}, paramOpt...)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return usernameToUserID, nil
}

// assignUserToGroup will assign the given user to the group.
func (w *wrappedClient) assignUserToGroup(ctx context.Context, userID oktaUserID, groupId oktaGroupID) error {
	if _, err := w.client.Group.AddUserToGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// unassignUserFromGroup will unassign the given user from the group.
func (w *wrappedClient) unassignUserFromGroup(ctx context.Context, userID oktaUserID, groupId oktaGroupID) error {
	if _, err := w.client.Group.RemoveUserFromGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// assignUserToApplication will assign the given user to the application.
func (w *wrappedClient) assignUserToApplication(ctx context.Context, userID oktaUserID, applicationId oktaAppID) error {
	user, _, err := w.client.User.GetUser(ctx, string(userID))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	appUser := okta.AppUser{
		Id: user.Id,
	}
	if _, _, err := w.client.Application.AssignUserToApplication(ctx, string(applicationId), appUser); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// assignGroupToApplicationByID assigns the given group to the given application
func (w *wrappedClient) assignGroupToApplication(ctx context.Context, groupId oktaGroupID, applicationId oktaAppID) error {
	body := okta.ApplicationGroupAssignment{}
	if _, _, err := w.client.Application.CreateApplicationGroupAssignment(ctx, string(applicationId), string(groupId), body); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// unassignUserFromApplication will unassign the given user from the application.
func (w *wrappedClient) unassignUserFromApplication(ctx context.Context, userId oktaUserID, applicationId oktaAppID) error {
	user, _, err := w.client.User.GetUser(ctx, string(userId))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	// Unlike groups, deleting a non-existent application user will produce an error from the Okta API.
	if _, err := w.client.Application.DeleteApplicationUser(ctx, string(applicationId), user.Id, query.NewQueryParams()); err != nil {
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

// getApplication fetches the data for a single application, by ID
func (w *wrappedClient) getApplication(ctx context.Context, appID oktaAppID, appType okta.App) (okta.App, error) {
	app, _, err := w.client.Application.GetApplication(ctx, string(appID), appType, nil)
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
	var oktaErr *okta.Error
	// If this is not an Okta error, just wrap the error and return it.
	if !errors.As(err, &oktaErr) {
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

// TestCredentials validates the credentials used by the supplied client by
// querying the current user profile.
func TestCredentials(ctx context.Context, client OktaClient) error {
	_, err := client.getCurrentUser(ctx)
	if err != nil {
		return trace.Wrap(err, "testing Okta credentials")
	}
	return nil
}
