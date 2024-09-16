package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
)

type StatusCodeUpdater func(context.Context, types.PluginStatusCode)

type ClientConfig struct {
	// HTTPClient is an optional HTTP client that can be used to override the
	// default client for testing. Do not set in production.
	HTTPClient *http.Client
	// Endpoint is a URL indicating the root endpoint of the Okta API service
	Endpoint string
	// Token is an Okta-supplied user API access token
	Token string
	// Log receives any log info
	Log *slog.Logger
	// StatusSink receives status update information from the OktaClient.
	// May be nil, in which case status updates will be dropped.
	StatusSink common.StatusSink
	// UpdateStatusCode is a function that will report a new status code for the
	// entire integration.
	UpdateStatusCode StatusCodeUpdater
}

// Check validates the state of the ClientConfig, returning a non-nil error
// if the config is invalid.
func (cfg *ClientConfig) Check() error {
	if cfg.Endpoint == "" {
		return trace.BadParameter("missing Okta Client parameter EndPoint")
	}
	if cfg.Token == "" {
		return trace.BadParameter("missing Okta Client parameter Token")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Transport: &rateLimitingHTTPTransport{
				// This transport was taken from the Okta client.
				delegate: &http.Transport{
					IdleConnTimeout: oktaTransportIdleTimeout,
				},
				rateLimiter: rate.NewLimiter(
					rate.Every(time.Second/time.Duration(APICallsPerSecond)), 1),
			},
			Timeout: oktaConnectionTimeout,
		}
	}
	return nil
}

var clientProviderMtx sync.Mutex

// SetClientProvider sets the Okta client provider for testing.
func SetClientProvider(fn clientProviderFunc) {
	clientProviderMtx.Lock()
	defer clientProviderMtx.Unlock()
	clientProvider = fn
}

func getClientProvider() clientProviderFunc {
	clientProviderMtx.Lock()
	defer clientProviderMtx.Unlock()
	return clientProvider
}

type clientProviderFunc func(ctx context.Context, cfg ...okta.ConfigSetter) (OktaAPI, error)

// clientProvider is an Okta client interface that can be mocked for testing.
var clientProvider = func(ctx context.Context, cfg ...okta.ConfigSetter) (OktaAPI, error) {
	_, client, err := okta.NewClient(ctx, cfg...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewClientAPIAdapter(client), err
}

// OktaAPI is an interface for interacting with the Okta API.
type OktaAPI interface {
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

func NewClientAPIAdapter(client *okta.Client) OktaAPI {
	return &APIClient{
		client: client,
	}
}

// APIClient is an adapter for the Okta API client.
type APIClient struct {
	client *okta.Client
}

// AddUserToGroup will assign the given user to the group.
func (o *APIClient) AddUserToGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error) {
	return o.client.Group.AddUserToGroup(ctx, groupId, userId)
}

// ListGroupUsers will return the list of users in the group.
func (o *APIClient) ListGroupUsers(ctx context.Context, groupId string, qp *query.Params) ([]*okta.User, *okta.Response, error) {
	return o.client.Group.ListGroupUsers(ctx, groupId, qp)
}

// ListGroups will return the list of groups.
func (o *APIClient) ListGroups(ctx context.Context, qp *query.Params) ([]*okta.Group, *okta.Response, error) {
	return o.client.Group.ListGroups(ctx, qp)
}

// CloneRequestExecutor returns a clone of the request executor.
func (o *APIClient) CloneRequestExecutor() *okta.RequestExecutor {
	return o.client.CloneRequestExecutor()
}

// GetOrgSettings will return the organization settings.
func (o *APIClient) GetOrgSettings(ctx context.Context) (*okta.OrgSetting, *okta.Response, error) {
	return o.client.OrgSetting.GetOrgSettings(ctx)
}

// GetUser will fetch the profile of the user with the given ID.
func (o *APIClient) GetUser(ctx context.Context, userId string) (*okta.User, *okta.Response, error) {
	return o.client.User.GetUser(ctx, userId)
}

// ListUsers will return the list of users.
func (o *APIClient) ListUsers(ctx context.Context, qp *query.Params) ([]*okta.User, *okta.Response, error) {
	return o.client.User.ListUsers(ctx, qp)
}

// ListUserGroups will return the list of groups a user belongs to.
func (o *APIClient) ListUserGroups(ctx context.Context, userId string) ([]*okta.Group, *okta.Response, error) {
	return o.client.User.ListUserGroups(ctx, userId)
}

// ListApplications will return the list of applications.
func (o *APIClient) ListApplications(ctx context.Context, qp *query.Params) ([]okta.App, *okta.Response, error) {
	return o.client.Application.ListApplications(ctx, qp)
}

// ListApplicationUsers will return the list of users assigned to the application.
func (o *APIClient) ListApplicationUsers(ctx context.Context, appId string, qp *query.Params) ([]*okta.AppUser, *okta.Response, error) {
	return o.client.Application.ListApplicationUsers(ctx, appId, qp)
}

// ListApplicationGroupAssignments will return the list of group assignments for the application.
func (o *APIClient) ListApplicationGroupAssignments(ctx context.Context, appId string, qp *query.Params) ([]*okta.ApplicationGroupAssignment, *okta.Response, error) {
	return o.client.Application.ListApplicationGroupAssignments(ctx, appId, qp)
}

// RemoveUserFromGroup will remove the user from the group.
func (o *APIClient) RemoveUserFromGroup(ctx context.Context, groupId string, userId string) (*okta.Response, error) {
	return o.client.Group.RemoveUserFromGroup(ctx, groupId, userId)
}

// AssignUserToApplication will assign the given user to the application.
func (o *APIClient) AssignUserToApplication(ctx context.Context, appId string, body okta.AppUser) (*okta.AppUser, *okta.Response, error) {
	return o.client.Application.AssignUserToApplication(ctx, appId, body)
}

// CreateApplicationGroupAssignment will create a new group assignment for the application.
func (o *APIClient) CreateApplicationGroupAssignment(ctx context.Context, appId string, groupId string, body okta.ApplicationGroupAssignment) (*okta.ApplicationGroupAssignment, *okta.Response, error) {
	return o.client.Application.CreateApplicationGroupAssignment(ctx, appId, groupId, body)
}

// DeleteApplicationUser will remove the user from the application.
func (o *APIClient) DeleteApplicationUser(ctx context.Context, appId string, userId string, qp *query.Params) (*okta.Response, error) {
	return o.client.Application.DeleteApplicationUser(ctx, appId, userId, qp)
}

// CreateApplication attempts to create a new Okta application from the supplied application request.
func (o *APIClient) CreateApplication(ctx context.Context, body okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	return o.client.Application.CreateApplication(ctx, body, qp)
}

// GetApplication fetches the data for a single application, by ID
func (o *APIClient) GetApplication(ctx context.Context, appId string, appInstance okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	return o.client.Application.GetApplication(ctx, appId, appInstance, qp)
}
