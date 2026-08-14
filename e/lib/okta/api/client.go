package oktaapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log"
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
	// oktaHTTPClientTimeout limits the total time for a single HTTP round-trip to the Okta API.
	// Some requests can take more than 30s, e.g. /api/v1/apps/{appID}/users?limit=500
	// (For instance in our Okta Prod Ashby or Anthropic apps)
	// To avoid hitting the HTTP client timeout, we set it to 5 minutes
	// TODO(smallinsky) switch  from apps/{appID}/users to semi official apps/{appID}/skinny_users skinny users app API
	// that allows list list app users instantly.
	// See: https://support.okta.com/help/s/article/efficiently-retrieve-user-lists-using-skinny-users-endpoints
	oktaHTTPClientTimeout = 5 * time.Minute
)

// Interface provides higher-level operations on underlying Okta SDK API client. It's implemented
// by [Client].
//
// TODO(kopiczko) get rid of the interface and test only with mocking [APIClient]
type Interface interface {
	// GetOrgUrl will return the org URL for the client.
	GetOrgUrl() string

	// GetAuthorizedScopes verifies and returns the list of configured OAuth scopes trimmed
	// down to those allowed by the configured credentials.
	GetAuthorizedScopes(ctx context.Context) ([]string, error)
	// GetCurrentUser will fetch the profile of the user currently logged into
	// Okta.
	GetCurrentUser(context.Context) (*okta.User, error)
	// IterateUsers will iterate over the list of all Okta users. The supplied
	// iterator callback may return ErrStopIteration to signal that it does not want
	// to continue receiving users. All other non-nil return values are
	// considered an error and will be propagated to the caller.
	IterateUsers(context.Context, func(*okta.User) error, ...query.ParamOptions) error
	// ListUserGroups will return the list of groups a user belongs to.
	ListUserGroups(ctx context.Context, userID string) ([]UserGroup, error)
	// IterateAppUsers will iterate over the list of all Okta users assigned to
	// a given app. The supplied iterator callback may return stopIteration to
	// signal that it does not want to continue receiving users. All other
	// non-nil return values are considered an error and will be propagated to
	// the caller.
	IterateAppUsers(context.Context, OktaAppID, func(*okta.AppUser) error) error
	// IterateGroups will iterate over the list of all Okta groups. The supplied
	// iterator callback may return ErrStopIteration to signal that it does not want
	// to continue receiving groups. All other non-nil return values are
	// considered an error and will be propagated to the caller.
	IterateGroups(context.Context, func(*okta.Group) error) error
	// IterateApps will iterate over the list of all Okta applications. The
	// supplied iterator callback may return ErrStopIteration to signal that it
	// does not want to continue receiving apps. All other non-nil return values
	// are considered an error and will be propagated to the caller.
	IterateApps(context.Context, func(okta.App) error, ...query.ParamOptions) error
	// GetGroupAssignments will return the list of users assigned to a group.
	GetGroupAssignments(ctx context.Context, groupID OktaGroupID) ([]OktaUserID, error)
	// GetAppAssignments will return the list of users assigned to an app.
	GetAppAssignments(ctx context.Context, appID OktaAppID) ([]AppAssignment, error)
	// GetAppGroups will return the list of groups an application belongs to.
	GetAppGroups(ctx context.Context, appID OktaAppID) ([]OktaGroupID, error)
	// ListUsers will return a mapping of usernames to user IDs from Okta.
	ListUsers(ctx context.Context, paramOpts ...query.ParamOptions) (map[UserName]OktaUserID, error)
	// AssignUserToGroup will assign the given user to the group.
	AssignUserToGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error
	// UnassignUserFromGroup will unassign the given user from the group.
	UnassignUserFromGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error
	// AssignUserToApplication will assign the given user to the application.
	AssignUserToApplication(ctx context.Context, userID OktaUserID, applicationId OktaAppID) error
	// AssignGroupToApplication assigns the given group to the application.
	AssignGroupToApplication(ctx context.Context, groupID OktaGroupID, applicationID OktaAppID) error
	// UnassignUserFromApplication will unassign the given user from the application.
	UnassignUserFromApplication(ctx context.Context, userID OktaUserID, applicationId OktaAppID) error
	// CreateApplication attempts to create a new Okta application from the
	// supplied application request.
	CreateApplication(ctx context.Context, application okta.App) (okta.App, error)
	// GetApplication fetches the data for single application.
	GetApplication(ctx context.Context, appID OktaAppID, appType okta.App) (okta.App, error)
	// OrgName returns the configured org name.
	// TODO(kopiczko) Remove when cleaning up the legacy connector creation code.
	OrgName(context.Context) (string, error)
	// DoHttp executes an HTTP request on the supplied URL using the same credentials and
	// headers used by underlying Okta client. Mainly used for retrieving SAML connector
	// metadata.
	DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error)
	// ListLogEvents will return a list of Okta log events.
	ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error)
	// ListApiTokens will return a list of Okta API tokens.
	ListApiTokens(ctx context.Context, qp *query.Params) ([]*ApiToken, *okta.Response, error)
	// ListUsersWithRoleAssignments will return a list of Okta users with role assignments.
	ListUsersWithRoleAssignments(ctx context.Context) (*RoleAssignedUsers, *okta.Response, error)
	// ListAssignedRolesForUser will return a list of Okta roles assigned to a user.
	ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error)
}

