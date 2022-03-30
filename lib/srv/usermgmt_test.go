/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"fmt"
	"os/user"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type testHostUserBackend struct {
	users  map[string][]string
	groups map[string]string
}

func newTestUserMgmt() *testHostUserBackend {
	return &testHostUserBackend{
		users:  map[string][]string{},
		groups: map[string]string{},
	}
}

func (tm *testHostUserBackend) GetAllUsers() ([]string, error) {
	keys := make([]string, 0, len(tm.users))
	for key := range tm.users {
		keys = append(keys, key)
	}
	return keys, nil
}

func (tm *testHostUserBackend) Lookup(username string) (*user.User, error) {
	if _, ok := tm.users[username]; !ok {
		return nil, nil
	}
	return &user.User{
		Username: username,
	}, nil
}

func (tm *testHostUserBackend) LookupGroup(groupname string) (*user.Group, error) {
	return &user.Group{
		Gid:  tm.groups[groupname],
		Name: groupname,
	}, nil
}

func (tm *testHostUserBackend) UserGIDs(u *user.User) ([]string, error) {
	ids := make([]string, 0, len(tm.users[u.Username]))
	for _, id := range tm.users[u.Username] {
		ids = append(ids, tm.groups[id])
	}
	return ids, nil
}

func (tm *testHostUserBackend) CreateGroup(group string) error {
	_, ok := tm.groups[group]
	if ok {
		return trace.AlreadyExists("Group %q, already exists", group)
	}
	tm.groups[group] = fmt.Sprint(len(tm.groups) + 1)
	return nil
}

func (tm *testHostUserBackend) CreateUser(user string, groups []string) error {
	_, ok := tm.users[user]
	if ok {
		return trace.AlreadyExists("Group %q, already exists", user)
	}
	tm.users[user] = groups
	return nil
}

func (tm *testHostUserBackend) DeleteUser(user string) error {
	delete(tm.users, user)
	return nil
}

var _ HostUsersBackend = &testHostUserBackend{}

func TestUserMgmt_CreateTemporaryUser(t *testing.T) {
	backend := newTestUserMgmt()
	users := HostUserManagement{backend: backend}

	userinfo := &services.HostUsersInfo{Groups: []string{"hello", "sudo"}}
	// create a user with some groups
	_, closer, err := users.CreateUser("bob", userinfo)
	require.NoError(t, err)
	require.NotNil(t, closer, "user closer was nil")

	// temproary users must always include the teleport-service group
	require.Equal(t, []string{
		"hello", "sudo", types.TeleportServiceGroup,
	}, backend.users["bob"])

	// try creat the same user again
	_, secondCloser, err := users.CreateUser("bob", userinfo)
	require.True(t, trace.IsAlreadyExists(err))
	require.NotNil(t, secondCloser)

	// Close will remove the user if the user is in the teleport-system group
	require.NoError(t, closer.Close())
	require.NotContains(t, backend.users, "bob")

	backend.CreateGroup("testgroup")
	backend.CreateUser("simon", []string{})

	// try to create a temporary user for simon
	_, closer, err = users.CreateUser("simon", userinfo)
	require.True(t, trace.IsAlreadyExists(err))
	require.NotNil(t, closer)

	// close should not delete simon as they already existed outside
	// of the teleport-system group
	require.NoError(t, closer.Close())
	require.Contains(t, backend.users, "simon")
}

func TestUserMgmt_DeleteAllTeleportSystemUsers(t *testing.T) {
	type userAndGroups struct {
		user   string
		groups []string
	}

	usersDB := []userAndGroups{
		{"fgh", []string{"teleport-system"}},
		{"xyz", []string{"teleport-system"}},
		{"pqr", []string{"not-deleted"}},
		{"abc", []string{"not-deleted"}},
	}

	remainingUsers := []string{"pqr", "abc"}

	mgmt := newTestUserMgmt()
	users := HostUserManagement{
		backend: mgmt,
	}

	for _, user := range usersDB {
		for _, group := range user.groups {
			mgmt.CreateGroup(group)
		}
		mgmt.CreateUser(user.user, user.groups)
	}

	require.NoError(t, users.DeleteAllUsers())
	resultingUsers, err := mgmt.GetAllUsers()
	require.NoError(t, err)

	require.ElementsMatch(t, remainingUsers, resultingUsers)
}
