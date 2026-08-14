package oktaapitest

import (
	context "context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing" //nolint:depguard // this a shared test package

	"github.com/gravitational/trace"
	okta "github.com/okta/okta-sdk-golang/v2/okta"
	query "github.com/okta/okta-sdk-golang/v2/okta/query"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
)

// LocalData is a structure holds in-memory data representing Okta state which can be manipulated
// as needed during tests. Use [[NewLocalDataClient]] to create a [[Client]] backed with LocalData.
// LocalData can be changed at any time of the Client's lifecycle.
type LocalData struct {
	// data
	apps   utils.SyncMap[oktaapi.OktaAppID, *okta.Application]
	groups utils.SyncMap[oktaapi.OktaGroupID, *okta.Group]
	users  utils.SyncMap[oktaapi.UserName, *okta.User]
	// relationships
	groupsToUsers utils.SyncMap[oktaapi.OktaGroupID, set.Set[oktaapi.OktaUserID]]
	appsToUsers   utils.SyncMap[oktaapi.OktaAppID, set.Set[oktaapi.OktaUserID]]
	appsToGroups  utils.SyncMap[oktaapi.OktaAppID, set.Set[oktaapi.OktaGroupID]]
}

func newLocalData() *LocalData {
	return &LocalData{
		// data
		apps:   utils.SyncMap[oktaapi.OktaAppID, *okta.Application]{},
		groups: utils.SyncMap[oktaapi.OktaGroupID, *okta.Group]{},
		users:  utils.SyncMap[oktaapi.UserName, *okta.User]{},
		// relationships
		groupsToUsers: utils.SyncMap[oktaapi.OktaGroupID, set.Set[oktaapi.OktaUserID]]{},
		appsToUsers:   utils.SyncMap[oktaapi.OktaAppID, set.Set[oktaapi.OktaUserID]]{},
		appsToGroups:  utils.SyncMap[oktaapi.OktaAppID, set.Set[oktaapi.OktaGroupID]]{},
	}
}

// GetGroupUserAssignments is used for test assertions.
func (d *LocalData) GetGroupUserAssignments() map[oktaapi.OktaGroupID]set.Set[oktaapi.OktaUserID] {
	return d.groupsToUsers.Clone()
}

// GetAppUserAssignments is used for test assertions.
func (d *LocalData) GetAppUserAssignments() map[oktaapi.OktaAppID]set.Set[oktaapi.OktaUserID] {
	return d.appsToUsers.Clone()
}

// UpsertUserForId is used set up testing Okta org user.  It's a convenience function for UpsertUser.
func (d *LocalData) UpsertUserForId(username oktaapi.UserName, id string) {
	u := NewOrgUser(UserArgs{Name: string(username), ID: id, Status: StatusActive})
	d.UpsertUser(u)
}

// UpsertUser is used set up testing Okta org user. [NewOrgUser] helper can be used to create the
// user struct.
func (d *LocalData) UpsertUser(u *okta.User) {
	username := getProfileLogin(u.Profile)
	d.users.Store(oktaapi.UserName(username), u)
}

func (d *LocalData) DeleteUser(username oktaapi.UserName) {
	d.users.Delete(username)
}

// UpsertApp is used set up testing Okta application. It's a convenience function for UpsertApp.
func (d *LocalData) UpsertAppForId(appId oktaapi.OktaAppID) {
	d.UpsertApp(NewApplication(ApplicationArgs{
		ID:     string(appId),
		Label:  "Test Application " + string(appId),
		Status: StatusActive,
		Links:  []AppLink{{Name: TestLink1Name, Href: TestLink1Href}},
	}))
}

// UpsertApp is used set up testing Okta application. [NewApplication] helper can be used to create the
// application struct.
func (d *LocalData) UpsertApp(app *okta.Application) {
	d.apps.Store(oktaapi.OktaAppID(app.Id), app)
}

func (d *LocalData) DeleteApp(appId oktaapi.OktaAppID) {
	d.apps.Delete(appId)
	d.appsToGroups.Delete(appId)
	d.appsToUsers.Delete(appId)
}