type Config struct {
	// Log receives any log info
	Log *slog.Logger
	// OrgUrl is a URL indicating the root endpoint of the Okta API service
	OrgUrl string
	// Scopes is a list of scopes to use for the Okta client.
	Scopes []string
	// Oauth is an optional OAuth configuration for the Okta client.
	AuthProvider AuthProvider
	// TestHTTPClient is an optional HTTP client that can be used to override the
	// default client for testing. Do not set in production.
	TestHTTPClient *http.Client
}

// Check validates the state of the ClientConfig, returning a non-nil error
// if the config is invalid.
func (cfg *Config) Check() error {
	cfg.Log = createOrSetupLogger(cfg.Log)
	if cfg.OrgUrl == "" {
		return trace.BadParameter("missing Okta org URL")
	}
	if cfg.Scopes == nil {
		cfg.Scopes = oktaAPIScopes
	}
	if cfg.AuthProvider == nil {
		return trace.BadParameter("missing Okta AuthProvider")
	}
	if cfg.TestHTTPClient == nil {
		cfg.TestHTTPClient = &http.Client{
			Transport: &rateLimitingHTTPTransport{
				// This transport was taken from the Okta client.
				delegate: &http.Transport{
					IdleConnTimeout: oktaTransportIdleTimeout,
				},
				rateLimiter: rate.NewLimiter(
					rate.Every(time.Second/time.Duration(APICallsPerSecond)), 1),
			},
			Timeout: oktaHTTPClientTimeout,
		}
	}
	return nil
}

func createOrSetupLogger(l *slog.Logger) *slog.Logger {
	if l == nil {
		return log.NewPackageLogger(teleport.ComponentKey, eteleport.ComponentOktaClient)
	}
	return l.With(teleport.ComponentKey, eteleport.ComponentOktaClient)
}

// New creates and initializes a new Okta client implementing the [Interface].
func New(ctx context.Context, cfg Config) (Interface, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	settings := []okta.ConfigSetter{
		okta.WithCache(false), // We don't want a cache as we need up to date info.
		okta.WithOrgUrl(cfg.OrgUrl),
		okta.WithHttpClientPtr(cfg.TestHTTPClient),
		// This will retry until the request timeout has passed, doing a backoff
		// of up to 30 seconds.
		okta.WithRequestTimeout(RequestTimeoutSeconds),
		okta.WithRateLimitMaxRetries(math.MaxInt32),
		okta.WithScopes(cfg.Scopes),
	}

	settings = append(settings, cfg.AuthProvider.GetAuthOptions()...)
	_, client, err := okta.NewClient(ctx, settings...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		Log:       cfg.Log,
		APIClient: NewAPIClient(client),
	}, nil
}

// NewForAPIClient creates new Client for the given [APIClient]. It's useful for testing. [New]
// should be used in the production code.
func NewForAPIClient(apiClient APIClient) *Client {
	return &Client{
		Log:       createOrSetupLogger(nil),
		APIClient: apiClient,
	}
}

// ErrStopIteration is a sentinel value that iterator functions can use to
// signals oktaClient iterate* methods to stop iterating without it
// being passed up the call stack.
var ErrStopIteration = errors.New("stop iterating")

// Static assertion that the wrappedClient type implements OktaClient
var _ Interface = (*Client)(nil)

// Client is the default [Interface] implementation.
type Client struct {
	Log       *slog.Logger
	APIClient APIClient
}

