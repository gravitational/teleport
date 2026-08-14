package oktaapitest

import (
	context "context"
	url "net/url"
	"testing" //nolint:depguard // this a shared test package

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
)

var _ oktaapi.Interface = (*Client)(nil)

type Client struct {
	t *testing.T
	ClientFuncs
}

func NewClient(t *testing.T, funcs ClientFuncs) *Client {
	return &Client{
		t:           t,
		ClientFuncs: funcs,
	}
}

func NewOrgURLOnlyClient(t *testing.T) *Client {
	return &Client{t, ClientFuncs{
		OrgURLFunc: func(_ *testing.T) string { return TestOrgURL },
	}}
}

func NewLocalDataClient(t *testing.T) (*Client, *LocalData) {
	data := newLocalData()
	client := NewClient(t, data.newClientFuncs())
	return client, data
}

// GetCurrentUser implements [[oktaapi.Interface]].
func (c *Client) GetCurrentUser(ctx context.Context) (*okta.User, error) {
	if c.GetCurrentUserFunc != nil {
		return c.GetCurrentUserFunc(c.t, ctx)
	}
	panic("Client.GetCurrentUser not implemented")
}

// IterateUsers implements [[oktaapi.Interface]].
func (c *Client) IterateUsers(ctx context.Context, fn func(*okta.User) error, queryParams ...query.ParamOptions) error {
	if c.IterateUsersFunc != nil {
		return c.IterateUsersFunc(c.t, ctx, fn, queryParams...)
	}
	panic("Client.IterateUsers not implemented")
}

// ListUserGroups implements [[oktaapi.Interface]].
func (c *Client) ListUserGroups(ctx context.Context, userID string) ([]oktaapi.UserGroup, error) {
	if c.ListUserGroupsFunc != nil {
		return c.ListUserGroupsFunc(c.t, ctx, userID)
	}
	panic("Client.ListUserGroups not implemented")
}

// IterateAppUsers implements [[oktaapi.Interface]].
func (c *Client) IterateAppUsers(ctx context.Context, appID oktaapi.OktaAppID, fn func(*okta.AppUser) error) error {
	if c.IterateAppUsersFunc != nil {
		return c.IterateAppUsersFunc(c.t, ctx, appID, fn)
	}
	panic("Client.IterateAppUsers not implemented")
}

// IterateGroups implements [[oktaapi.Interface]].
func (c *Client) IterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	if c.IterateGroupsFunc != nil {
		return c.IterateGroupsFunc(c.t, ctx, fn)
	}
	panic("Client.IterateGroups not implemented")
}

// IterateApps implements [[oktaapi.Interface]].
func (c *Client) IterateApps(ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
	if c.IterateAppsFunc != nil {
		return c.IterateAppsFunc(c.t, ctx, fn, queryParams...)
	}
	panic("Client.IterateApps not implemented")
}

// GetGroupAssignments implements [[oktaapi.Interface]].
func (c *Client) GetGroupAssignments(ctx context.Context, groupID oktaapi.OktaGroupID) ([]oktaapi.OktaUserID, error) {
	if c.GetGroupAssignmentsFunc != nil {
		return c.GetGroupAssignmentsFunc(c.t, ctx, groupID)
	}
	panic("Client.GetGroupAssignments not implemented")
}

// GetAppAssignments implements [[oktaapi.Interface]].
func (c *Client) GetAppAssignments(ctx context.Context, appID oktaapi.OktaAppID) ([]oktaapi.AppAssignment, error) {
	if c.GetAppAssignmentsFunc != nil {
		return c.GetAppAssignmentsFunc(c.t, ctx, appID)
	}
	panic("Client.GetAppAssignments not implemented")
}

// GetAppGroups implements [[oktaapi.Interface]].
func (c *Client) GetAppGroups(ctx context.Context, appID oktaapi.OktaAppID) ([]oktaapi.OktaGroupID, error) {
	if c.GetAppGroupsFunc != nil {
		return c.GetAppGroupsFunc(c.t, ctx, appID)
	}
	panic("Client.GetAppGroups not implemented")
}

// ListUsers implements [[oktaapi.Interface]].
func (c *Client) ListUsers(ctx context.Context, paramOpts ...query.ParamOptions) (map[oktaapi.UserName]oktaapi.OktaUserID, error) {
	if c.ListUsersFunc != nil {
		return c.ListUsersFunc(c.t, ctx, paramOpts...)
	}
	panic("Client.ListUsers not implemented")
}