func (d *LocalData) UpsertGroupForId(groupId oktaapi.OktaGroupID) {
	d.UpsertGroup(NewGroup(GroupArgs{ID: string(groupId), Name: "Name of group " + string(groupId)}))
}

// UpsertGroup sets up Okta group for testing. You can user [NewGroup] to create the group struct.
func (d *LocalData) UpsertGroup(group *okta.Group) {
	d.groups.Store(oktaapi.OktaGroupID(group.Id), group)
}

func (d *LocalData) DeleteGroup(groupId oktaapi.OktaGroupID) {
	d.groups.Delete(groupId)
	d.groupsToUsers.Delete(groupId)
}

func (d *LocalData) UpsertGroupUserAssignments(groupId oktaapi.OktaGroupID, userIds ...oktaapi.OktaUserID) {
	if _, ok := d.groups.Load(groupId); !ok {
		panic(fmt.Sprintf("group with ID %q not found, call UpsertGroup or UpsertGroupForId first", groupId))
	}
	d.groupsToUsers.Store(groupId, set.New(userIds...))
}

func (d *LocalData) UpsertAppUserAssignments(appId oktaapi.OktaAppID, userIds ...oktaapi.OktaUserID) {
	if _, ok := d.apps.Load(appId); !ok {
		panic(fmt.Sprintf("application with ID %q not found, call UpsertApp or UpsertAppForId first", appId))
	}
	d.appsToUsers.Store(appId, set.New(userIds...))
}

func (d *LocalData) UpsertAppGroups(appId oktaapi.OktaAppID, groupIds ...oktaapi.OktaGroupID) {
	if _, ok := d.apps.Load(appId); !ok {
		panic(fmt.Sprintf("application with ID %q not found, call UpsertApp or UpsertAppForId first", appId))
	}
	d.appsToGroups.Store(appId, set.New(groupIds...))
}

