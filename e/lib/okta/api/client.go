package api

import (
	"context"
	"net/url"

	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
)

// Client is an Okta client interface that can be mocked for testing.
type Client interface {
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
	// OrgURL will return the org URL for the client.
	OrgURL() string
	// OrgName returns the configured
	OrgName(context.Context) (string, error)
	// DoHttp executes an HTTP request on the supplied URL using the same
	// credentials and headers used by underlying Okta client
	DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error)
	// GetScopes returns the scopes that the client is configured to use.
	GetScopes() []string
}

// CreateNewOktaClient will create a new Okta client.
func CreateNewOktaClient(ctx context.Context, config ClientConfig) (Client, error) {
	return NewClient(ctx, config)
}
