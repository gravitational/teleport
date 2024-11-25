package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport/e/lib/okta/api"
)

type mockOktaAPIClient struct {
	mu           sync.Mutex
	users        map[string]*okta.User
	groups       map[string]*okta.Group
	applications map[string]okta.App
	// Represents        groupId -> userId -> true
	groupAssignments map[string]map[string]bool
	// Represents       appId -> userId -> true
	appUserAssignments map[string]map[string]bool
	// Represents        appId -> groupId -> assignment
	appGroupAssignments map[string]map[string]*okta.ApplicationGroupAssignment
	// Okta Token Scopes
	scopes []string
	rt     http.RoundTripper
}

// GetScopes returns the scopes.
func (m *mockOktaAPIClient) GetScopes() []string {
	return m.scopes
}

func newMockOktaAPIClient() *mockOktaAPIClient {
	m := &mockOktaAPIClient{
		users:               make(map[string]*okta.User),
		groups:              make(map[string]*okta.Group),
		applications:        make(map[string]okta.App),
		groupAssignments:    make(map[string]map[string]bool),
		appUserAssignments:  make(map[string]map[string]bool),
		appGroupAssignments: make(map[string]map[string]*okta.ApplicationGroupAssignment),
		scopes: []string{
			api.ScopeUserManage,
			api.ScopeUserRead,
			api.ScopeAppsManage,
			api.ScopeAppsRead,
			api.ScopeGroupsManage,
			api.ScopeGroupsRead,
			api.ScopeOrgsRead,
		},
	}
	setOktaMockedAPIClient(m)
	return m
}

func setOktaMockedAPIClient(apiMock *mockOktaAPIClient) {
	api.SetClientProvider(func(ctx context.Context, cfg ...okta.ConfigSetter) (api.OktaAPI, error) {
		return apiMock, nil
	})
}

func (m *mockOktaAPIClient) DeactivateUser(ctx context.Context, userId string, qp *query.Params) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userId]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	user.Status = "DEPROVISIONED"
	return &okta.Response{}, nil
}

func (m *mockOktaAPIClient) CreateUser(_ context.Context, body okta.CreateUserRequest, qp *query.Params) (*okta.User, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	userID := uuid.NewString()

	// Create the user
	user := &okta.User{
		Id:      userID,
		Profile: body.Profile,
		Status:  "PROVISIONED",
		Type:    body.Type,
	}

	if qp != nil && qp.Activate != nil && *qp.Activate {
		user.Status = "ACTIVE"
		activatedTime := time.Now()
		user.Activated = &activatedTime
	}

	m.users[userID] = user

	for _, groupID := range body.GroupIds {
		if m.groupAssignments[groupID] == nil {
			m.groupAssignments[groupID] = make(map[string]bool)
		}
		m.groupAssignments[groupID][userID] = true
	}

	return user, &okta.Response{}, nil
}

// CreateGroup will create a new group.
func (m *mockOktaAPIClient) CreateGroup(_ context.Context, body okta.Group) (*okta.Group, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	groupID := uuid.NewString()
	body.Id = groupID
	m.groups[groupID] = &body

	return &body, &okta.Response{}, nil
}

// AddUserToGroup will assign the given user to the group.
func (m *mockOktaAPIClient) AddUserToGroup(_ context.Context, groupId string, userId string) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.groupAssignments[groupId] == nil {
		m.groupAssignments[groupId] = make(map[string]bool)
	}
	m.groupAssignments[groupId][userId] = true

	return &okta.Response{}, nil
}

// ListGroupUsers will return the list of users in the group.
func (m *mockOktaAPIClient) ListGroupUsers(_ context.Context, groupId string, _ *query.Params) ([]*okta.User, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var users []*okta.User
	for userId := range m.groupAssignments[groupId] {
		if user, exists := m.users[userId]; exists {
			users = append(users, user)
		}
	}

	return users, &okta.Response{}, nil
}

// ListGroups will return the list of groups.
func (m *mockOktaAPIClient) ListGroups(_ context.Context, _ *query.Params) ([]*okta.Group, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var groups []*okta.Group
	for _, group := range m.groups {
		groups = append(groups, group)
	}

	return groups, &okta.Response{}, nil
}

