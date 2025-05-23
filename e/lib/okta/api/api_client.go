package oktaapi

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
)

// APIClient is a very thin layer wrapping original Okta SDK. Its purpose is to allow replacing
// Okta SDK with custom implementation using [APIClientProvider].Set() and it's meant to be used as
// a backing client for [Client].
type APIClient interface {
	// GetOrgUrl returns Okta API URL this client is configured with.
	GetOrgUrl() string

	// GetAuthorizedScopes verifies and returns the list of configured OAuth scopes trimmed
	// down to those allowed by the configured credentials.
	GetAuthorizedScopes(ctx context.Context) ([]string, error)
	GetOrgSettings(ctx context.Context) (*okta.OrgSetting, *okta.Response, error)
	GetUser(ctx context.Context, userId string) (*okta.User, *okta.Response, error)
	ListUsers(ctx context.Context, qp *query.Params) ([]*okta.User, *okta.Response, error)
	ListGroups(ctx context.Context, qp *query.Params) ([]*okta.Group, *okta.Response, error)
	ListUserGroups(ctx context.Context, userId string) ([]*okta.Group, *okta.Response, error)
	ListApplications(ctx context.Context, qp *query.Params) ([]okta.App, *okta.Response, error)
	ListApplicationUsers(ctx context.Context, appId string, qp *query.Params) ([]*okta.AppUser, *okta.Response, error)
	ListApplicationGroupAssignments(ctx context.Context, appId string, qp *query.Params) ([]*okta.ApplicationGroupAssignment, *okta.Response, error)
	RemoveUserFromGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error)
	AssignUserToApplication(ctx context.Context, appId string, body okta.AppUser) (*okta.AppUser, *okta.Response, error)
	CreateApplicationGroupAssignment(ctx context.Context, appId string, groupId string, body okta.ApplicationGroupAssignment) (*okta.ApplicationGroupAssignment, *okta.Response, error)
	DeleteApplicationUser(ctx context.Context, appId string, userId string, qp *query.Params) (*okta.Response, error)
	CreateApplication(ctx context.Context, body okta.App, qp *query.Params) (okta.App, *okta.Response, error)
	GetApplication(ctx context.Context, appId string, appInstance okta.App, qp *query.Params) (okta.App, *okta.Response, error)
	CloneRequestExecutor() *okta.RequestExecutor
	ListGroupUsers(ctx context.Context, groupId string, qp *query.Params) ([]*okta.User, *okta.Response, error)
	AddUserToGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error)
	ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error)
	ListApiTokens(ctx context.Context, qp *query.Params) ([]*ApiToken, *okta.Response, error)
	ListUsersWithRoleAssignments(ctx context.Context) (*RoleAssignedUsers, *okta.Response, error)
	ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error)
}

// NewAPIClient creates APIClient from the provided upstream Okta SDK client.
func NewAPIClient(client *okta.Client) APIClient {
	return &apiClient{
		client: client,
		orgUrl: client.GetConfig().Okta.Client.OrgUrl,
	}
}

// apiClient is an adapter for the Okta API client.
type apiClient struct {
	client *okta.Client
	orgUrl string
}

// GetOrgUrl implements [APIClient].GetOrgUrl.
func (o *apiClient) GetOrgUrl() string {
	return o.orgUrl
}

// GetAuthorizedScopes implements [APIClient].GetAuthorizedScopes.
func (o *apiClient) GetAuthorizedScopes(ctx context.Context) ([]string, error) {
	scopes, err := getAuthorizedScopes(ctx, o.client)
	return scopes, trace.Wrap(err)
}

// AddUserToGroup will assign the given user to the group.
func (o *apiClient) AddUserToGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error) {
	return o.client.Group.AddUserToGroup(ctx, groupId, userId)
}

// ListGroupUsers will return the list of users in the group.
func (o *apiClient) ListGroupUsers(ctx context.Context, groupId string, qp *query.Params) ([]*okta.User, *okta.Response, error) {
	return o.client.Group.ListGroupUsers(ctx, groupId, qp)
}

// ListGroups will return the list of groups.
func (o *apiClient) ListGroups(ctx context.Context, qp *query.Params) ([]*okta.Group, *okta.Response, error) {
	return o.client.Group.ListGroups(ctx, qp)
}

// CloneRequestExecutor returns a clone of the request executor.
func (o *apiClient) CloneRequestExecutor() *okta.RequestExecutor {
	return o.client.CloneRequestExecutor()
}

// GetOrgSettings will return the organization settings.
func (o *apiClient) GetOrgSettings(ctx context.Context) (*okta.OrgSetting, *okta.Response, error) {
	return o.client.OrgSetting.GetOrgSettings(ctx)
}

// GetUser will fetch the profile of the user with the given ID.
func (o *apiClient) GetUser(ctx context.Context, userId string) (*okta.User, *okta.Response, error) {
	return o.client.User.GetUser(ctx, userId)
}

// ListUsers will return the list of users.
func (o *apiClient) ListUsers(ctx context.Context, qp *query.Params) ([]*okta.User, *okta.Response, error) {
	return o.client.User.ListUsers(ctx, qp)
}

