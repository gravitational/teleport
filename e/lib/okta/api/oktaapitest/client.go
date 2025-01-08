package oktaapitest

import (
	context "context"
	url "net/url"
	"testing"

	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport/e/lib/okta/api"
)

var _ api.Client = (*Client)(nil)

type Client struct {
	t *testing.T
	ClientFuncs
}

const TestOrgURL = "https://test.okta.example.com"

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

// GetCurrentUser implements [[api.Client]].
func (c *Client) GetCurrentUser(ctx context.Context) (*okta.User, error) {
	if c.GetCurrentUserFunc != nil {
		return c.GetCurrentUserFunc(c.t, ctx)
	}
	panic("Client.GetCurrentUser not implemented")
}

// IterateUsers implements [[api.Client]].
func (c *Client) IterateUsers(ctx context.Context, fn func(*okta.User) error, queryParams ...query.ParamOptions) error {
	if c.IterateUsersFunc != nil {
		return c.IterateUsersFunc(c.t, ctx, fn, queryParams...)
	}
	panic("Client.IterateUsers not implemented")
}

// ListUserGroups implements [[api.Client]].
func (c *Client) ListUserGroups(ctx context.Context, userID string) ([]api.UserGroup, error) {
	if c.ListUserGroupsFunc != nil {
		return c.ListUserGroupsFunc(c.t, ctx, userID)
	}
	panic("Client.ListUserGroups not implemented")
}

// IterateAppUsers implements [[api.Client]].
func (c *Client) IterateAppUsers(ctx context.Context, appID api.OktaAppID, fn func(*okta.AppUser) error) error {
	if c.IterateAppUsersFunc != nil {
		return c.IterateAppUsersFunc(c.t, ctx, appID, fn)
	}
	panic("Client.IterateAppUsers not implemented")
}

// IterateGroups implements [[api.Client]].
func (c *Client) IterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	if c.IterateGroupsFunc != nil {
		return c.IterateGroupsFunc(c.t, ctx, fn)
	}
	panic("Client.IterateGroups not implemented")
}

// IterateApps implements [[api.Client]].
func (c *Client) IterateApps(ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
	if c.IterateAppsFunc != nil {
		return c.IterateAppsFunc(c.t, ctx, fn, queryParams...)
	}
	panic("Client.IterateApps not implemented")
}

// GetGroupAssignments implements [[api.Client]].
func (c *Client) GetGroupAssignments(ctx context.Context, groupID api.OktaGroupID) ([]api.OktaUserID, error) {
	if c.GetGroupAssignmentsFunc != nil {
		return c.GetGroupAssignmentsFunc(c.t, ctx, groupID)
	}
	panic("Client.GetGroupAssignments not implemented")
}

// GetAppAssignments implements [[api.Client]].
func (c *Client) GetAppAssignments(ctx context.Context, appID api.OktaAppID) ([]api.AppAssignment, error) {
	if c.GetAppAssignmentsFunc != nil {
		return c.GetAppAssignmentsFunc(c.t, ctx, appID)
	}
	panic("Client.GetAppAssignments not implemented")
}

// GetAppGroups implements [[api.Client]].
func (c *Client) GetAppGroups(ctx context.Context, appID api.OktaAppID) ([]api.OktaGroupID, error) {
	if c.GetAppGroupsFunc != nil {
		return c.GetAppGroupsFunc(c.t, ctx, appID)
	}
	panic("Client.GetAppGroups not implemented")
}

// ListUsers implements [[api.Client]].
func (c *Client) ListUsers(ctx context.Context, paramOpts ...query.ParamOptions) (map[api.UserName]api.OktaUserID, error) {
	if c.ListUsersFunc != nil {
		return c.ListUsersFunc(c.t, ctx, paramOpts...)
	}
	panic("Client.ListUsers not implemented")
}

// AssignUserToGroup implements [[api.Client]].
func (c *Client) AssignUserToGroup(ctx context.Context, userID api.OktaUserID, groupId api.OktaGroupID) error {
	if c.AssignUserToGroupFunc != nil {
		return c.AssignUserToGroupFunc(c.t, ctx, userID, groupId)
	}
	panic("Client.AssignUserToGroup not implemented")
}

// UnassignUserFromGroup implements [[api.Client]].
func (c *Client) UnassignUserFromGroup(ctx context.Context, userID api.OktaUserID, groupId api.OktaGroupID) error {
	if c.UnassignUserFromGroupFunc != nil {
		return c.UnassignUserFromGroupFunc(c.t, ctx, userID, groupId)
	}
	panic("Client.UnassignUserFromGroup not implemented")
}

// AssignUserToApplication implements [[api.Client]].
func (c *Client) AssignUserToApplication(ctx context.Context, userID api.OktaUserID, applicationId api.OktaAppID) error {
	if c.AssignUserToApplicationFunc != nil {
		return c.AssignUserToApplicationFunc(c.t, ctx, userID, applicationId)
	}
	panic("Client.AssignUserToApplication not implemented")
}

// AssignGroupToApplication implements [[api.Client]].
func (c *Client) AssignGroupToApplication(ctx context.Context, groupID api.OktaGroupID, applicationID api.OktaAppID) error {
	if c.AssignGroupToApplicationFunc != nil {
		return c.AssignGroupToApplicationFunc(c.t, ctx, groupID, applicationID)
	}
	panic("Client.AssignGroupToApplication not implemented")
}

// UnassignUserFromApplication implements [[api.Client]].
func (c *Client) UnassignUserFromApplication(ctx context.Context, userID api.OktaUserID, applicationId api.OktaAppID) error {
	if c.UnassignUserFromApplicationFunc != nil {
		return c.UnassignUserFromApplicationFunc(c.t, ctx, userID, applicationId)
	}
	panic("Client.UnassignUserFromApplication not implemented")
}

// CreateApplication implements [[api.Client]].
func (c *Client) CreateApplication(ctx context.Context, application okta.App) (okta.App, error) {
	if c.CreateApplicationFunc != nil {
		return c.CreateApplicationFunc(c.t, ctx, application)
	}
	panic("Client.CreateApplication not implemented")
}

// GetApplication implements [[api.Client]].
func (c *Client) GetApplication(ctx context.Context, appID api.OktaAppID, appType okta.App) (okta.App, error) {
	if c.GetApplicationFunc != nil {
		return c.GetApplicationFunc(c.t, ctx, appID, appType)
	}
	panic("Client.GetApplication not implemented")
}

// OrgURL implements [[api.Client]].
func (c *Client) OrgURL() string {
	if c.OrgURLFunc != nil {
		return c.OrgURLFunc(c.t)
	}
	panic("Client.OrgURL not implemented")
}

// OrgName implements [[api.Client]].
func (c *Client) OrgName(ctx context.Context) (string, error) {
	if c.OrgNameFunc != nil {
		return c.OrgNameFunc(c.t, ctx)
	}
	panic("Client.OrgName not implemented")
}

// DoHttp implements [[api.Client]].
func (c *Client) DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	if c.DoHttpFunc != nil {
		return c.DoHttpFunc(c.t, ctx, method, url, accept)
	}
	panic("Client.DoHttp not implemented")
}

// GetScopes implements [[api.Client]].
func (c *Client) GetScopes() []string {
	if c.GetScopesFunc != nil {
		return c.GetScopesFunc(c.t)
	}
	panic("Client.GetScopes not implemented")
}
