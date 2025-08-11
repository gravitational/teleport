package oktaapi

import (
	"context"
	"errors"
	"maps"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (
	TestOrgURL = "https://test.okta.example.com"
)

// OktaClientFn is a function interface for creating Okta client.
type OktaClientFn func(context.Context, Config) (Interface, error)

// TestOktaClient is a testing Okta client that is backed by fixed values.
type TestOktaClient struct {
	OktaUsers    []*okta.User
	OktaAppUsers []*okta.AppUser
	OktaGroups   []*okta.Group
	OktaApps     []okta.App
	OktaOrgURL   string

	UsernamesToUserIDs utils.SyncMap[UserName, OktaUserID]

	// groupsToUsers is a mapping of group IDs to users that have been assigned to them.
	GroupsToUsers utils.SyncMap[OktaGroupID, set.Set[OktaUserID]]
	// appsToUsers is a mapping of application IDs to users that have been assigned to them.
	AppsToUsers utils.SyncMap[OktaAppID, set.Set[AppAssignment]]

	AppsToGroups map[OktaAppID][]OktaGroupID

	UnassignGroupErr map[OktaGroupID]error
	UnassignAppErr   map[OktaAppID]error

	// monkeyPatch allows individual tests to override the default
	// TestOktaClient behavior in cases where it is difficult to rig the
	// internal state in the way necessary for a test.
	MonkeyPatch struct {
		CreateApp                func(ctx context.Context, app okta.App) (okta.App, error)
		AssignGroupToApplication func(ctx context.Context, groupId OktaGroupID, appId OktaAppID) error
		IterateAppUsers          func(context.Context, OktaAppID, func(*okta.AppUser) error) error
		GetAppAssignments        func(context.Context, OktaAppID) ([]AppAssignment, error)
		GetAppGroups             func(context.Context, OktaAppID) ([]OktaGroupID, error)
		GetGroupAssignments      func(context.Context, OktaGroupID) ([]OktaUserID, error)
		DoHttp                   func(context.Context, string, *url.URL, []string) ([]byte, error)
		OrgName                  func(context.Context) (string, error)
	}
}

// Static assertion that TestOktaClient actually implements OktaClient
var _ Interface = (*TestOktaClient)(nil)

func NewTestClient() *TestOktaClient {
	return &TestOktaClient{
		AppsToGroups:     map[OktaAppID][]OktaGroupID{},
		UnassignGroupErr: map[OktaGroupID]error{},
		UnassignAppErr:   map[OktaAppID]error{},
		OktaOrgURL:       TestOrgURL,
	}
}

func CreatorFromTestClient(testClient *TestOktaClient) OktaClientFn {
	return func(_ context.Context, _ Config) (Interface, error) {
		return testClient, nil
	}
}

// GetCurrentUser always returns NotImplemented
func (t *TestOktaClient) GetCurrentUser(_ context.Context) (*okta.User, error) {
	return nil, trace.NotImplemented("getCurrentUser")
}

// GetApplication always returns NotImplemented
func (t *TestOktaClient) GetApplication(context.Context, OktaAppID, okta.App) (okta.App, error) {
	return nil, trace.NotImplemented("getApplication")
}

// IterateUsers will iterate over the list of all Okta users.
func (t *TestOktaClient) IterateUsers(_ context.Context, fn func(*okta.User) error, paramOpts ...query.ParamOptions) error {
	for _, oktaUser := range t.OktaUsers {
		if err := fn(oktaUser); err != nil {
			if errors.Is(err, ErrStopIteration) {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// IterateAppUsers will iterate over the list of all Okta users in a given app.
func (t *TestOktaClient) IterateAppUsers(ctx context.Context, appID OktaAppID, fn func(*okta.AppUser) error) error {
	if t.MonkeyPatch.IterateAppUsers != nil {
		return t.MonkeyPatch.IterateAppUsers(ctx, appID, fn)
	}
	for _, oktaAppUser := range t.OktaAppUsers {
		if err := fn(oktaAppUser); err != nil {
			if errors.Is(err, ErrStopIteration) {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

func (t *TestOktaClient) ListUserGroups(ctx context.Context, userID string) ([]UserGroup, error) {
	return []UserGroup{}, nil
}

// IterateGroups will iterate over the list of all Okta groups.
func (t *TestOktaClient) IterateGroups(_ context.Context, fn func(*okta.Group) error) error {
	for _, oktaGroup := range t.OktaGroups {
		if err := fn(oktaGroup); err != nil {
			if errors.Is(err, ErrStopIteration) {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// IterateApps will iterate over the list of all Okta applications.
func (t *TestOktaClient) IterateApps(_ context.Context, fn func(okta.App) error, _ ...query.ParamOptions) error {
	for _, oktaApp := range t.OktaApps {
		if err := fn(oktaApp); err != nil {
			if errors.Is(err, ErrStopIteration) {
				break
			}
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetGroupAssignments will return the list of users assigned to a group.
func (t *TestOktaClient) GetGroupAssignments(ctx context.Context, groupID OktaGroupID) ([]OktaUserID, error) {
	if t.MonkeyPatch.GetGroupAssignments != nil {
		return t.MonkeyPatch.GetGroupAssignments(ctx, groupID)
	}
	return t.GetTestGroupAssignments(groupID)
}

// GetAppAssignments will return the list of users assigned to an app.
func (t *TestOktaClient) GetAppAssignments(ctx context.Context, appID OktaAppID) ([]AppAssignment, error) {
	if t.MonkeyPatch.GetAppAssignments != nil {
		return t.MonkeyPatch.GetAppAssignments(ctx, appID)
	}
	return t.GetTestAppAssignments(appID)
}

// ListUsers will return a mapping of usernames to user IDs from Okta.
func (t *TestOktaClient) ListUsers(_ context.Context, _ ...query.ParamOptions) (map[UserName]OktaUserID, error) {
	return t.UsernamesToUserIDs.Clone(), nil
}

// AddUserID will add a mapping from the username to the user ID.
func (t *TestOktaClient) AddUserID(username UserName, userID OktaUserID) {
	t.UsernamesToUserIDs.Store(username, userID)
}

// AddGroupToMapping will add the given group to the group to user mapping in the test client.
func (t *TestOktaClient) AddGroupToMapping(groupId string) {
	t.GroupsToUsers.Write(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		// Don't overwrite the existing mapping if it exists.
		if _, ok := groupsToUsers[OktaGroupID(groupId)]; ok {
			return
		}

		t.OktaGroups = append(t.OktaGroups, &okta.Group{Id: groupId})
		groupsToUsers[OktaGroupID(groupId)] = set.New[OktaUserID]()
	})

}

// AddOktaGroupToMapping will add the given Okta group to the group to user mapping in the test client.
func (t *TestOktaClient) AddOktaGroupToMapping(group *okta.Group) {
	t.GroupsToUsers.Write(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		t.OktaGroups = append(t.OktaGroups, group)
		groupsToUsers[OktaGroupID(group.Id)] = set.New[OktaUserID]()
	})
}

// AssignUserToGroup will assign the given user to the group.
func (t *TestOktaClient) AssignUserToGroup(_ context.Context, userID OktaUserID, groupID OktaGroupID) error {
	var err error

	t.GroupsToUsers.Write(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		if members, ok := groupsToUsers[groupID]; ok {
			members.Add(userID)
			return
		}
		err = trace.NotFound("provision: unable to find group %s", groupID)
	})

	return err
}

// UnassignUserFromGroup will unassign the given user from the group.
func (t *TestOktaClient) UnassignUserFromGroup(_ context.Context, userID OktaUserID, groupID OktaGroupID) error {

	if err, ok := t.UnassignGroupErr[groupID]; ok {
		return err
	}

	var err error

	t.GroupsToUsers.Write(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		if members, ok := groupsToUsers[groupID]; ok {
			members.Remove(userID)
			return
		}
		err = trace.NotFound("cleanup: unable to find group %s", groupID)
	})

	return err
}

// AddApplicationToMapping will add the given application to the application to user mapping in the test client.
func (t *TestOktaClient) AddApplicationToMapping(applicationId string) {

	t.AppsToUsers.Store(OktaAppID(applicationId), set.New[AppAssignment]())

	t.OktaApps = append(t.OktaApps, &okta.Application{
		Id: applicationId,
	})
}

// AddOktaApplicationToMapping will add the given Okta application to the application to user mapping in the test client.
func (t *TestOktaClient) AddOktaApplicationToMapping(application okta.App) {
	t.OktaApps = append(t.OktaApps, application)

	// Only add in the mapping if this is an actual *okta.Application object, otherwise skip.
	if oktaApp, ok := application.(*okta.Application); ok {
		t.AppsToUsers.Store(OktaAppID(oktaApp.Id), set.New[AppAssignment]())
	}
}

// AssignUserToApplication will assign the given user to the application.
func (t *TestOktaClient) AssignUserToApplication(_ context.Context, userID OktaUserID, applicationId OktaAppID) error {
	var err error

	t.AppsToUsers.Write(func(appsToUsers map[OktaAppID]set.Set[AppAssignment]) {
		if assignments, ok := appsToUsers[applicationId]; ok {
			assignments.Add(AppAssignment{UserID: string(userID), Scope: UserScope})
			return
		}
		err = trace.NotFound("provision: unable to find application %s", applicationId)
	})

	return err
}

// UnassignUserFromApplication will unassign the given user from the application.
func (t *TestOktaClient) UnassignUserFromApplication(_ context.Context, userID OktaUserID, applicationId OktaAppID) error {
	if err, ok := t.UnassignAppErr[applicationId]; ok {
		return err
	}

	var err error

	t.AppsToUsers.Write(
		func(appsToUsers map[OktaAppID]set.Set[AppAssignment]) {
			if assignments, ok := appsToUsers[applicationId]; ok {
				assignments.Remove(AppAssignment{UserID: string(userID), Scope: UserScope})
				return
			}
			err = trace.NotFound("cleanup: unable to find application %s", applicationId)
		})

	return err
}

func (t *TestOktaClient) AssignGroupToApplication(ctx context.Context, groupId OktaGroupID, appId OktaAppID) error {
	if t.MonkeyPatch.AssignGroupToApplication != nil {
		return t.MonkeyPatch.AssignGroupToApplication(ctx, groupId, appId)
	}
	return trace.NotImplemented("assignGroupToApplication")
}

func (t *TestOktaClient) CreateApplication(ctx context.Context, app okta.App) (okta.App, error) {
	if t.MonkeyPatch.CreateApp != nil {
		return t.MonkeyPatch.CreateApp(ctx, app)
	}
	return nil, trace.NotImplemented("createApp")
}

// GetOrgUrl will return the org URL for the client.
func (t *TestOktaClient) GetOrgUrl() string {
	return t.OktaOrgURL
}

func (t *TestOktaClient) OrgName(ctx context.Context) (string, error) {
	if t.MonkeyPatch.OrgName != nil {
		return t.MonkeyPatch.OrgName(ctx)
	}
	return "", nil
}

func (t *TestOktaClient) DoHttp(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
	if t.MonkeyPatch.DoHttp != nil {
		return t.MonkeyPatch.DoHttp(ctx, method, url, accept)
	}
	return nil, trace.NotImplemented("doHttp")
}

func (t *TestOktaClient) AddGroupAssignments(groupID string, users ...string) {
	t.GroupsToUsers.Write(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		var assignedUsers set.Set[OktaUserID]
		var ok bool

		if assignedUsers, ok = groupsToUsers[OktaGroupID(groupID)]; !ok {
			assignedUsers = set.New[OktaUserID]()
			groupsToUsers[OktaGroupID(groupID)] = assignedUsers
		}

		for _, user := range users {
			assignedUsers.Add(OktaUserID(user))
		}
	})
}

func (t *TestOktaClient) AddAppAssignments(appID string, users ...string) {
	t.AppsToUsers.Write(func(appsToUsers map[OktaAppID]set.Set[AppAssignment]) {
		var assignments set.Set[AppAssignment]
		var ok bool

		if assignments, ok = appsToUsers[OktaAppID(appID)]; !ok {
			assignments = set.New[AppAssignment]()
			appsToUsers[OktaAppID(appID)] = assignments
		}

		for _, user := range users {
			assignments.Add(AppAssignment{UserID: user, Scope: UserScope})
		}
	})
}

type DummyOktaApp struct{}

func (d *DummyOktaApp) IsApplicationInstance() bool {
	return false
}

// GetTestAppAssignments will return the pre-configured list of users assigned
// to an app.  This provides the default behavior for
// OktaClient.getAppAssignments().
func (t *TestOktaClient) GetTestAppAssignments(appID OktaAppID) ([]AppAssignment, error) {
	var err error
	var assignments []AppAssignment

	t.AppsToUsers.Read(func(appsToUsers map[OktaAppID]set.Set[AppAssignment]) {
		users, ok := appsToUsers[appID]
		if !ok {
			err = trace.NotFound("assignments for app %s not found", appID)
			return
		}

		assignments = make([]AppAssignment, 0, len(users))
		for user := range users {
			assignments = append(assignments, user)
		}
	})

	return assignments, err
}

// GetTestGroupAssignments returns the pre-configured group assignments. This
// provides the default behavior for OktaClient.getGroupAssignments().
func (t *TestOktaClient) GetTestGroupAssignments(groupID OktaGroupID) ([]OktaUserID, error) {
	var err error
	var users []OktaUserID

	t.GroupsToUsers.Read(func(groupsToUsers map[OktaGroupID]set.Set[OktaUserID]) {
		members, ok := groupsToUsers[groupID]
		if !ok {
			err = trace.NotFound("assignments for group %s not found", groupID)
			return
		}

		for k := range maps.Keys(members) {
			users = append(users, k)
		}

	})

	return users, err
}

// GetAppGroups will return the list of groups an application belongs to.
func (t *TestOktaClient) GetAppGroups(ctx context.Context, appID OktaAppID) ([]OktaGroupID, error) {
	if t.MonkeyPatch.GetAppGroups != nil {
		return t.MonkeyPatch.GetAppGroups(ctx, appID)
	}
	return t.AppsToGroups[appID], nil
}

// GetScopes returns the list of scopes that the Okta client has access to.
func (t *TestOktaClient) GetAuthorizedScopes(ctx context.Context) ([]string, error) {
	return oktaAPIScopes, nil
}

func (w *TestOktaClient) ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listLogEvents")
}
func (w *TestOktaClient) ListApiTokens(ctx context.Context, qp *query.Params) ([]*ApiToken, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listApiTokens")
}
func (w *TestOktaClient) ListUsersWithRoleAssignments(ctx context.Context) (*RoleAssignedUsers, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listUsersWithRoleAssignments")
}

func (w *TestOktaClient) ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error) {
	return nil, nil, trace.NotImplemented("listAssignedRolesForUser")
}