// ListUserGroups will return the list of groups a user belongs to.
func (o *apiClient) ListUserGroups(ctx context.Context, userId string) ([]*okta.Group, *okta.Response, error) {
	return o.client.User.ListUserGroups(ctx, userId)
}

// ListApplications will return the list of applications.
func (o *apiClient) ListApplications(ctx context.Context, qp *query.Params) ([]okta.App, *okta.Response, error) {
	return o.client.Application.ListApplications(ctx, qp)
}

// ListApplicationUsers will return the list of users assigned to the application.
func (o *apiClient) ListApplicationUsers(ctx context.Context, appId string, qp *query.Params) ([]*okta.AppUser, *okta.Response, error) {
	return o.client.Application.ListApplicationUsers(ctx, appId, qp)
}

// ListApplicationGroupAssignments will return the list of group assignments for the application.
func (o *apiClient) ListApplicationGroupAssignments(ctx context.Context, appId string, qp *query.Params) ([]*okta.ApplicationGroupAssignment, *okta.Response, error) {
	return o.client.Application.ListApplicationGroupAssignments(ctx, appId, qp)
}

// RemoveUserFromGroup will remove the user from the group.
func (o *apiClient) RemoveUserFromGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error) {
	return o.client.Group.RemoveUserFromGroup(ctx, groupId, userId)
}

// AssignUserToApplication will assign the given user to the application.
func (o *apiClient) AssignUserToApplication(ctx context.Context, appId string, body okta.AppUser) (*okta.AppUser, *okta.Response, error) {
	return o.client.Application.AssignUserToApplication(ctx, appId, body)
}

// CreateApplicationGroupAssignment will create a new group assignment for the application.
func (o *apiClient) CreateApplicationGroupAssignment(ctx context.Context, appId string, groupId string, body okta.ApplicationGroupAssignment) (*okta.ApplicationGroupAssignment, *okta.Response, error) {
	return o.client.Application.CreateApplicationGroupAssignment(ctx, appId, groupId, body)
}

// DeleteApplicationUser will remove the user from the application.
func (o *apiClient) DeleteApplicationUser(ctx context.Context, appId string, userId string, qp *query.Params) (*okta.Response, error) {
	return o.client.Application.DeleteApplicationUser(ctx, appId, userId, qp)
}

// CreateApplication attempts to create a new Okta application from the supplied application request.
func (o *apiClient) CreateApplication(ctx context.Context, body okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	return o.client.Application.CreateApplication(ctx, body, qp)
}

// GetApplication fetches the data for a single application, by ID
func (o *apiClient) GetApplication(ctx context.Context, appId string, appInstance okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	return o.client.Application.GetApplication(ctx, appId, appInstance, qp)
}

// ListLogEvents will return the list of log events.
func (o *apiClient) ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error) {
	logs, resp, err := o.client.LogEvent.GetLogs(ctx, qp)
	return logs, resp, trace.Wrap(err)
}

func (o *apiClient) ListApiTokens(ctx context.Context, qp *query.Params) ([]*ApiToken, *okta.Response, error) {
	rq := o.client.CloneRequestExecutor()
	url := "/api/v1/api-tokens"
	if qp != nil {
		url = url + qp.String()
	}

	req, err := rq.WithAccept("application/json").WithContentType("application/json").NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var tokens []*ApiToken

	resp, err := rq.Do(ctx, req, &tokens)
	if err != nil {
		return nil, resp, trace.Wrap(err)
	}

	return tokens, resp, nil
}

// ApiToken An API token for an Okta User. This token is NOT scoped any further and can be used for any API the user has permissions to call.
type ApiToken struct {
	ClientName  *string    `json:"clientName,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	Id          *string    `json:"id,omitempty"`
	LastUpdated *time.Time `json:"lastUpdated,omitempty"`
	Name        string     `json:"name"`
	// A time duration specified as an [ISO-8601 duration](https://en.wikipedia.org/wiki/ISO_8601#Durations).
	TokenWindow *string `json:"tokenWindow,omitempty"`
	UserId      *string `json:"userId,omitempty"`
}

func (o *apiClient) ListUsersWithRoleAssignments(ctx context.Context) (*RoleAssignedUsers, *okta.Response, error) {
	rq := o.client.CloneRequestExecutor()
	url := "/api/v1/iam/assignees/users"
	req, err := rq.WithAccept("application/json").WithContentType("application/json").NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var users RoleAssignedUsers

	resp, err := rq.Do(ctx, req, &users)
	if err != nil {
		return nil, resp, trace.Wrap(err)
	}

	return &users, resp, nil
}

// RoleAssignedUsers struct for RoleAssignedUsers
type RoleAssignedUsers struct {
	Value []RoleAssignedUser `json:"value,omitempty"`
}

// RoleAssignedUser struct for RoleAssignedUser
type RoleAssignedUser struct {
	Id  *string `json:"id,omitempty"`
	Orn *string `json:"orn,omitempty"`
}

func (o *apiClient) ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error) {
	roles, rsp, err := o.client.User.ListAssignedRolesForUser(ctx, userId, nil)
	return roles, rsp, trace.Wrap(err)
}
