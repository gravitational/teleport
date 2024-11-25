package okta

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/tests/common"
)

type oktaUserType struct {
	*okta.User
}

func (s *oktaUserType) login() string {
	if s == nil {
		panic("okta user is nil")
	}
	if s.Profile == nil {
		panic("okta user profile is nil")
	}
	val, ok := (*s.Profile)["login"]
	if !ok {
		panic("login field not found")
	}
	var login string
	login, ok = val.(string)
	if !ok {
		panic("login field is not a string")
	}
	return login
}

type oktaInfraSetup struct {
	Users  []*oktaUserType
	Apps   []*okta.BasicAuthApplication
	Groups []*okta.Group
	client *mockOktaAPIClient
	ctx    context.Context
}

type oktaSetupOptions struct {
	appCount    int
	groupsCount int
	usersCount  int
}

type oktaSetupOptionFun func(*oktaSetupOptions)

func withAppsGroupsUsersCount(apps, groups, users int) oktaSetupOptionFun {
	return func(o *oktaSetupOptions) {
		o.appCount = apps
		o.groupsCount = groups
		o.usersCount = users
	}
}

func createOktaSetupTreeAppGroupUserAndBasicUserGroupAssigment(t *testing.T, ctx context.Context) *oktaInfraSetup {
	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), oktaAppGroupsUsersCount)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[0].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[2].Id)
	return oktaInfra
}

func createOktaSetup(t *testing.T, ctx context.Context, oktaClient *mockOktaAPIClient, opts ...oktaSetupOptionFun) *oktaInfraSetup {
	defaultOptions := oktaSetupOptions{
		appCount:    1,
		groupsCount: 2,
		usersCount:  7,
	}

	for _, opt := range opts {
		opt(&defaultOptions)
	}
	oktaInfra := oktaInfraSetup{
		client: oktaClient,
		ctx:    ctx,
	}
	for i := 0; i < defaultOptions.usersCount; i++ {
		oktaInfra.Users = append(oktaInfra.Users, &oktaUserType{
			createOktaUser(t, ctx, oktaInfra.client, fmt.Sprintf("user-%d", i)),
		})
	}
	for i := 0; i < defaultOptions.appCount; i++ {
		oktaInfra.Apps = append(oktaInfra.Apps, createOktaApp(t, ctx, oktaInfra.client, fmt.Sprintf("app-%d", i)))
	}
	for i := 0; i < defaultOptions.groupsCount; i++ {
		oktaInfra.Groups = append(oktaInfra.Groups, createOktaGroup(t, ctx, oktaInfra.client, fmt.Sprintf("group-%d", i)))
	}

	return &oktaInfra
}

func (s *oktaInfraSetup) isUserAssignedToGroup(t *testing.T, userID, groupID string) bool {
	var found bool
	for _, group := range s.getUserGroups(t, userID) {
		if group == groupID {
			return true
		}
	}
	return found
}

func (s *oktaInfraSetup) getUserGroups(t *testing.T, oktaUserID string) []string {
	var groups []*okta.Group
	var err error
	groups, _, err = s.client.ListUserGroups(s.ctx, oktaUserID)
	require.NoError(t, err)

	var out []string
	for _, v := range groups {
		out = append(out, v.Id)
	}
	return out
}

func (s *oktaInfraSetup) assertUserWasAssignedToOktaGroup(t *testing.T, userID string, groupID string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ok := s.isUserAssignedToGroup(t, userID, groupID)
		assert.True(c, ok)
	}, time.Second*10, time.Millisecond*50, "User %s was assigned to group %s", userID, groupID)
}

func (s *oktaInfraSetup) assertUserWasUnassignedFromOktaGroup(t *testing.T, userID, groupID string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ok := s.isUserAssignedToGroup(t, userID, groupID)
		assert.False(c, ok)
	}, time.Second*10, time.Millisecond*50, "User %s is still assigned to group %s", userID, groupID)
}

func (s *oktaInfraSetup) createApplicationGroupAssignment(t *testing.T, appID, groupID string) {
	_, _, err := s.client.CreateApplicationGroupAssignment(s.ctx, appID, groupID, okta.ApplicationGroupAssignment{})
	require.NoError(t, err)
}