// newClientFuncs creates ClientFuncs operating on the local data.
func (d *LocalData) newClientFuncs() ClientFuncs {
	return ClientFuncs{
		ListUsersFunc: func(_ *testing.T, _ context.Context, paramOpt ...query.ParamOptions) (map[oktaapi.UserName]oktaapi.OktaUserID, error) {
			p := &query.Params{}
			for _, par := range paramOpt {
				par(p)
			}
			switch p.Filter {
			case "":
			case `status eq "DEPROVISIONED"`:
				return nil, nil
			case `status eq "SUSPENDED"`:
				return nil, nil
			default:
				panic("unhandled filter: " + p.Filter)
			}

			m := make(map[oktaapi.UserName]oktaapi.OktaUserID, d.users.Len())
			for k, u := range d.users.Range {
				m[k] = oktaapi.OktaUserID(u.Id)
			}
			return m, nil
		},

		IterateUsersFunc: func(_ *testing.T, ctx context.Context, fn func(*okta.User) error, queryParams ...query.ParamOptions) error {
			for _, u := range d.users.Range {
				if err := fn(u); err != nil {
					if errors.Is(err, oktaapi.ErrStopIteration) {
						break
					}
					return trace.Wrap(err)
				}
			}
			return nil
		},

		IterateAppsFunc: func(_ *testing.T, _ context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
			for _, a := range d.apps.Range {
				if err := fn(a); err != nil {
					if errors.Is(err, oktaapi.ErrStopIteration) {
						break
					}
					return trace.Wrap(err)
				}
			}
			return nil
		},

		IterateGroupsFunc: func(_ *testing.T, _ context.Context, fn func(*okta.Group) error) error {
			for _, g := range d.groups.Range {
				if err := fn(g); err != nil {
					if errors.Is(err, oktaapi.ErrStopIteration) {
						break
					}
					return trace.Wrap(err)
				}
			}
			return nil
		},

		ListUserGroupsFunc: func(t *testing.T, ctx context.Context, userID string) ([]oktaapi.UserGroup, error) {
			res := make([]oktaapi.UserGroup, 0, d.groups.Len())
			for _, g := range d.groups.Range {
				res = append(res, oktaapi.UserGroup{
					ID:   g.Id,
					Name: g.Profile.Name,
				})
			}
			return res, nil
		},

		GetAppGroupsFunc: func(_ *testing.T, _ context.Context, appId oktaapi.OktaAppID) (groups []oktaapi.OktaGroupID, err error) {
			if _, ok := d.apps.Load(appId); !ok {
				return nil, trace.NotFound("application %q not found", appId)
			}
			d.appsToGroups.Read(func(appsToGroups map[oktaapi.OktaAppID]set.Set[oktaapi.OktaGroupID]) {
				if groupIds, ok := appsToGroups[appId]; ok {
					groups = slices.Collect(maps.Keys(groupIds))
				}
			})
			return groups, nil
		},

		GetAppAssignmentsFunc: func(_ *testing.T, _ context.Context, appId oktaapi.OktaAppID) (assignments []oktaapi.AppAssignment, err error) {
			if _, ok := d.apps.Load(appId); !ok {
				return nil, trace.NotFound("application %q not found", appId)
			}
			d.appsToUsers.Read(func(appsToUsers map[oktaapi.OktaAppID]set.Set[oktaapi.OktaUserID]) {
				for u := range appsToUsers[appId] {
					assignments = append(assignments, oktaapi.AppAssignment{UserID: string(u), Scope: oktaapi.UserScope})
				}
			})
			return assignments, nil
		},

		GetGroupAssignmentsFunc: func(_ *testing.T, _ context.Context, groupId oktaapi.OktaGroupID) (users []oktaapi.OktaUserID, err error) {
			if _, ok := d.groups.Load(groupId); !ok {
				return nil, trace.NotFound("group %q not found", groupId)
			}
			d.groupsToUsers.Read(func(groupsToUsers map[oktaapi.OktaGroupID]set.Set[oktaapi.OktaUserID]) {
				if userIds, ok := groupsToUsers[groupId]; ok {
					users = slices.Collect(maps.Keys(userIds))
				}
			})
			return users, nil
		},

		AssignUserToGroupFunc: func(_ *testing.T, _ context.Context, userId oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) (err error) {
			if _, ok := d.groups.Load(groupId); !ok {
				return trace.NotFound("group %q not found", groupId)
			}
			addToRelation(&d.groupsToUsers, groupId, userId)
			return nil
		},

		UnassignUserFromGroupFunc: func(_ *testing.T, _ context.Context, userId oktaapi.OktaUserID, groupId oktaapi.OktaGroupID) (err error) {
			if _, ok := d.groups.Load(groupId); !ok {
				return trace.NotFound("group %q not found", groupId)
			}
			rmFromRelation(&d.groupsToUsers, groupId, userId)
			return err
		},

		AssignUserToApplicationFunc: func(_ *testing.T, _ context.Context, userId oktaapi.OktaUserID, appId oktaapi.OktaAppID) (err error) {
			if _, ok := d.apps.Load(appId); !ok {
				return trace.NotFound("application %q not found", appId)
			}
			addToRelation(&d.appsToUsers, appId, userId)
			return nil
		},

		UnassignUserFromApplicationFunc: func(_ *testing.T, _ context.Context, userId oktaapi.OktaUserID, appId oktaapi.OktaAppID) (err error) {
			if _, ok := d.apps.Load(appId); !ok {
				return trace.NotFound("application %q not found", appId)
			}
			rmFromRelation(&d.appsToUsers, appId, userId)
			return nil
		},
	}
}

func addToRelation[K comparable, V comparable](relation *utils.SyncMap[K, set.Set[V]], k K, v V) {
	relation.Write(func(m map[K]set.Set[V]) {
		if s, ok := m[k]; ok {
			s.Add(v)
		} else {
			m[k] = set.New(v)
		}
	})
}

func rmFromRelation[K comparable, V comparable](relation *utils.SyncMap[K, set.Set[V]], k K, v V) {
	relation.Write(func(m map[K]set.Set[V]) {
		if s, ok := m[k]; ok {
			s.Remove(v)
			if len(s) == 0 {
				delete(m, k)
			}
		}
	})
}

func getProfileLogin(profile *okta.UserProfile) string {
	if profile == nil {
		panic("profile is nil")
	}
	login, ok := (*profile)["login"]
	if !ok {
		panic("profile does not have 'login' key")
	}
	return login.(string)
}
