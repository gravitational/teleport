package oktaapi

import (
	"context"

	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
)

// APIClient is a very thin layer wrapping original Okta SDK. Its purpose is to allow replacing
// Okta SDK with custom implementation using [APIClientProvider].Set() and it's meant to be used as
// a backing client for [Client].
type APIClient interface {
	// GetOrgUrl returns Okta API URL this client is configured with.
	GetOrgUrl() string
	// GetScopes returns Okta API scopes this client was configured with. Please note that
	// [ApiClientProvider].defaultNewClient fetches the scopes from Okta using the provided
	// credentials.
	//
	// Scopes are never refreshed so it they may become inaccurate if the Okta Services API app
	// is changed during client's operation.
	GetScopes() []string

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
}

// NewAPIClient creates APIClient from the provided upstream Okta SDK client.
func NewAPIClient(client *okta.Client) APIClient {
	return &apiClient{
		client: client,
		orgUrl: client.GetConfig().Okta.Client.OrgUrl,
		scopes: client.GetConfig().Okta.Client.Scopes,
	}
}

// apiClient is an adapter for the Okta API client.
type apiClient struct {
	client *okta.Client
	orgUrl string
	scopes []string
}

// GetScopes returns the scopes for the Okta client.
func (o *apiClient) GetOrgUrl() string {
	return o.orgUrl
}

// GetScopes returns the scopes for the Okta client.
func (o *apiClient) GetScopes() []string {
	return o.scopes
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
