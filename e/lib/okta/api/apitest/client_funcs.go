package oktaapitest

import (
	context "context"
	url "net/url"
	"testing" //nolint:depguard // this a shared test package

	okta "github.com/okta/okta-sdk-golang/v2/okta"
	query "github.com/okta/okta-sdk-golang/v2/okta/query"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
)

type ClientFuncs struct {
	GetCurrentUserFunc              func(t *testing.T, ctx context.Context) (*okta.User, error)
	IterateUsersFunc                func(t *testing.T, ctx context.Context, fn func(*okta.User) error, queryParams ...query.ParamOptions) error
	ListUserGroupsFunc              func(t *testing.T, ctx context.Context, userID string) ([]oktaapi.UserGroup, error)
	IterateAppUsersFunc             func(t *testing.T, ctx context.Context, appID oktaapi.OktaAppID, fn func(*okta.AppUser) error) error
	IterateGroupsFunc               func(t *testing.T, ctx context.Context, fn func(*okta.Group) error) error
	IterateAppsFunc                 func(t *testing.T, ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error
	GetGroupAssignmentsFunc         func(t *testing.T, ctx context.Context, groupID oktaapi.OktaGroupID) ([]oktaapi.OktaUserID, error)
	GetAppAssignmentsFunc           func(t *testing.T, ctx context.Context, appID oktaapi.OktaAppID) ([]oktaapi.AppAssignment, error)
	GetAppGroupsFunc                func(t *testing.T, ctx context.Context, appID oktaapi.OktaAppID) ([]oktaapi.OktaGroupID, error)
	ListUsersFunc                   func(t *testing.T, ctx context.Context, paramOpts ...query.ParamOptions) (map[oktaapi.UserName]oktaapi.OktaUserID, error)
	AssignUserToGroupFunc           func(t *testing.T, ctx context.Context, userID oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) error
	UnassignUserFromGroupFunc       func(t *testing.T, ctx context.Context, userID oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) error
	AssignUserToApplicationFunc     func(t *testing.T, ctx context.Context, userID oktaapi.OktaUserID, applicationId oktaapi.OktaAppID) error
	AssignGroupToApplicationFunc    func(t *testing.T, ctx context.Context, groupID oktaapi.OktaGroupID, applicationID oktaapi.OktaAppID) error
	UnassignUserFromApplicationFunc func(t *testing.T, ctx context.Context, userID oktaapi.OktaUserID, applicationId oktaapi.OktaAppID) error
	CreateApplicationFunc           func(t *testing.T, ctx context.Context, application okta.App) (okta.App, error)
	GetApplicationFunc              func(t *testing.T, ctx context.Context, appID oktaapi.OktaAppID, appType okta.App) (okta.App, error)
	OrgURLFunc                      func(t *testing.T) string
	OrgNameFunc                     func(t *testing.T, ctx context.Context) (string, error)
	DoHttpFunc                      func(t *testing.T, ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error)
	GetAuthorizedScopesFunc         func(t *testing.T, ctx context.Context) ([]string, error)
}