// CloneRequestExecutor returns a clone of the request executor.
func (m *mockOktaAPIClient) CloneRequestExecutor() *okta.RequestExecutor {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, c, err := okta.NewClient(
		context.Background(),
		okta.WithOrgUrl("https://example.com"),
		okta.WithHttpClientPtr(&http.Client{Transport: m.rt}),
		okta.WithToken("token"),
	)
	if err != nil {
		panic(err)
	}
	executor := c.CloneRequestExecutor()
	return executor
}

// GetOrgSettings will return the organization settings.
func (m *mockOktaAPIClient) GetOrgSettings(_ context.Context) (*okta.OrgSetting, *okta.Response, error) {
	return &okta.OrgSetting{}, &okta.Response{}, nil
}

// GetUser will fetch the profile of the user with the given ID.
func (m *mockOktaAPIClient) GetUser(_ context.Context, userId string) (*okta.User, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userId]
	if !exists {
		return nil, nil, fmt.Errorf("user not found")
	}

	return user, &okta.Response{}, nil
}

// ListUsers will return the list of users.
func (m *mockOktaAPIClient) ListUsers(_ context.Context, qp *query.Params) ([]*okta.User, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var users []*okta.User
	for _, user := range m.users {
		if user.Status == "DEPROVISIONED" {
			continue
		}
		users = append(users, user)
	}

	return users, &okta.Response{}, nil
}

// ListUserGroups will return the list of groups a user belongs to.
func (m *mockOktaAPIClient) ListUserGroups(_ context.Context, userId string) ([]*okta.Group, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var groups []*okta.Group
	for groupId, users := range m.groupAssignments {
		if users[userId] {
			if group, exists := m.groups[groupId]; exists {
				groups = append(groups, group)
			}
		}
	}

	return groups, &okta.Response{}, nil
}

// ListApplications will return the list of applications.
func (m *mockOktaAPIClient) ListApplications(_ context.Context, qp *query.Params) ([]okta.App, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var apps []okta.App
	for _, app := range m.applications {
		buff, err := json.Marshal(app)
		if err != nil {
			panic(err)
		}
		var item okta.Application
		if err := json.Unmarshal(buff, &item); err != nil {
			panic(err)
		}
		apps = append(apps, &item)
	}

	return apps, &okta.Response{}, nil
}

// ListApplicationUsers will return the list of users assigned to the application.
func (m *mockOktaAPIClient) ListApplicationUsers(_ context.Context, appId string, qp *query.Params) ([]*okta.AppUser, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var appUsers []*okta.AppUser
	for userId := range m.appUserAssignments[appId] {
		appUsers = append(appUsers, &okta.AppUser{Id: userId})
	}

	return appUsers, &okta.Response{}, nil
}

// ListApplicationGroupAssignments will return the list of group assignments for the application.
func (m *mockOktaAPIClient) ListApplicationGroupAssignments(_ context.Context, appId string, qp *query.Params) ([]*okta.ApplicationGroupAssignment, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var assignments []*okta.ApplicationGroupAssignment
	if groups, exists := m.appGroupAssignments[appId]; exists {
		for _, assignment := range groups {
			assignments = append(assignments, assignment)
		}
	}

	return assignments, &okta.Response{}, nil
}

// RemoveUserFromGroup will remove the user from the group.
func (m *mockOktaAPIClient) RemoveUserFromGroup(_ context.Context, groupId string, userId string) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.groupAssignments[groupId] != nil {
		delete(m.groupAssignments[groupId], userId)
	}

	return &okta.Response{}, nil
}

// AssignUserToApplication will assign the given user to the application.
func (m *mockOktaAPIClient) AssignUserToApplication(_ context.Context, appId string, body okta.AppUser) (*okta.AppUser, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.appUserAssignments[appId] == nil {
		m.appUserAssignments[appId] = make(map[string]bool)
	}
	m.appUserAssignments[appId][body.Id] = true

	return &body, &okta.Response{}, nil
}

