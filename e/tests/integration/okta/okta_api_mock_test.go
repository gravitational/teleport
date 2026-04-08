package okta

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

var apiCredentials = &oktav1.OktaAPICredentials{
	Auth: &oktav1.OktaAPICredentials_OauthId{
		OauthId: "test-oauth-client-id-12345",
	},
}

type fakeOktaServerOptions struct {
	appCount    int
	userCount   int
	groupCount  int
	SAMLAppName bool
}

type fakeOktaServer struct {
	server *httptest.Server

	// These resources are provisioned if the fakeOktaServer is created
	// with fakeOktaServerOptions. All resources are static and do not
	// get modified as a result of user actions.
	provisionedApps    []*okta.BasicAuthApplication
	provisionedGroups  []*okta.Group
	provisionedUsers   []*okta.User
	provisionedSAMLApp *okta.SamlApplication

	mu           sync.Mutex
	scopes       []string
	users        map[string]*okta.User
	groups       map[string]*okta.Group
	applications map[string]okta.App
	// Represents        groupId -> userId
	groupAssignments map[string]map[string]struct{}
	// Represents       appId -> userId
	appUserAssignments map[string]map[string]struct{}
	// Represents        appId -> groupId -> assignment
	appGroupAssignments map[string]map[string]*okta.ApplicationGroupAssignment
}

func withAppCount(n int) func(options *fakeOktaServerOptions) {
	return func(options *fakeOktaServerOptions) {
		options.appCount = n
	}
}

func withUserCount(n int) func(options *fakeOktaServerOptions) {
	return func(options *fakeOktaServerOptions) {
		options.userCount = n
	}
}

func withGroupCount(n int) func(options *fakeOktaServerOptions) {
	return func(options *fakeOktaServerOptions) {
		options.groupCount = n
	}
}

func withSAMLApp() func(options *fakeOktaServerOptions) {
	return func(options *fakeOktaServerOptions) {
		options.SAMLAppName = true
	}
}

func newFakeOktaServer(opts ...func(options *fakeOktaServerOptions)) *fakeOktaServer {
	f := &fakeOktaServer{
		users:               make(map[string]*okta.User),
		groups:              make(map[string]*okta.Group),
		applications:        make(map[string]okta.App),
		groupAssignments:    make(map[string]map[string]struct{}),
		appUserAssignments:  make(map[string]map[string]struct{}),
		appGroupAssignments: make(map[string]map[string]*okta.ApplicationGroupAssignment),
		scopes: []string{
			oktaapi.ScopeUserRead,
			oktaapi.ScopeAppsManage,
			oktaapi.ScopeAppsRead,
			oktaapi.ScopeGroupsManage,
			oktaapi.ScopeGroupsRead,
		},
	}

	f.server = httptest.NewTLSServer(f.routes())

	var opt fakeOktaServerOptions
	for _, o := range opts {
		o(&opt)
	}

	f.provisionedUsers = make([]*okta.User, 0, opt.userCount)
	for i := range opt.userCount {
		f.provisionedUsers = append(f.provisionedUsers, f.CreateUser("user-"+strconv.Itoa(i)))
	}

	f.provisionedApps = make([]*okta.BasicAuthApplication, 0, opt.appCount)
	for i := range opt.appCount {
		f.provisionedApps = append(f.provisionedApps, f.CreateBasicApp("app-"+strconv.Itoa(i)))
	}

	f.provisionedGroups = make([]*okta.Group, 0, opt.groupCount)
	for i := range opt.groupCount {
		f.provisionedGroups = append(f.provisionedGroups, f.CreateGroup("group-"+strconv.Itoa(i)+"-"+uuid.New().String()))
	}

	if opt.SAMLAppName {
		f.provisionedSAMLApp = f.CreateSAMLApp("example_test-okta-app-name")
	}

	return f
}

