package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// OktaErrorID is the key used to store the Okta error ID in the trace.
	OktaErrorID = "errorId"
	// OktaUserProfileLogin is the key used to access the login attribute in the Okta user profile.
	OktaUserProfileLogin = "login"
)
const (

	// APICallsPerSecond defines 4 quests per second is the absolute maximum okta will allow due to End User Rate Limits.
	// https://developer.okta.com/docs/reference/rl-additional-limits/#end-user-rate-limits
	APICallsPerSecond     = 4
	RequestTimeoutSeconds = 300 // Okta request timeout is 5 minutes.

	// Default to running synchronizations every half hour.
	oktaTransportIdleTimeout = 30 * time.Second
	oktaConnectionTimeout    = 30 * time.Second
)

// NewClient creates and initializes a new okta client
func NewClient(ctx context.Context, cfg ClientConfig) (Client, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	settings := []okta.ConfigSetter{
		okta.WithCache(false), // We don't want a cache as we need up to date info.
		okta.WithOrgUrl(cfg.Endpoint),
		okta.WithHttpClientPtr(cfg.HTTPClient),
		// This will retry until the request timeout has passed, doing a backoff
		// of up to 30 seconds.
		okta.WithRequestTimeout(RequestTimeoutSeconds),
		okta.WithRateLimitMaxRetries(math.MaxInt32),
		okta.WithScopes(cfg.Scopes),
	}

	settings = append(settings, cfg.AuthProvider.GetAuthOptions()...)
	createFunc := getClientProvider()
	client, err := createFunc(ctx, settings...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &WrappedClient{
		Log:              cfg.Log,
		Client:           client,
		oktaOrgURL:       cfg.Endpoint,
		pluginStatusSink: cfg.StatusSink,
	}, nil
}

// ErrStopIteration is a sentinel value that iterator functions can use to
// signals oktaClient iterate* methods to stop iterating without it
// being passed up the call stack.
var ErrStopIteration = errors.New("stop iterating")

// AppAssignment an individual assignment to an application.
type AppAssignment struct {
	// UserID is the ID of the user assigned to the application.
	UserID string
	// Scope is the scope of the assignment.
	Scope AppAssignmentScope
}

// Static assertion that the wrappedClient type implements OktaClient
var _ Client = (*WrappedClient)(nil)

// WrappedClient is a wrapper around an Okta SDK client that provides
// higher-level operations and for interacting with an Okta server
// over using the basic SDK client (e.g. result pagination)
type WrappedClient struct {
	Log              *slog.Logger
	Client           OktaAPI
	oktaOrgURL       string
	pluginStatusSink common.StatusSink
}

// GetCurrentUser fetches the Okta profile of the user represented by the API token.
func (w *WrappedClient) GetCurrentUser(ctx context.Context) (*okta.User, error) {
	// Fetch the current Okta user, as per
	//   https://developer.okta.com/docs/reference/api/users/#get-current-user
	me, _, err := w.Client.GetUser(ctx, "me")
	if err != nil {
		return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error getting current user")
	}
	return me, nil
}

// IterateUsers iterates over all users in the Okta system, invoking the
// supplied function for every user. The user callback may return the
// `stopIteration` error to signal that it doesn't want any more users.
func (w *WrappedClient) IterateUsers(ctx context.Context, fn func(*okta.User) error, paramsOpt ...query.ParamOptions) error {
	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/users/#list-users
	paramsOpt = append(paramsOpt, query.WithLimit(200))
	users, resp, err := w.Client.ListUsers(ctx, query.NewQueryParams(paramsOpt...))

	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error while iterating over okta users")
		}

		for _, user := range users {
			if err = fn(user); err != nil {
				if errors.Is(err, ErrStopIteration) {
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

// ListUserGroups will return the list of groups a user belongs to.
func (w *WrappedClient) ListUserGroups(ctx context.Context, userID string) ([]UserGroup, error) {
	groups, _, err := w.Client.ListUserGroups(ctx, userID)
	if err != nil {
		return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error while iterating over okta user groups")
	}
	var out []UserGroup
	for _, v := range groups {
		out = append(out, UserGroup{
			ID:   v.Id,
			Name: v.Profile.Name,
		})
	}
	return out, nil
}

// IterateGroups will iterate over the list of all Okta groups, invoking the
// supplied function for every group record. The callback may return the
// `stopIteration` error to signal that it doesn't want any more groups.
func (w *WrappedClient) IterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	// The default page size is 10000 here, but that seems to be beyond what the HTTP client built
	// into the go Okta client can handle, so I'm limiting it to 200.
	// https://developer.okta.com/docs/reference/api/groups/#list-groups
	oktaGroups, resp, err := w.Client.ListGroups(ctx, query.NewQueryParams(
		query.WithLimit(200), // A smaller page size so that we don't blow up the go Okta HTTP client.
	))
	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when iterating through groups")
		}

		for _, oktaGroup := range oktaGroups {
			if err := fn(oktaGroup); err != nil {
				if errors.Is(err, ErrStopIteration) {
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

// IterateApps will iterate over the list of all Okta applications. The callback
// may return the `stopIteration` error to signal that it doesn't want any more
// apps.
func (w *WrappedClient) IterateApps(ctx context.Context, fn func(okta.App) error) error {
	// The default for application listing is 20 per page. Here we'll bump it
	// to the max of 200 per page to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-applications
	oktaApps, resp, err := w.Client.ListApplications(ctx, query.NewQueryParams(
		query.WithLimit(200), // Max size.
	))
	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when iterating through apps")
		}

		for _, oktaApp := range oktaApps {
			if err := fn(oktaApp); err != nil {
				if errors.Is(err, ErrStopIteration) {
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

// IterateAppUsers will iterate over the list of all Okta users assigned to
// a given app. The supplied iterator callback may return stopIteration to
// signal that it does not want to continue receiving users. All other
// non-nil return values are considered an error and will be propagated to
// the caller.
func (w *WrappedClient) IterateAppUsers(ctx context.Context, appID OktaAppID, fn func(*okta.AppUser) error) error {

	// We'll use the max page size of 500 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-users-assigned-to-application
	appUsers, resp, err := w.Client.ListApplicationUsers(ctx, string(appID), query.NewQueryParams(
		query.WithLimit(500),
	))

	for {
		if err != nil {
			return trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application user assignments")
		}

		for _, appUser := range appUsers {
			if err := fn(appUser); err != nil {
				if errors.Is(err, ErrStopIteration) {
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

// GetGroupAssignments will return the list of users assigned to a group.
func (w *WrappedClient) GetGroupAssignments(ctx context.Context, groupID OktaGroupID) ([]OktaUserID, error) {
	var userIDs []OktaUserID

	// The default number of users here is 1000, which will be fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-group-members
	groupUsers, resp, err := w.Client.ListGroupUsers(ctx, string(groupID), query.NewQueryParams())

	for {
		if err != nil {
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting group user assignments")
		}

		for _, groupUser := range groupUsers {
			userIDs = append(userIDs, OktaUserID(groupUser.Id))
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groupUsers)
	}

	return userIDs, nil
}

// GetAppAssignments will return the list of users assigned to an app.
func (w *WrappedClient) GetAppAssignments(ctx context.Context, appID OktaAppID) ([]AppAssignment, error) {
	var assignments []AppAssignment
	err := w.IterateAppUsers(ctx, appID, func(appUser *okta.AppUser) error {
		assignments = append(assignments, AppAssignment{
			UserID: appUser.Id,
			Scope:  AppAssignmentScope(appUser.Scope),
		})
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return assignments, nil
}

// GetAppGroups will return the list of groups an application belongs to.
func (w *WrappedClient) GetAppGroups(ctx context.Context, appID OktaAppID) ([]OktaGroupID, error) {
	var groupIDs []OktaGroupID

	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-groups-assigned-to-application
	groups, resp, err := w.Client.ListApplicationGroupAssignments(ctx, string(appID), query.NewQueryParams(
		query.WithLimit(200),
	))

	for {
		if err != nil {
			return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error when getting application groups")
		}

		for _, group := range groups {
			groupIDs = append(groupIDs, OktaGroupID(group.Id))
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &groups)
	}

	return groupIDs, nil
}

// ListUsers will return a mapping of usernames to user IDs from Okta.
func (w *WrappedClient) ListUsers(ctx context.Context, paramOpt ...query.ParamOptions) (map[UserName]OktaUserID, error) {
	usernameToUserID := map[UserName]OktaUserID{}
	err := w.IterateUsers(ctx, func(user *okta.User) error {
		log := w.Log.With("okta_user_id", user.Id)

		profile := user.Profile
		if profile == nil {
			log.WarnContext(ctx, "Malformed Okta user: missing user profile")
			return nil
		}

		login, ok := (*profile)[OktaUserProfileLogin]
		if !ok {
			log.With("missing", OktaUserProfileLogin).WarnContext(ctx, "Malformed Okta user profile")
			return nil
		}

		// Use Sprintf to coerce the "login" profile attribute into a
		// string. It _should_ already be one, but the Okta profile
		// gives no such guarantee.
		n := fmt.Sprintf("%s", login)
		usernameToUserID[UserName(n)] = OktaUserID(user.Id)
		return nil
	}, paramOpt...)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return usernameToUserID, nil
}

// AssignUserToGroup will assign the given user to the group.
func (w *WrappedClient) AssignUserToGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error {
	if _, err := w.Client.AddUserToGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// UnassignUserFromGroup will unassign the given user from the group.
func (w *WrappedClient) UnassignUserFromGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error {
	if _, err := w.Client.RemoveUserFromGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}
	return nil
}

// AssignUserToApplication will assign the given user to the application.
func (w *WrappedClient) AssignUserToApplication(ctx context.Context, userID OktaUserID, applicationId OktaAppID) error {
	user, _, err := w.Client.GetUser(ctx, string(userID))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	appUser := okta.AppUser{
		Id: user.Id,
	}
	if _, _, err := w.Client.AssignUserToApplication(ctx, string(applicationId), appUser); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// AssignGroupToApplication assigns the given group to the given application
func (w *WrappedClient) AssignGroupToApplication(ctx context.Context, groupId OktaGroupID, applicationId OktaAppID) error {
	body := okta.ApplicationGroupAssignment{}
	if _, _, err := w.Client.CreateApplicationGroupAssignment(ctx, string(applicationId), string(groupId), body); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// UnassignUserFromApplication will unassign the given user from the application.
func (w *WrappedClient) UnassignUserFromApplication(ctx context.Context, userId OktaUserID, applicationId OktaAppID) error {
	user, _, err := w.Client.GetUser(ctx, string(userId))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	// Unlike groups, deleting a non-existent application user will produce an error from the Okta API.
	if _, err := w.Client.DeleteApplicationUser(ctx, string(applicationId), user.Id, query.NewQueryParams()); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// CreateApplication attempts to create a new Okta application.
func (w *WrappedClient) CreateApplication(ctx context.Context, application okta.App) (okta.App, error) {
	app, _, err := w.Client.CreateApplication(ctx, application, nil)
	if err != nil {
		return nil, w.oktaErrToTrace(ctx, err)
	}
	return app, nil
}

// GetApplication fetches the data for a single application, by ID
func (w *WrappedClient) GetApplication(ctx context.Context, appID OktaAppID, appType okta.App) (okta.App, error) {
	app, _, err := w.Client.GetApplication(ctx, string(appID), appType, nil)
	if err != nil {
		return nil, w.oktaErrToTrace(ctx, err)
	}
	return app, nil
}

// OrgName returns the name of the organization.
func (w *WrappedClient) OrgName(ctx context.Context) (string, error) {
	settings, _, err := w.Client.GetOrgSettings(ctx)
	if err != nil {
		return "", w.oktaErrToTrace(ctx, err)
	}
	return settings.CompanyName, nil
}

// OrgURL will return the org URL for the client.
func (w *WrappedClient) OrgURL() string {
	return w.oktaOrgURL
}

// GetScopes returns the scopes that the client is authorized to access.
func (w *WrappedClient) GetScopes() []string {
	return w.Client.GetScopes()
}

// DoHttp performs an HTTP request to the Okta API.
func (w *WrappedClient) DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	requester := w.Client.CloneRequestExecutor()
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

// oktaErrToTrace takes Okta errors and converts them into appropriate trace equivalents.
func (w *WrappedClient) oktaErrToTrace(_ context.Context, err error) error {
	var oktaErr *okta.Error
	// If this is not an Okta error, just wrap the error and return it.
	if !errors.As(err, &oktaErr) {
		return trace.Wrap(err)
	}

	switch oktaErr.ErrorCode {
	case OktaErrCodeAuthenticationException, OktaErrCodeInvalidSessionException, OktaErrCodeInvalidTokenProvidedException:
		return trace.WithField(trace.AccessDenied(oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeAccessDeniedException:
		return trace.WithField(trace.AccessDenied(oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeResourceNotFoundException, OktaErrCodeNotFoundException:
		return trace.WithField(trace.NotFound(oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeAPIValidationException:
		return &OktaAPIValidationError{ErrorID: oktaErr.ErrorId, Summary: oktaErr.ErrorSummary}
	default:
		// If we don't have a more specific error to provide, just wrap the error and return it.
		return trace.WithField(trace.BadParameter(oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	}
}

// TestCredentials validates the credentials used by the supplied client by
// querying the current user profile.
func TestCredentials(ctx context.Context, client Client) error {
	_, err := client.GetCurrentUser(ctx)
	if err != nil {
		return trace.Wrap(err, "testing Okta credentials")
	}
	return nil
}