// CreateApplicationGroupAssignment will create a new group assignment for the application.
func (m *mockOktaAPIClient) CreateApplicationGroupAssignment(_ context.Context, appId string, groupId string, body okta.ApplicationGroupAssignment) (*okta.ApplicationGroupAssignment, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	body.Id = groupId

	if m.appGroupAssignments[appId] == nil {
		m.appGroupAssignments[appId] = make(map[string]*okta.ApplicationGroupAssignment)
	}
	m.appGroupAssignments[appId][groupId] = &body

	return &body, &okta.Response{}, nil
}

// DeleteApplicationUser will remove the user from the application.
func (m *mockOktaAPIClient) DeleteApplicationUser(_ context.Context, appId string, userId string, qp *query.Params) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.appUserAssignments[appId] != nil {
		delete(m.appUserAssignments[appId], userId)
	}

	return &okta.Response{}, nil
}

// CreateApplication attempts to create a new Okta application from the supplied application request.
func (m *mockOktaAPIClient) CreateApplication(_ context.Context, body okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	appID := uuid.NewString()

	links := map[string]interface{}{
		"appLinks": []interface{}{
			map[string]interface{}{
				"name": "applink-name",
				"href": "https://www.link.com",
			},
		},
	}
	switch t := body.(type) {
	case *okta.BasicAuthApplication:
		t.Id = appID
		t.Status = "ACTIVE"
		t.Links = links
	case *okta.SamlApplication:
		t.Id = appID
		t.Status = "ACTIVE"
		t.Links = map[string]any{
			"metadata": map[string]string{
				"href": "https://12345.okta.com/api/v1/apps/12345/sso/saml/metadata",
				"type": "application/xml",
			},
		}
	case *okta.BookmarkApplication:
		t.Id = appID
		t.Status = "ACTIVE"
		t.Links = links
	default:
		panic(fmt.Sprintf("unexpected Okta application type %T", t))
	}

	m.applications[appID] = body

	return body, &okta.Response{}, nil
}

// GetApplication fetches the data for a single application, by ID
func (m *mockOktaAPIClient) GetApplication(_ context.Context, appId string, appInstance okta.App, qp *query.Params) (okta.App, *okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.applications[appId]
	if !exists {
		return nil, nil, fmt.Errorf("application not found")
	}

	return app, &okta.Response{}, nil
}

func (m *mockOktaAPIClient) DeactivateOrDeleteUser(_ context.Context, userId string, qp *query.Params) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userId]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if user.Status == "DEPROVISIONED" {
		delete(m.users, userId)
	}

	return &okta.Response{}, nil

}

func (m *mockOktaAPIClient) DeleteGroup(_ context.Context, groupId string) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.groups[groupId]
	if !exists {
		return nil, fmt.Errorf("group not found")
	}

	delete(m.groups, groupId)
	if m.groupAssignments[groupId] != nil {
		delete(m.groupAssignments, groupId)
	}

	for appId, groupAssignments := range m.appGroupAssignments {
		if _, exists := groupAssignments[groupId]; exists {
			delete(groupAssignments, groupId)
			if len(groupAssignments) == 0 {
				delete(m.appGroupAssignments, appId)
			}
		}
	}

	return &okta.Response{}, nil
}

func (m *mockOktaAPIClient) DeactivateApplication(_ context.Context, appId string) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.applications[appId]
	if !exists {
		return nil, fmt.Errorf("application not found")
	}

	switch t := app.(type) {
	case *okta.BasicAuthApplication:
		t.Status = "INACTIVE"
	case *okta.SamlApplication:
		t.Status = "INACTIVE"
	case *okta.BookmarkApplication:
		t.Status = "INACTIVE"
	default:
		panic(fmt.Sprintf("unexpected Okta application type %T", t))
	}

	m.applications[appId] = app

	return &okta.Response{}, nil
}

func (m *mockOktaAPIClient) DeleteApplication(_ context.Context, appId string) (*okta.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.applications[appId]
	if !exists {
		return nil, fmt.Errorf("application not found")
	}

	delete(m.applications, appId)

	if m.appUserAssignments[appId] != nil {
		delete(m.appUserAssignments, appId)
	}

	if m.appGroupAssignments[appId] != nil {
		delete(m.appGroupAssignments, appId)
	}
	return &okta.Response{}, nil
}

func (m *mockOktaAPIClient) setRoundTripper(rt http.RoundTripper) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rt = rt
}