// AssignUserToGroup implements [[oktaapi.Interface]].
func (c *Client) AssignUserToGroup(ctx context.Context, userID oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) error {
	if c.AssignUserToGroupFunc != nil {
		return c.AssignUserToGroupFunc(c.t, ctx, userID, groupId)
	}
	panic("Client.AssignUserToGroup not implemented")
}

// UnassignUserFromGroup implements [[oktaapi.Interface]].
func (c *Client) UnassignUserFromGroup(ctx context.Context, userID oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) error {
	if c.UnassignUserFromGroupFunc != nil {
		return c.UnassignUserFromGroupFunc(c.t, ctx, userID, groupId)
	}
	panic("Client.UnassignUserFromGroup not implemented")
}

// AssignUserToApplication implements [[oktaapi.Interface]].
func (c *Client) AssignUserToApplication(ctx context.Context, userID oktaapi.OktaUserID, applicationId oktaapi.OktaAppID) error {
	if c.AssignUserToApplicationFunc != nil {
		return c.AssignUserToApplicationFunc(c.t, ctx, userID, applicationId)
	}
	panic("Client.AssignUserToApplication not implemented")
}

// AssignGroupToApplication implements [[oktaapi.Interface]].
func (c *Client) AssignGroupToApplication(ctx context.Context, groupID oktaapi.OktaGroupID, applicationID oktaapi.OktaAppID) error {
	if c.AssignGroupToApplicationFunc != nil {
		return c.AssignGroupToApplicationFunc(c.t, ctx, groupID, applicationID)
	}
	panic("Client.AssignGroupToApplication not implemented")
}

// UnassignUserFromApplication implements [[oktaapi.Interface]].
func (c *Client) UnassignUserFromApplication(ctx context.Context, userID oktaapi.OktaUserID, applicationId oktaapi.OktaAppID) error {
	if c.UnassignUserFromApplicationFunc != nil {
		return c.UnassignUserFromApplicationFunc(c.t, ctx, userID, applicationId)
	}
	panic("Client.UnassignUserFromApplication not implemented")
}

// CreateApplication implements [[oktaapi.Interface]].
func (c *Client) CreateApplication(ctx context.Context, application okta.App) (okta.App, error) {
	if c.CreateApplicationFunc != nil {
		return c.CreateApplicationFunc(c.t, ctx, application)
	}
	panic("Client.CreateApplication not implemented")
}

// GetApplication implements [[oktaapi.Interface]].
func (c *Client) GetApplication(ctx context.Context, appID oktaapi.OktaAppID, appType okta.App) (okta.App, error) {
	if c.GetApplicationFunc != nil {
		return c.GetApplicationFunc(c.t, ctx, appID, appType)
	}
	panic("Client.GetApplication not implemented")
}

// GetOrgUrl implements [[oktaapi.Interface]].
func (c *Client) GetOrgUrl() string {
	if c.OrgURLFunc != nil {
		return c.OrgURLFunc(c.t)
	}
	panic("Client.OrgURL not implemented")
}

// OrgName implements [[oktaapi.Interface]].
func (c *Client) OrgName(ctx context.Context) (string, error) {
	if c.OrgNameFunc != nil {
		return c.OrgNameFunc(c.t, ctx)
	}
	panic("Client.OrgName not implemented")
}

// DoHttp implements [[oktaapi.Interface]].
func (c *Client) DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	if c.DoHttpFunc != nil {
		return c.DoHttpFunc(c.t, ctx, method, url, accept)
	}
	panic("Client.DoHttp not implemented")
}

// GetAuthorizedScopes implements [[oktaapi.Interface]].
func (c *Client) GetAuthorizedScopes(ctx context.Context) ([]string, error) {
	if c.GetAuthorizedScopesFunc != nil {
		return c.GetAuthorizedScopesFunc(c.t, ctx)
	}
	panic("Client.GetAuthorizedScopesFunc not implemented")
}

func (w *Client) ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listLogEvents")
}
func (w *Client) ListApiTokens(ctx context.Context, qp *query.Params) ([]*oktaapi.ApiToken, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listApiTokens")
}
func (w *Client) ListUsersWithRoleAssignments(ctx context.Context) (*oktaapi.RoleAssignedUsers, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listUsersWithRoleAssignments")
}

func (w *Client) ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listAssignedRolesForUser")
}
