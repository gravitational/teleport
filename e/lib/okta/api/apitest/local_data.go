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
	users  utils.SyncMap[oktaapi.UserName, oktaapi.OktaUserID]
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
		users:  utils.SyncMap[oktaapi.UserName, oktaapi.OktaUserID]{},
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

func (d *LocalData) UpsertUser(username oktaapi.UserName, userId oktaapi.OktaUserID) {
	d.users.Store(username, userId)
}

func (d *LocalData) DeleteUser(username oktaapi.UserName) {
	d.users.Delete(username)
}

func (d *LocalData) UpsertAppForId(appId oktaapi.OktaAppID) {
	d.UpsertApp(&okta.Application{Id: string(appId)})
}

func (d *LocalData) UpsertApp(app *okta.Application) {
	d.apps.Store(oktaapi.OktaAppID(app.Id), app)
}

func (d *LocalData) DeleteApp(appId oktaapi.OktaAppID) {
	d.apps.Delete(appId)
	d.appsToGroups.Delete(appId)
	d.appsToUsers.Delete(appId)
}

func (d *LocalData) UpsertGroupForId(groupId oktaapi.OktaGroupID) {
	d.UpsertGroup(&okta.Group{Id: string(groupId)})
}

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
		ListUsersFunc: func(_ *testing.T, _ context.Context, _ ...query.ParamOptions) (map[oktaapi.UserName]oktaapi.OktaUserID, error) {
			return d.users.Clone(), nil
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