func (s *oktaInfraSetup) addUserToGroup(t *testing.T, groupID, userID string) {
	_, err := s.client.AddUserToGroup(s.ctx, groupID, userID)
	require.NoError(t, err, "failed to add %s user to group %s", userID, groupID)
}

func createOktaApp(t *testing.T, ctx context.Context, client *mockOktaAPIClient, name string) *okta.BasicAuthApplication {
	basicApplication := okta.NewBasicAuthApplication()
	basicApplication.Settings = &okta.BasicApplicationSettings{
		App: &okta.BasicApplicationSettingsApplication{
			AuthURL: "https://example.com/auth.html",
			Url:     "https://example.com/auth.html",
		},
	}
	basicApplication.Label = fmt.Sprintf("%s-%s", name, uuid.New())

	application, _, err := client.CreateApplication(ctx, basicApplication, nil)
	require.NoError(t, err)
	app, ok := application.(*okta.BasicAuthApplication)
	require.True(t, ok)
	return app
}

func createOktaGroup(t *testing.T, ctx context.Context, client *mockOktaAPIClient, groupName string) *okta.Group {
	g := okta.Group{
		Profile: &okta.GroupProfile{
			Name: fmt.Sprintf("%s-%s", groupName, uuid.New().String()),
		},
	}
	var group *okta.Group
	var err error
	group, _, err = client.CreateGroup(ctx, g)
	require.NoError(t, err, "Creating a group user should not error")
	return group
}

func createOktaUser(t *testing.T, ctx context.Context, client *mockOktaAPIClient, name string) *okta.User {
	email := fmt.Sprintf("%s@example.com", name)
	u := &okta.CreateUserRequest{
		Credentials: &okta.UserCredentials{
			Password: &okta.PasswordCredential{
				Value: uuid.NewString(),
			},
		},
		Profile: &okta.UserProfile{
			"firstName": name,
			"lastName":  fmt.Sprintf("last-%s", name),
			"email":     email,
			"login":     email,
		},
	}
	qp := query.NewQueryParams(query.WithActivate(true))
	oktaUser, _, err := client.CreateUser(ctx, *u, qp)
	require.NoError(t, err, "Creating a new user should not error")
	return oktaUser
}

func mustGetAppIDbyAppLabel(t *testing.T, sut *common.SUT, oktaInfra *oktaInfraSetup) string {
	apps, err := sut.Teleport.Process.GetAuthServer().GetApplicationServers(context.Background(), "default")
	require.NoError(t, err)
	var appID string
	for _, v := range apps {
		if v.GetApp().GetDescription() == oktaInfra.Apps[0].Label {
			appID = v.GetApp().GetName()
			break
		}
	}
	require.NotEmpty(t, appID)
	return appID
}

func oktaUserToSCIMUser(oktaUser *oktaUserType) *oktaSCIMUser {
	return &oktaSCIMUser{
		ExternalID: oktaUser.Id,
		ID:         oktaUser.login(),
		UserName:   oktaUser.login(),
		Groups:     []string{},
	}
}

type oktaSCIMUser struct {
	ExternalID string `json:"externalId"`
	ID         string `json:"id"`
	Meta       struct {
		Created  time.Time `json:"created"`
		Location string    `json:"location"`
		Version  string    `json:"version"`
	} `json:"meta"`
	Schemas  []string `json:"schemas"`
	UserName string   `json:"userName"`
	Name     struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	Emails []struct {
		Primary bool   `json:"primary"`
		Value   string `json:"value"`
		Type    string `json:"type"`
	} `json:"emails"`
	DisplayName string   `json:"displayName"`
	Locale      string   `json:"locale"`
	Groups      []string `json:"groups"`
}

func pushSCIMUserCreate(t *testing.T, sut *common.SUT, user *oktaUserType, scimToken string) int {
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	u := url.URL{
		Scheme: "https",
		Path:   "/v1/webapi/scim/okta/Users",
		Host:   sut.ProxyAddr,
	}
	buff, err := json.Marshal(oktaUserToSCIMUser(user))
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buff))
	require.NoError(t, err)
	req.Header.Add("Authorization", "Bearer "+scimToken)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	return resp.StatusCode
}