func (f *fakeOktaServer) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(http.MethodGet+" /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListUsers()); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		user, err := f.GetUser(r.PathValue("userID"))
		if err != nil {
			f.error(w, r, "user does not exist", http.StatusNotFound)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(user); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/users/{userID}/groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListUserGroups(r.PathValue("userID"))); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListGroups()); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodPut+" /api/v1/groups/{groupID}/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		f.AddUserToGroup(r.PathValue("groupID"), r.PathValue("userID"))
	})

	mux.HandleFunc(http.MethodDelete+" /api/v1/groups/{groupID}/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		f.RemoveUserFromGroup(r.PathValue("groupID"), r.PathValue("userID"))
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/groups/{groupID}", func(w http.ResponseWriter, r *http.Request) {
		group, found := f.GetGroup(r.PathValue("groupID"))
		if !found {
			f.error(w, r, "group does not exist", http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(group); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/groups/{groupID}/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListGroupUsers(r.PathValue("groupID"))); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListApplications(r.URL.Query().Get("q"))); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/apps/{appID}/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListApplicationUsers(r.PathValue("appID"))); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodPost+" /api/v1/apps/{appID}/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		defer r.Body.Close()
		var user okta.User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			f.error(w, r, err.Error(), http.StatusBadRequest)
			return
		}

		if err := json.NewEncoder(w).Encode(f.AssignUserToApplication(r.PathValue("appID"), user.Id)); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/apps/{appID}/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		appID, userID := r.PathValue("appID"), r.PathValue("userID")
		if appID == "" {
			f.error(w, r, "appID path segment cannot be empty", http.StatusBadRequest)
			return
		}
		if userID == "" {
			f.error(w, r, "userID path segment cannot be empty", http.StatusBadRequest)
			return
		}
		user, err := f.GetAppUser(appID, userID)
		if err != nil {
			if trace.IsNotFound(err) {
				f.error(w, r, err.Error(), http.StatusNotFound)
				return
			}
			f.error(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(user); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodDelete+" /api/v1/apps/{appID}/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		if err := f.UnassignUserFromApplication(r.PathValue("appID"), r.PathValue("userID")); err != nil {
			f.error(w, r, err.Error(), http.StatusNotFound)
			return
		}
	})

	mux.HandleFunc(http.MethodGet+" /api/v1/apps/{appID}/groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(f.ListApplicationGroupAssignments(r.PathValue("appID"))); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodPost+" /oauth2/v1/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(okta.RequestAccessToken{
			TokenType:   "test",
			ExpiresIn:   int((10 * time.Hour).Seconds()),
			AccessToken: "test-123",
			Scope:       strings.Join(f.scopes, " "),
		}); err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(http.MethodGet+" /sso/saml/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/xml")

		if f.provisionedSAMLApp == nil {
			w.Write([]byte(idp.EntityDescriptor))
			return
		}

		decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(idp.TestOktaSAMLConnector(f.URL())), defaults.LookaheadBufSize)
		var raw services.UnknownResource
		if err := decoder.Decode(&raw); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		connector, err := services.UnmarshalSAMLConnector(raw.Raw)
		if err != nil {
			f.error(w, r, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(connector.GetEntityDescriptor()))
	})

	// Catch-all route for easier debugging.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.error(w, r, "404 page not found", http.StatusNotFound)
	})

	return mux
}

func (*fakeOktaServer) error(w http.ResponseWriter, r *http.Request, msg string, statusCode int) {
	type Error struct {
		ErrorDescription string `json:"error_description"`
	}
	e := Error{
		ErrorDescription: fmt.Sprintf("(*fakeOktaServer) handling %s [%d]: %s", r.URL, statusCode, msg),
	}
	encodedErr, err := json.Marshal(e)
	if err != nil {
		encodedErr = []byte(e.ErrorDescription)
	}
	http.Error(w, string(encodedErr), statusCode)
}

func (f *fakeOktaServer) Stop() {
	f.server.Close()
}

func (f *fakeOktaServer) URL() string {
	return f.server.URL
}

func (f *fakeOktaServer) Client() *http.Client {
	return f.server.Client()
}

func (f *fakeOktaServer) SetScopes(scopes []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.scopes = scopes
}

func (f *fakeOktaServer) CreateSAMLApp(name string) *okta.SamlApplication {
	f.mu.Lock()
	defer f.mu.Unlock()

	app := &okta.SamlApplication{
		SignOnMode: "SAML_2_0",
		Name:       name,
		Label:      name,
		Id:         uuid.NewString(),
		Status:     "ACTIVE",
		Links: map[string]any{
			"metadata": map[string]string{
				"href": f.URL() + "/sso/saml/metadata",
				"type": "application/xml",
			},
			"appLinks": []any{
				map[string]any{
					"name": "1234_oktaappname_1_link",
					"href": f.URL() + "/home/api/" + name + "//12345/someRandom697",
					"type": "text/html",
				},
			},
		},
	}

	f.applications[app.Id] = app
	return app
}

type oktaApplicationEmbedLinks struct {
	AppLinks []oktaApplicationEmbedLink
}

type oktaApplicationEmbedLink struct {
	Name string
	Href string
}

