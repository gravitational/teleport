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
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/host"
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
	sudoersTestDir := t.TempDir()
	backend := srv.HostUsersProvisioningBackend{
		SudoersPath: sudoersTestDir,
		HostUUID:    "hostuuid",
	}
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

	t.Run("Test CreateUser and group", func(t *testing.T) {
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

	t.Run("Test sudoers", func(t *testing.T) {
		if _, err := exec.LookPath("visudo"); err != nil {
			t.Skip("visudo not found on path")
		}
		validSudoersEntry := []byte("root ALL=(ALL) ALL")
		err := backend.CheckSudoers(validSudoersEntry)
		require.NoError(t, err)
		invalidSudoersEntry := []byte("yipee i broke sudo!!!!")
		err = backend.CheckSudoers(invalidSudoersEntry)
		require.Contains(t, err.Error(), "visudo: invalid sudoers file")
		// test sudoers entry containing . or ~
		require.NoError(t, backend.WriteSudoersFile("user.name", validSudoersEntry))
		_, err = os.Stat(filepath.Join(sudoersTestDir, "teleport-hostuuid-user_name"))
		require.NoError(t, err)
		require.NoError(t, backend.RemoveSudoersFile("user.name"))
		_, err = os.Stat(filepath.Join(sudoersTestDir, "teleport-hostuuid-user_name"))
		require.True(t, os.IsNotExist(err))
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
	ctx := context.Background()
	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	presence := local.NewPresenceService(bk)

	t.Run("test create temporary user and close", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")

		testGroups := []string{"group1", "group2"}
		closer, err := users.CreateUser(testuser, &services.HostUsersInfo{Groups: testGroups, Mode: types.CreateHostUserMode_HOST_USER_MODE_DROP})
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

	t.Run("test create sudoers enabled users", func(t *testing.T) {
		if _, err := exec.LookPath("visudo"); err != nil {
			t.Skip("Visudo not found on path")
		}
		uuid := "host_uuid"
		users := srv.NewHostUsers(context.Background(), presence, uuid)

		sudoersPath := func(username, uuid string) string {
			return fmt.Sprintf("/etc/sudoers.d/teleport-%s-%s", uuid, username)
		}

		t.Cleanup(func() {
			os.Remove(sudoersPath(testuser, uuid))
			host.UserDel(testuser)
		})
		closer, err := users.CreateUser(testuser,
			&services.HostUsersInfo{
				Sudoers: []string{"ALL=(ALL) ALL"},
				Mode:    types.CreateHostUserMode_HOST_USER_MODE_DROP,
			})
		require.NoError(t, err)
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.NoError(t, err)

		// delete the user and ensure the sudoers file got deleted
		require.NoError(t, closer.Close())
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.True(t, os.IsNotExist(err))

		// ensure invalid sudoers entries dont get written
		closer, err = users.CreateUser(testuser,
			&services.HostUsersInfo{
				Sudoers: []string{"badsudoers entry!!!"},
				Mode:    types.CreateHostUserMode_HOST_USER_MODE_DROP,
			})
		require.Error(t, err)
		defer closer.Close()
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("test delete all users in teleport service group", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
		users.SetHostUserDeletionGrace(0)

		deleteableUsers := []string{"teleport-user1", "teleport-user2", "teleport-user3"}
		for _, user := range deleteableUsers {
			_, err := users.CreateUser(user, &services.HostUsersInfo{Mode: types.CreateHostUserMode_HOST_USER_MODE_DROP})
			require.NoError(t, err)
		}

		// this user should not be in the service group as it was created with mode keep.
		closer, err := users.CreateUser("teleport-user4", &services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		})
		require.NoError(t, err)
		require.Nil(t, closer)

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
