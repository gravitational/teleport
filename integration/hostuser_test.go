//go:build linux
// +build linux

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

package integration

import (
	"context"
	"os/exec"
	"os/user"
	"testing"

	"github.com/cloudflare/cfssl/log"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const testuser = "teleport-testuser"
const testgroup = "teleport-testgroup"

func requireRoot(t *testing.T) {
	t.Helper()
	if !isRoot() {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}
}

func TestRootHostUsersBackend(t *testing.T) {
	requireRoot(t)

	backend := srv.HostUsersProvisioningBackend{}
	t.Cleanup(func() {
		// cleanup users if they got left behind due to a failing test
		host.UserDel(testuser)
		cmd := exec.Command("groupdel", testgroup)
		cmd.Run()
	})

	t.Run("Test CreateGroup", func(t *testing.T) {
		err := backend.CreateGroup(testgroup)
		require.NoError(t, err)
		err = backend.CreateGroup(testgroup)
		require.True(t, trace.IsAlreadyExists(err))
	})

	t.Run("Test CreateUser", func(t *testing.T) {
		err := backend.CreateUser(testuser, []string{testgroup})
		require.NoError(t, err)

		tuser, err := backend.Lookup(testuser)
		require.NoError(t, err)

		group, err := backend.LookupGroup(testgroup)
		require.NoError(t, err)

		tuserGids, err := tuser.GroupIds()
		require.NoError(t, err)
		require.Contains(t, tuserGids, group.Gid)

		err = backend.CreateUser(testuser, []string{})
		require.True(t, trace.IsAlreadyExists(err))

	})

	t.Run("Test DeleteUser", func(t *testing.T) {
		err := backend.DeleteUser(testuser)
		require.NoError(t, err)

		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("Test GetAllUsers", func(t *testing.T) {
		checkUsers := []string{"teleport-usera", "teleport-userb", "teleport-userc"}
		t.Cleanup(func() {
			for _, u := range checkUsers {
				backend.DeleteUser(u)
			}
		})
		for _, u := range checkUsers {
			err := backend.CreateUser(u, []string{})
			require.NoError(t, err)
		}

		users, err := backend.GetAllUsers()
		require.NoError(t, err)
		require.Subset(t, users, append(checkUsers, "root"))
	})
}

func requireUserInGroups(t *testing.T, u *user.User, requiredGroups []string) {
	var userGroups []string
	userGids, err := u.GroupIds()
	require.NoError(t, err)
	for _, gid := range userGids {
		group, err := user.LookupGroupId(gid)
		require.NoError(t, err)
		userGroups = append(userGroups, group.Name)
	}
	require.Subset(t, userGroups, requiredGroups)
}

func cleanupUsersAndGroups(users []string, groups []string) func() {
	return func() {
		for _, group := range groups {
			cmd := exec.Command("groupdel", group)
			err := cmd.Run()
			if err != nil {
				log.Debugf("Error deleting group %s: %s", group, err)
			}
		}
		for _, user := range users {
			host.UserDel(user)
		}
	}
}

func TestRootHostUsers(t *testing.T) {
	requireRoot(t)

	t.Run("test create temporary user and close", func(t *testing.T) {
		users, err := srv.NewHostUsers(context.Background())
		require.NoError(t, err)

		testGroups := []string{"group1", "group2"}
		_, closer, err := users.CreateUser(testuser, &services.HostUsersInfo{Groups: testGroups})
		require.NoError(t, err)

		testGroups = append(testGroups, types.TeleportServiceGroup)
		t.Cleanup(cleanupUsersAndGroups([]string{testuser}, testGroups))

		u, err := user.Lookup(testuser)
		require.NoError(t, err)
		requireUserInGroups(t, u, testGroups)

		require.NoError(t, closer.Close())
		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("test delete all users in teleport service group", func(t *testing.T) {
		users, err := srv.NewHostUsers(context.Background())
		require.NoError(t, err)

		deleteableUsers := []string{"teleport-user1", "teleport-user2", "teleport-user3"}
		for _, user := range deleteableUsers {
			_, _, err := users.CreateUser(user, &services.HostUsersInfo{})
			require.NoError(t, err)
		}
		_, err = host.UserAdd("teleport-user4", []string{})
		require.NoError(t, err)

		t.Cleanup(cleanupUsersAndGroups(
			[]string{"teleport-user1", "teleport-user2", "teleport-user3", "teleport-user4"},
			[]string{types.TeleportServiceGroup}))

		err = users.DeleteAllUsers()
		require.NoError(t, err)

		_, err = user.Lookup("teleport-user4")
		require.NoError(t, err)

		for _, us := range deleteableUsers {
			_, err := user.Lookup(us)
			require.Equal(t, err, user.UnknownUserError(us))
		}
	})
}