func (f *fakeOktaServer) CreateBasicApp(name string, appLinks ...oktaApplicationEmbedLink) *okta.BasicAuthApplication {
	f.mu.Lock()
	defer f.mu.Unlock()

	app := &okta.BasicAuthApplication{
		Name:       "template_basic_auth",
		SignOnMode: "BASIC_AUTH",
		Id:         uuid.NewString(),
		Status:     "ACTIVE",
		Settings: &okta.BasicApplicationSettings{
			App: &okta.BasicApplicationSettingsApplication{
				AuthURL: "https://example.com/auth.html",
				Url:     "https://example.com/auth.html",
			},
		},
		Label: name,
	}

	if appLinks != nil {
		app.Links = oktaApplicationEmbedLinks{AppLinks: appLinks}
	} else {
		app.Links = map[string]any{
			"metadata": map[string]string{
				"href": f.URL() + "/api/v1/apps/12345/sso/saml/metadata",
				"type": "application/xml",
			},
			"appLinks": []any{
				map[string]any{
					"name": "1234_oktaappname_1_link",
					"href": f.URL() + "/home/api/" + app.Label + "/12345/someRandom697",
					"type": "text/html",
				},
			},
		}
	}

	f.applications[app.Id] = app
	return app
}

func (f *fakeOktaServer) AssignUserToApplication(appID, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.applications[appID]; !ok {
		return trace.NotFound("application %q not found", appID)
	}

	if f.appUserAssignments[appID] == nil {
		f.appUserAssignments[appID] = make(map[string]struct{})
	}
	f.appUserAssignments[appID][userID] = struct{}{}

	return nil
}

func (f *fakeOktaServer) UnassignUserFromApplication(appID, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.applications[appID]; !ok {
		return trace.NotFound("application %q not found", appID)
	}

	delete(f.appUserAssignments[appID], userID)

	return nil
}

func (f *fakeOktaServer) IsUserAssignedToApplication(appID, userID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.appUserAssignments[appID]) == 0 {
		return false
	}

	_, ok := f.appUserAssignments[appID][userID]

	return ok
}

func (f *fakeOktaServer) Application(id string) (okta.App, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	app, ok := f.applications[id]
	return app, ok
}

func (f *fakeOktaServer) GroupAssignments(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.groupAssignments[id])
}

func (f *fakeOktaServer) UserAssignedGroup(groupID, userID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	assignments, ok := f.groupAssignments[groupID]
	if !ok {
		return false
	}

	_, ok = assignments[userID]
	return ok
}

func (f *fakeOktaServer) DeactivateUser(userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, exists := f.users[userID]
	if !exists {
		return trace.NotFound("user not found")
	}

	user.Status = "DEPROVISIONED"
	return nil
}

func (f *fakeOktaServer) CreateUser(name string) *okta.User {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	user := &okta.User{
		Id:        uuid.NewString(),
		Status:    "ACTIVE",
		Activated: &now,
		Profile: &okta.UserProfile{
			"firstName": name,
			"lastName":  "last-" + name,
			"email":     name + "@example.com",
			"login":     name + "@example.com",
		},
	}

	f.users[user.Id] = user
	return user
}

func (f *fakeOktaServer) CreateGroup(name string) *okta.Group {
	f.mu.Lock()
	defer f.mu.Unlock()

	group := &okta.Group{
		Id: uuid.NewString(),
		Profile: &okta.GroupProfile{
			Name: name,
		},
		Links: map[string]string{
			"href": f.URL() + "/sso/saml/metadata",
			"type": "application/xml",
		},
	}

	f.groups[group.Id] = group

	return group
}

func (f *fakeOktaServer) CreateBuiltInGroup(name string) *okta.Group {
	f.mu.Lock()
	defer f.mu.Unlock()

	group := &okta.Group{
		Type: "BUILT_IN",
		Id:   uuid.NewString(),
		Profile: &okta.GroupProfile{
			Name: name,
		},
		Links: map[string]string{
			"href": f.URL() + "/sso/saml/metadata",
			"type": "application/xml",
		},
	}

	f.groups[group.Id] = group

	return group
}

func (f *fakeOktaServer) CreateOktaGroup(name string) *okta.Group {
	f.mu.Lock()
	defer f.mu.Unlock()

	group := &okta.Group{
		Type: "OKTA_GROUP",
		Id:   uuid.NewString(),
		Profile: &okta.GroupProfile{
			Name: name,
		},
		Links: map[string]string{
			"href": f.URL() + "/sso/saml/metadata",
			"type": "application/xml",
		},
	}

	f.groups[group.Id] = group

	return group
}

func (f *fakeOktaServer) AddUserToGroup(groupID, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.groupAssignments[groupID] == nil {
		f.groupAssignments[groupID] = make(map[string]struct{})
	}
	f.groupAssignments[groupID][userID] = struct{}{}
}

// ListGroupUsers will return the list of users in the group.
func (f *fakeOktaServer) ListGroupUsers(groupID string) []*okta.User {
	f.mu.Lock()
	defer f.mu.Unlock()

	var users []*okta.User
	for userId := range f.groupAssignments[groupID] {
		if user, exists := f.users[userId]; exists {
			users = append(users, user)
		}
	}

	return users
}

// ListGroups will return the list of groups.
func (f *fakeOktaServer) ListGroups() []*okta.Group {
	f.mu.Lock()
	defer f.mu.Unlock()

	var groups []*okta.Group
	for _, group := range f.groups {
		groups = append(groups, group)
	}

	return groups
}

