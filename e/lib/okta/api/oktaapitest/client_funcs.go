package oktaapitest

import (
	context "context"
	url "net/url"
	"testing"

	okta "github.com/okta/okta-sdk-golang/v2/okta"
	query "github.com/okta/okta-sdk-golang/v2/okta/query"

	api "github.com/gravitational/teleport/e/lib/okta/api"
)

type ClientFuncs struct {
	GetCurrentUserFunc              func(t *testing.T, ctx context.Context) (*okta.User, error)
	IterateUsersFunc                func(t *testing.T, ctx context.Context, fn func(*okta.User) error, queryParams ...query.ParamOptions) error
	ListUserGroupsFunc              func(t *testing.T, ctx context.Context, userID string) ([]api.UserGroup, error)
	IterateAppUsersFunc             func(t *testing.T, ctx context.Context, appID api.OktaAppID, fn func(*okta.AppUser) error) error
	IterateGroupsFunc               func(t *testing.T, ctx context.Context, fn func(*okta.Group) error) error
	IterateAppsFunc                 func(t *testing.T, ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error
	GetGroupAssignmentsFunc         func(t *testing.T, ctx context.Context, groupID api.OktaGroupID) ([]api.OktaUserID, error)
	GetAppAssignmentsFunc           func(t *testing.T, ctx context.Context, appID api.OktaAppID) ([]api.AppAssignment, error)
	GetAppGroupsFunc                func(t *testing.T, ctx context.Context, appID api.OktaAppID) ([]api.OktaGroupID, error)
	ListUsersFunc                   func(t *testing.T, ctx context.Context, paramOpts ...query.ParamOptions) (map[api.UserName]api.OktaUserID, error)
	AssignUserToGroupFunc           func(t *testing.T, ctx context.Context, userID api.OktaUserID, groupId api.OktaGroupID) error
	UnassignUserFromGroupFunc       func(t *testing.T, ctx context.Context, userID api.OktaUserID, groupId api.OktaGroupID) error
	AssignUserToApplicationFunc     func(t *testing.T, ctx context.Context, userID api.OktaUserID, applicationId api.OktaAppID) error
	AssignGroupToApplicationFunc    func(t *testing.T, ctx context.Context, groupID api.OktaGroupID, applicationID api.OktaAppID) error
	UnassignUserFromApplicationFunc func(t *testing.T, ctx context.Context, userID api.OktaUserID, applicationId api.OktaAppID) error
	CreateApplicationFunc           func(t *testing.T, ctx context.Context, application okta.App) (okta.App, error)
	GetApplicationFunc              func(t *testing.T, ctx context.Context, appID api.OktaAppID, appType okta.App) (okta.App, error)
	OrgURLFunc                      func(t *testing.T) string
	OrgNameFunc                     func(t *testing.T, ctx context.Context) (string, error)
	DoHttpFunc                      func(t *testing.T, ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error)
	GetScopesFunc                   func(t *testing.T) []string
}