// GetCurrentUser fetches the Okta profile of the user represented by the API token.
func (w *Client) GetCurrentUser(ctx context.Context) (*okta.User, error) {
	// Fetch the current Okta user, as per
	//   https://developer.okta.com/docs/reference/api/users/#get-current-user
	me, _, err := w.APIClient.GetUser(ctx, "me")
	if err != nil {
		return nil, trace.Wrap(w.oktaErrToTrace(ctx, err), "error getting current user")
	}
	return me, nil
}

// IterateUsers iterates over all users in the Okta system, invoking the
// supplied function for every user. The user callback may return the
// `stopIteration` error to signal that it doesn't want any more users.
func (w *Client) IterateUsers(ctx context.Context, fn func(*okta.User) error, paramsOpt ...query.ParamOptions) error {
	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/users/#list-users
	paramsOpt = append(paramsOpt, query.WithLimit(200))
	users, resp, err := w.APIClient.ListUsers(ctx, query.NewQueryParams(paramsOpt...))

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
func (w *Client) ListUserGroups(ctx context.Context, userID string) ([]UserGroup, error) {
	groups, _, err := w.APIClient.ListUserGroups(ctx, userID)
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
func (w *Client) IterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	// The default page size is 10000 here, but that seems to be beyond what the HTTP client built
	// into the go Okta client can handle, so I'm limiting it to 200.
	// https://developer.okta.com/docs/reference/api/groups/#list-groups
	oktaGroups, resp, err := w.APIClient.ListGroups(ctx, query.NewQueryParams(
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
func (w *Client) IterateApps(ctx context.Context, fn func(okta.App) error, paramOpt ...query.ParamOptions) error {
	// The default for application listing is 20 per page. Here we'll bump it
	// to the max of 200 per page to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-applications
	paramOpt = append(paramOpt, query.WithLimit(200))
	oktaApps, resp, err := w.APIClient.ListApplications(ctx, query.NewQueryParams(paramOpt...))
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
func (w *Client) IterateAppUsers(ctx context.Context, appID OktaAppID, fn func(*okta.AppUser) error) error {

	// We'll use the max page size of 500 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-users-assigned-to-application
	appUsers, resp, err := w.APIClient.ListApplicationUsers(ctx, string(appID), query.NewQueryParams(
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
func (w *Client) GetGroupAssignments(ctx context.Context, groupID OktaGroupID) ([]OktaUserID, error) {
	var userIDs []OktaUserID

	// The default number of users here is 1000, which will be fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-group-members
	groupUsers, resp, err := w.APIClient.ListGroupUsers(ctx, string(groupID), query.NewQueryParams())

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
func (w *Client) GetAppAssignments(ctx context.Context, appID OktaAppID) ([]AppAssignment, error) {
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
func (w *Client) GetAppGroups(ctx context.Context, appID OktaAppID) ([]OktaGroupID, error) {
	var groupIDs []OktaGroupID

	// We'll use the max page size of 200 here to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-groups-assigned-to-application
	groups, resp, err := w.APIClient.ListApplicationGroupAssignments(ctx, string(appID), query.NewQueryParams(
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
func (w *Client) ListUsers(ctx context.Context, paramOpt ...query.ParamOptions) (map[UserName]OktaUserID, error) {
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
func (w *Client) AssignUserToGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error {
	if _, err := w.APIClient.AddUserToGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// UnassignUserFromGroup will unassign the given user from the group.
func (w *Client) UnassignUserFromGroup(ctx context.Context, userID OktaUserID, groupId OktaGroupID) error {
	if _, err := w.APIClient.RemoveUserFromGroup(ctx, string(groupId), string(userID)); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}
	return nil
}

// AssignUserToApplication will assign the given user to the application.
func (w *Client) AssignUserToApplication(ctx context.Context, userID OktaUserID, applicationId OktaAppID) error {
	user, _, err := w.APIClient.GetUser(ctx, string(userID))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	appUser := okta.AppUser{
		Id: user.Id,
	}
	if _, _, err := w.APIClient.AssignUserToApplication(ctx, string(applicationId), appUser); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// AssignGroupToApplication assigns the given group to the given application
func (w *Client) AssignGroupToApplication(ctx context.Context, groupId OktaGroupID, applicationId OktaAppID) error {
	body := okta.ApplicationGroupAssignment{}
	if _, _, err := w.APIClient.CreateApplicationGroupAssignment(ctx, string(applicationId), string(groupId), body); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// UnassignUserFromApplication will unassign the given user from the application.
func (w *Client) UnassignUserFromApplication(ctx context.Context, userId OktaUserID, applicationId OktaAppID) error {
	user, _, err := w.APIClient.GetUser(ctx, string(userId))
	if err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	// Unlike groups, deleting a non-existent application user will produce an error from the Okta API.
	if _, err := w.APIClient.DeleteApplicationUser(ctx, string(applicationId), user.Id, query.NewQueryParams()); err != nil {
		return w.oktaErrToTrace(ctx, err)
	}

	return nil
}

// CreateApplication attempts to create a new Okta application.
func (w *Client) CreateApplication(ctx context.Context, application okta.App) (okta.App, error) {
	app, _, err := w.APIClient.CreateApplication(ctx, application, nil)
	if err != nil {
		return nil, w.oktaErrToTrace(ctx, err)
	}
	return app, nil
}

// GetApplication fetches the data for a single application, by ID
func (w *Client) GetApplication(ctx context.Context, appID OktaAppID, appType okta.App) (okta.App, error) {
	app, _, err := w.APIClient.GetApplication(ctx, string(appID), appType, nil)
	if err != nil {
		return nil, w.oktaErrToTrace(ctx, err)
	}
	return app, nil
}

// OrgName returns the name of the organization.
func (w *Client) OrgName(ctx context.Context) (string, error) {
	settings, _, err := w.APIClient.GetOrgSettings(ctx)
	if err != nil {
		return "", w.oktaErrToTrace(ctx, err)
	}
	return settings.CompanyName, nil
}

// GetOrgUrl implements [Interface].
func (c *Client) GetOrgUrl() string {
	return c.APIClient.GetOrgUrl()
}

// CheckScopes implements [Interface].CheckScopes.
func (c *Client) GetAuthorizedScopes(ctx context.Context) ([]string, error) {
	scopes, err := c.APIClient.GetAuthorizedScopes(ctx)
	return scopes, trace.Wrap(err)
}

// DoHttp performs an HTTP request to the Okta API.
func (w *Client) DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	requester := w.APIClient.CloneRequestExecutor()
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
func (w *Client) oktaErrToTrace(_ context.Context, err error) error {
	var oktaErr *okta.Error
	// If this is not an Okta error, just wrap the error and return it.
	if !errors.As(err, &oktaErr) {
		return trace.Wrap(err)
	}

	switch oktaErr.ErrorCode {
	case OktaErrCodeAuthenticationException, OktaErrCodeInvalidSessionException, OktaErrCodeInvalidTokenProvidedException:
		return trace.WithField(trace.AccessDenied("%s", oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeAccessDeniedException:
		return trace.WithField(trace.AccessDenied("%s", oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeResourceNotFoundException, OktaErrCodeNotFoundException:
		return trace.WithField(trace.NotFound("%s", oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	case OktaErrCodeAPIValidationException:
		return &OktaAPIValidationError{ErrorID: oktaErr.ErrorId, Summary: oktaErr.ErrorSummary}
	default:
		// If we don't have a more specific error to provide, just wrap the error and return it.
		return trace.WithField(trace.BadParameter("%s", oktaErr.ErrorSummary), OktaErrorID, oktaErr.ErrorId)
	}
}

func (w *Client) ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error) {
	events, rsp, err := w.APIClient.ListLogEvents(ctx, qp)
	return events, rsp, trace.Wrap(err)
}
func (w *Client) ListApiTokens(ctx context.Context, qp *query.Params) ([]*ApiToken, *okta.Response, error) {
	tokens, rsp, err := w.APIClient.ListApiTokens(ctx, qp)
	return tokens, rsp, trace.Wrap(err)
}
func (w *Client) ListUsersWithRoleAssignments(ctx context.Context) (*RoleAssignedUsers, *okta.Response, error) {
	users, rsp, err := w.APIClient.ListUsersWithRoleAssignments(ctx)
	return users, rsp, trace.Wrap(err)
}

func (w *Client) ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error) {
	roles, rsp, err := w.APIClient.ListAssignedRolesForUser(ctx, userId)
	return roles, rsp, trace.Wrap(err)
}

// TestCredentials validates the credentials used by the supplied client by
// querying the current user profile.
func TestCredentials(ctx context.Context, client Interface) error {
	_, err := client.GetCurrentUser(ctx)
	if err != nil {
		return trace.Wrap(err, "testing Okta credentials")
	}
	return nil
}