// GetGroup will return the matching group.
func (f *fakeOktaServer) GetGroup(groupID string) (*okta.Group, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	group, ok := f.groups[groupID]
	return group, ok
}

// GetUser will fetch the profile of the user with the given ID.
func (f *fakeOktaServer) GetUser(userID string) (*okta.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, exists := f.users[userID]
	if !exists {
		return nil, trace.NotFound("user not found")
	}

	return user, nil
}

func (f *fakeOktaServer) GetAppUser(appID, userID string) (*okta.AppUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	appUsers, ok := f.appUserAssignments[appID]
	if !ok {
		return nil, trace.NotFound("app not found")
	}
	user, ok := f.users[userID]
	if !ok {
		return nil, trace.NotFound("user not found")
	}
	if _, ok := appUsers[userID]; !ok {
		return nil, trace.NotFound("user not assigned to the app")
	}
	return oktaUserToOktaAppUser(user), nil
}

// ListUsers will return the list of users.
func (f *fakeOktaServer) ListUsers() []*okta.User {
	f.mu.Lock()
	defer f.mu.Unlock()

	var users []*okta.User
	for _, user := range f.users {
		if user.Status == "DEPROVISIONED" {
			continue
		}
		users = append(users, user)
	}

	return users
}

// ListUserGroups will return the list of groups a user belongs to.
func (f *fakeOktaServer) ListUserGroups(userID string) []*okta.Group {
	f.mu.Lock()
	defer f.mu.Unlock()

	var groups []*okta.Group
	for groupId, users := range f.groupAssignments {
		if _, ok := users[userID]; ok {
			if group, exists := f.groups[groupId]; exists {
				groups = append(groups, group)
			}
		}
	}

	return groups
}

// ListApplications will return the list of applications.
func (f *fakeOktaServer) ListApplications(filter string) []okta.App {
	f.mu.Lock()
	defer f.mu.Unlock()

	var apps []okta.App
	for _, app := range f.applications {
		buff, err := json.Marshal(app)
		if err != nil {
			panic(err)
		}
		var item okta.Application
		if err := json.Unmarshal(buff, &item); err != nil {
			panic(err)
		}
		if filter != "" && (item.Name != filter && item.Label != filter) {
			continue
		}
		apps = append(apps, &item)
	}

	return apps
}

// ListApplicationUsers will return the list of users assigned to the application.
func (f *fakeOktaServer) ListApplicationUsers(appID string) []*okta.AppUser {
	f.mu.Lock()
	defer f.mu.Unlock()

	var appUsers []*okta.AppUser
	for userId := range f.appUserAssignments[appID] {
		u := f.users[userId]
		if u.Status == "DEPROVISIONED" {
			continue
		}
		appUsers = append(appUsers, oktaUserToOktaAppUser(u))
	}

	return appUsers
}

// ListApplicationGroupAssignments will return the list of group assignments for the application.
func (f *fakeOktaServer) ListApplicationGroupAssignments(appID string) []*okta.ApplicationGroupAssignment {
	f.mu.Lock()
	defer f.mu.Unlock()

	var assignments []*okta.ApplicationGroupAssignment
	if groups, exists := f.appGroupAssignments[appID]; exists {
		for _, assignment := range groups {
			assignments = append(assignments, assignment)
		}
	}

	return assignments
}

// RemoveUserFromGroup will remove the user from the group.
func (f *fakeOktaServer) RemoveUserFromGroup(groupID string, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.groupAssignments[groupID] != nil {
		delete(f.groupAssignments[groupID], userID)
	}
}

// CreateApplicationGroupAssignment will create a new group assignment for the application.
func (f *fakeOktaServer) CreateApplicationGroupAssignment(appID string, groupID string) *okta.ApplicationGroupAssignment {
	f.mu.Lock()
	defer f.mu.Unlock()

	assignment := okta.ApplicationGroupAssignment{
		Id: groupID,
	}

	if f.appGroupAssignments[appID] == nil {
		f.appGroupAssignments[appID] = make(map[string]*okta.ApplicationGroupAssignment)
	}
	f.appGroupAssignments[appID][groupID] = &assignment

	return &assignment
}

// DeleteApplicationUser will remove the user from the application.
func (f *fakeOktaServer) DeleteApplicationUser(appID string, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.appUserAssignments[appID] != nil {
		delete(f.appUserAssignments[appID], userID)
	}
}

func oktaUserToOktaAppUser(u *okta.User) *okta.AppUser {
	return &okta.AppUser{
		Id:          u.Id,
		Credentials: &okta.AppUserCredentials{},
		Status:      u.Status,
		Profile:     map[string]any(*u.Profile),
		Scope:       string(oktaapi.UserScope),
	}
}
