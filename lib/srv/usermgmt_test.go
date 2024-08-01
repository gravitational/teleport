/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"errors"
	"fmt"
	"os/user"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type testHostUserBackend struct {
	// users: user -> []groups
	users map[string][]string
	// groups: group -> groupid
	groups map[string]string
	// sudoers: user -> entries
	sudoers map[string]string
	// userUID: user -> uid
	userUID map[string]string
	// userGID: user -> gid
	userGID map[string]string
}

func newTestUserMgmt() *testHostUserBackend {
	return &testHostUserBackend{
		users:   map[string][]string{},
		groups:  map[string]string{},
		sudoers: map[string]string{},
		userUID: map[string]string{},
		userGID: map[string]string{},
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

func (tm *testHostUserBackend) LookupGroupByID(gid string) (*user.Group, error) {
	return &user.Group{
		Gid:  tm.groups[gid],
		Name: gid,
	}, nil
}

func (tm *testHostUserBackend) UserGIDs(u *user.User) ([]string, error) {
	ids := make([]string, 0, len(tm.users[u.Username]))
	for _, id := range tm.users[u.Username] {
		ids = append(ids, tm.groups[id])
	}
	return ids, nil
}

func (tm *testHostUserBackend) CreateGroup(group, gid string) error {
	_, ok := tm.groups[group]
	if ok {
		return trace.AlreadyExists("Group %q, already exists", group)
	}
	tm.groups[group] = fmt.Sprint(len(tm.groups) + 1)
	return nil
}

func (tm *testHostUserBackend) CreateUser(user string, groups []string, home, uid, gid string) error {
	_, ok := tm.users[user]
	if ok {
		return trace.AlreadyExists("Group %q, already exists", user)
	}
	tm.users[user] = groups
	tm.userUID[user] = uid
	tm.userGID[user] = gid
	return nil
}

func (tm *testHostUserBackend) DeleteUser(user string) error {
	delete(tm.users, user)
	return nil
}

func (tm *testHostUserBackend) CreateHomeDirectory(user, uid, gid string) error {
	return nil
}

// RemoveSudoersFile implements HostUsersBackend
func (tm *testHostUserBackend) RemoveSudoersFile(user string) error {
	delete(tm.sudoers, user)
	return nil
}

// CheckSudoers implements HostUsersBackend
func (*testHostUserBackend) CheckSudoers(contents []byte) error {
	if strings.Contains(string(contents), "validsudoers") {
		return nil
	}
	return errors.New("invalid")
}

// WriteSudoersFile implements HostUsersBackend
func (tm *testHostUserBackend) WriteSudoersFile(user string, entries []byte) error {
	entry := strings.TrimSpace(string(entries))
	err := tm.CheckSudoers([]byte(entry))
	if err != nil {
		return trace.Wrap(err)
	}
	tm.sudoers[user] = entry
	return nil
}

func (*testHostUserBackend) RemoveAllSudoersFiles() error {
	return nil
}

var _ HostUsersBackend = &testHostUserBackend{}
var _ HostSudoersBackend = &testHostUserBackend{}

func TestUserMgmt_CreateTemporaryUser(t *testing.T) {
	t.Parallel()

	backend := newTestUserMgmt()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	pres := local.NewPresenceService(bk)
	users := HostUserManagement{
		backend: backend,
		storage: pres,
	}

	userinfo := &services.HostUsersInfo{
		Groups: []string{"hello", "sudo"},
		Mode:   types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
	}
	// create a user with some groups
	closer, err := users.CreateUser("bob", userinfo)
	require.NoError(t, err)
	require.NotNil(t, closer, "user closer was nil")

	// temproary users must always include the teleport-service group
	require.Equal(t, []string{
		"hello", "sudo", types.TeleportServiceGroup,
	}, backend.users["bob"])

	// try creat the same user again
	secondCloser, err := users.CreateUser("bob", userinfo)
	require.True(t, trace.IsAlreadyExists(err))
	require.NotNil(t, secondCloser)

	// Close will remove the user if the user is in the teleport-system group
	require.NoError(t, closer.Close())
	require.NotContains(t, backend.users, "bob")

	backend.CreateGroup("testgroup", "")
	backend.CreateUser("simon", []string{}, "", "", "")

	// try to create a temporary user for simon
	closer, err = users.CreateUser("simon", userinfo)
	require.True(t, trace.IsAlreadyExists(err))
	require.Nil(t, closer)
}

func TestUserMgmtSudoers_CreateTemporaryUser(t *testing.T) {
	t.Parallel()

	backend := newTestUserMgmt()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	pres := local.NewPresenceService(bk)
	users := HostUserManagement{
		backend: backend,
		storage: pres,
	}
	sudoers := HostSudoersManagement{
		backend: backend,
	}

	closer, err := users.CreateUser("bob", &services.HostUsersInfo{
		Groups: []string{"hello", "sudo"},
		Mode:   types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
	})
	require.NoError(t, err)
	require.NotNil(t, closer)

	require.Empty(t, backend.sudoers)
	sudoers.WriteSudoers("bob", []string{"validsudoers"})
	require.Equal(t, map[string]string{"bob": "bob validsudoers"}, backend.sudoers)
	sudoers.RemoveSudoers("bob")

	require.NoError(t, closer.Close())
	require.Empty(t, backend.sudoers)

	t.Run("no teleport-service group", func(t *testing.T) {
		backend := newTestUserMgmt()
		users := HostUserManagement{
			backend: backend,
			storage: pres,
		}
		// test user already exists but teleport-service group has not yet
		// been created
		backend.CreateUser("testuser", nil, "", "", "")
		_, err := users.CreateUser("testuser", &services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
		})
		require.True(t, trace.IsAlreadyExists(err))
		backend.CreateGroup(types.TeleportServiceGroup, "")
		// IsAlreadyExists error when teleport-service group now exists
		_, err = users.CreateUser("testuser", &services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
		})
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestUserMgmt_DeleteAllTeleportSystemUsers(t *testing.T) {
	t.Parallel()

	type userAndGroups struct {
		user   string
		groups []string
	}

	usersDB := []userAndGroups{
		{"fgh", []string{types.TeleportServiceGroup}},
		{"xyz", []string{types.TeleportServiceGroup}},
		{"pqr", []string{"not-deleted"}},
		{"abc", []string{"not-deleted"}},
	}

	remainingUsers := []string{"pqr", "abc"}

	mgmt := newTestUserMgmt()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	pres := local.NewPresenceService(bk)
	users := HostUserManagement{
		backend:   mgmt,
		storage:   pres,
		userGrace: 0,
	}

	for _, user := range usersDB {
		for _, group := range user.groups {
			mgmt.CreateGroup(group, "")
		}
		if slices.Contains(user.groups, types.TeleportServiceGroup) {
			users.CreateUser(user.user, &services.HostUsersInfo{Groups: user.groups})
		} else {
			mgmt.CreateUser(user.user, user.groups, "", "", "")
		}
	}
	require.NoError(t, users.DeleteAllUsers())
	resultingUsers, err := mgmt.GetAllUsers()
	require.NoError(t, err)

	require.ElementsMatch(t, remainingUsers, resultingUsers)

	users = HostUserManagement{
		backend: newTestUserMgmt(),
		storage: pres,
	}
	// teleport-system group doesnt exist, DeleteAllUsers will return nil, instead of erroring
	require.NoError(t, users.DeleteAllUsers())
}

func TestSudoersSanitization(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		user         string
		userExpected string
	}{
		{
			user:         "testuser",
			userExpected: "testuser",
		},
		{
			user:         "test.user",
			userExpected: "test_user",
		},
		{
			user:         "test.us~er",
			userExpected: "test_us_er",
		},
		{
			user:         "test../../us~er",
			userExpected: "test______us_er",
		},
	} {
		actual := sanitizeSudoersName(tc.user)
		require.Equal(t, tc.userExpected, actual)
	}
}

func TestIsUnknownGroupError(t *testing.T) {
	unknownGroupName := "unknown"
	for _, tc := range []struct {
		err                 error
		isUnknownGroupError bool
	}{
		{
			err:                 user.UnknownGroupError(unknownGroupName),
			isUnknownGroupError: true,
		}, {
			err:                 fmt.Errorf("lookup groupname %s: no such file or directory", unknownGroupName),
			isUnknownGroupError: true,
		},
	} {
		require.Equal(t, tc.isUnknownGroupError, isUnknownGroupError(tc.err, unknownGroupName))
	}
}
