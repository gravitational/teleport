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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
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
	usersbk := srv.HostUsersProvisioningBackend{}
	sudoersbk := srv.HostSudoersProvisioningBackend{
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
		err := usersbk.CreateGroup(testgroup, "")
		require.NoError(t, err)
		err = usersbk.CreateGroup(testgroup, "")
		require.True(t, trace.IsAlreadyExists(err))
	})

	t.Run("Test CreateUser and group", func(t *testing.T) {
		err := usersbk.CreateUser(testuser, []string{testgroup}, "", "", "")
		require.NoError(t, err)

		tuser, err := usersbk.Lookup(testuser)
		require.NoError(t, err)

		group, err := usersbk.LookupGroup(testgroup)
		require.NoError(t, err)

		tuserGids, err := tuser.GroupIds()
		require.NoError(t, err)
		require.Contains(t, tuserGids, group.Gid)

		err = usersbk.CreateUser(testuser, []string{}, "", "", "")
		require.True(t, trace.IsAlreadyExists(err))

		require.NoFileExists(t, filepath.Join("/home", testuser))
		err = usersbk.CreateHomeDirectory(testuser, tuser.Uid, tuser.Gid)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join("/home", testuser, ".bashrc"))
	})

	t.Run("Test DeleteUser", func(t *testing.T) {
		err := usersbk.DeleteUser(testuser)
		require.NoError(t, err)

		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("Test GetAllUsers", func(t *testing.T) {
		checkUsers := []string{"teleport-usera", "teleport-userb", "teleport-userc"}
		t.Cleanup(func() {
			for _, u := range checkUsers {
				usersbk.DeleteUser(u)
			}
		})
		for _, u := range checkUsers {
			err := usersbk.CreateUser(u, []string{}, "", "", "")
			require.NoError(t, err)
		}

		users, err := usersbk.GetAllUsers()
		require.NoError(t, err)
		require.Subset(t, users, append(checkUsers, "root"))
	})

	t.Run("Test sudoers", func(t *testing.T) {
		if _, err := exec.LookPath("visudo"); err != nil {
			t.Skip("visudo not found on path")
		}
		validSudoersEntry := []byte("root ALL=(ALL) ALL")
		err := sudoersbk.CheckSudoers(validSudoersEntry)
		require.NoError(t, err)
		invalidSudoersEntry := []byte("yipee i broke sudo!!!!")
		err = sudoersbk.CheckSudoers(invalidSudoersEntry)
		require.Contains(t, err.Error(), "visudo: invalid sudoers file")
		// test sudoers entry containing . or ~
		require.NoError(t, sudoersbk.WriteSudoersFile("user.name", validSudoersEntry))
		_, err = os.Stat(filepath.Join(sudoersTestDir, "teleport-hostuuid-user_name"))
		require.NoError(t, err)
		require.NoError(t, sudoersbk.RemoveSudoersFile("user.name"))
		_, err = os.Stat(filepath.Join(sudoersTestDir, "teleport-hostuuid-user_name"))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("Test CreateHomeDirectory does not follow symlinks", func(t *testing.T) {
		err := usersbk.CreateUser(testuser, []string{testgroup}, "", "", "")
		require.NoError(t, err)

		tuser, err := usersbk.Lookup(testuser)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile("/etc/skel/testfile", []byte("test\n"), 0o700))

		bashrcPath := filepath.Join("/home", testuser, ".bashrc")
		require.NoFileExists(t, bashrcPath)

		require.NoError(t, os.MkdirAll(filepath.Join("/home", testuser), 0o700))

		require.NoError(t, os.Symlink("/tmp/ignoreme", bashrcPath))
		require.NoFileExists(t, "/tmp/ignoreme")

		err = usersbk.CreateHomeDirectory(testuser, tuser.Uid, tuser.Gid)
		t.Cleanup(func() {
			os.RemoveAll(filepath.Join("/home", testuser))
		})
		require.NoError(t, err)
		require.NoFileExists(t, "/tmp/ignoreme")
	})
}

func getUserGroups(t *testing.T, u *user.User) []string {
	var userGroups []string
	userGids, err := u.GroupIds()
	require.NoError(t, err)
	for _, gid := range userGids {
		group, err := user.LookupGroupId(gid)
		require.NoError(t, err)
		userGroups = append(userGroups, group.Name)
	}
	return userGroups
}

func requireUserInGroups(t *testing.T, u *user.User, requiredGroups []string) {
	require.Subset(t, getUserGroups(t, u), requiredGroups)
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
		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Groups: testGroups, Mode: types.CreateHostUserMode_HOST_USER_MODE_DROP})
		require.NoError(t, err)

		testGroups = append(testGroups, types.TeleportDropGroup)
		t.Cleanup(cleanupUsersAndGroups([]string{testuser}, testGroups))

		u, err := user.Lookup(testuser)
		require.NoError(t, err)
		requireUserInGroups(t, u, testGroups)
		require.NotEmpty(t, u.HomeDir)
		require.DirExists(t, u.HomeDir)

		require.NoError(t, closer.Close())
		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
		require.NoDirExists(t, u.HomeDir)
	})

	t.Run("test create temporary user without home dir", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")

		testGroups := []string{"group1", "group2"}
		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Groups: testGroups, Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP})
		require.NoError(t, err)

		testGroups = append(testGroups, types.TeleportDropGroup)
		t.Cleanup(cleanupUsersAndGroups([]string{testuser}, testGroups))

		u, err := user.Lookup(testuser)
		require.NoError(t, err)
		requireUserInGroups(t, u, testGroups)
		require.NoDirExists(t, u.HomeDir)

		require.NoError(t, closer.Close())
		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("test create user with uid and gid", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")

		testUID := "1234"
		testGID := "1337"

		_, err := user.LookupGroupId(testGID)
		require.ErrorIs(t, err, user.UnknownGroupIdError(testGID))

		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
			UID:  testUID,
			GID:  testGID,
		})
		require.NoError(t, err)

		t.Cleanup(cleanupUsersAndGroups([]string{testuser}, []string{types.TeleportDropGroup}))

		group, err := user.LookupGroupId(testGID)
		require.NoError(t, err)
		require.Equal(t, testuser, group.Name)

		u, err := user.Lookup(testuser)
		require.NoError(t, err)

		require.Equal(t, u.Uid, testUID)
		require.Equal(t, u.Gid, testGID)

		require.NoError(t, closer.Close())
		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("test create permanent user", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
		expectedHome := filepath.Join("/home", testuser)
		require.NoDirExists(t, expectedHome)

		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Mode: types.CreateHostUserMode_HOST_USER_MODE_KEEP})
		require.NoError(t, err)
		require.Nil(t, closer)
		t.Cleanup(cleanupUsersAndGroups([]string{testuser}, []string{types.TeleportKeepGroup}))

		u, err := user.Lookup(testuser)
		require.NoError(t, err)
		require.Equal(t, expectedHome, u.HomeDir)
		require.DirExists(t, expectedHome)
		t.Cleanup(func() {
			os.RemoveAll(expectedHome)
		})
	})

	t.Run("test create sudoers enabled users", func(t *testing.T) {
		if _, err := exec.LookPath("visudo"); err != nil {
			t.Skip("Visudo not found on path")
		}
		uuid := "host_uuid"
		users := srv.NewHostUsers(context.Background(), presence, uuid)
		sudoers := srv.NewHostSudoers(uuid)

		sudoersPath := func(username, uuid string) string {
			return fmt.Sprintf("/etc/sudoers.d/teleport-%s-%s", uuid, username)
		}

		t.Cleanup(func() {
			os.Remove(sudoersPath(testuser, uuid))
			cleanupUsersAndGroups([]string{testuser}, nil)
		})
		closer, err := users.UpsertUser(testuser,
			services.HostUsersInfo{
				Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
			})
		require.NoError(t, err)
		err = sudoers.WriteSudoers(testuser, []string{"ALL=(ALL) ALL"})
		require.NoError(t, err)
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.NoError(t, err)

		// delete the user and ensure the sudoers file got deleted
		require.NoError(t, closer.Close())
		require.NoError(t, sudoers.RemoveSudoers(testuser))
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.True(t, os.IsNotExist(err))

		// ensure invalid sudoers entries dont get written
		err = sudoers.WriteSudoers(testuser,
			[]string{"badsudoers entry!!!"},
		)
		require.Error(t, err)
		_, err = os.Stat(sudoersPath(testuser, uuid))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("test delete all users in teleport service group", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
		users.SetHostUserDeletionGrace(0)

		deleteableUsers := []string{"teleport-user1", "teleport-user2", "teleport-user3"}
		for _, user := range deleteableUsers {
			_, err := users.UpsertUser(user, services.HostUsersInfo{Mode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP})
			require.NoError(t, err)
		}

		// this user should not be in the service group as it was created with mode keep.
		closer, err := users.UpsertUser("teleport-user4", services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		})
		require.NoError(t, err)
		require.Nil(t, closer)

		t.Cleanup(cleanupUsersAndGroups(
			[]string{"teleport-user1", "teleport-user2", "teleport-user3", "teleport-user4"},
			[]string{types.TeleportDropGroup, types.TeleportKeepGroup}))

		err = users.DeleteAllUsers()
		require.NoError(t, err)

		_, err = user.Lookup("teleport-user4")
		require.NoError(t, err)

		for _, us := range deleteableUsers {
			_, err := user.Lookup(us)
			require.Equal(t, err, user.UnknownUserError(us))
		}
	})

	t.Run("test update changed groups", func(t *testing.T) {
		tests := []struct {
			name         string
			firstGroups  []string
			secondGroups []string
		}{
			{
				name:         "add groups",
				secondGroups: []string{"group1", "group2"},
			},
			{
				name:        "delete groups",
				firstGroups: []string{"group1", "group2"},
			},
			{
				name:         "change groups",
				firstGroups:  []string{"group1", "group2"},
				secondGroups: []string{"group2", "group3"},
			},
			{
				name:         "no change",
				firstGroups:  []string{"group1", "group2"},
				secondGroups: []string{"group2", "group1"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				allGroups := make([]string, 0, len(tc.firstGroups)+len(tc.secondGroups))
				allGroups = append(allGroups, tc.firstGroups...)
				allGroups = append(allGroups, tc.secondGroups...)
				t.Cleanup(cleanupUsersAndGroups([]string{testuser}, allGroups))

				// Verify that the user is created with the first set of groups.
				users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
				_, err := users.UpsertUser(testuser, services.HostUsersInfo{
					Groups: tc.firstGroups,
					Mode:   types.CreateHostUserMode_HOST_USER_MODE_KEEP,
				})
				require.NoError(t, err)
				u, err := user.Lookup(testuser)
				require.NoError(t, err)
				requireUserInGroups(t, u, tc.firstGroups)

				// Verify that the user is updated with the second set of groups.
				_, err = users.UpsertUser(testuser, services.HostUsersInfo{
					Groups: tc.secondGroups,
					Mode:   types.CreateHostUserMode_HOST_USER_MODE_KEEP,
				})
				require.NoError(t, err)
				u, err = user.Lookup(testuser)
				require.NoError(t, err)
				requireUserInGroups(t, u, tc.secondGroups)

				// Verify that the appropriate groups form the first set were deleted.
				userGroups := getUserGroups(t, u)
				for _, group := range tc.firstGroups {
					if !slices.Contains(tc.secondGroups, group) {
						require.NotContains(t, userGroups, group)
					}
				}
			})
		}
	})

	t.Run("Test expiration removal", func(t *testing.T) {
		expiredUser := "expired-user"
		backendExpiredUser := "backend-expired-user"
		t.Cleanup(func() { cleanupUsersAndGroups([]string{expiredUser, backendExpiredUser}, []string{"test-group"}) })

		defaultBackend, err := srv.DefaultHostUsersBackend()
		require.NoError(t, err)

		backend := &hostUsersBackendWithExp{HostUsersBackend: defaultBackend}
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid", srv.WithHostUsersBackend(backend))

		// Make sure the backend actually creates expired users
		err = backend.CreateUser("backend-expired-user", nil, "", "", "")
		require.NoError(t, err)

		hasExpirations, _, err := host.UserHasExpirations(backendExpiredUser)
		require.NoError(t, err)
		require.True(t, hasExpirations)

		// Upsert a new user which should have the expirations removed
		_, err = users.UpsertUser(expiredUser, services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		})
		require.NoError(t, err)

		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.False(t, hasExpirations)

		// Expire existing user so we can test that updates also remove expirations
		expireUser := func(username string) error {
			chageBin, err := exec.LookPath("chage")
			require.NoError(t, err)

			cmd := exec.Command(chageBin, "-E", "1", "-I", "1", "-M", "1", username)
			return cmd.Run()
		}
		require.NoError(t, expireUser(expiredUser))
		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.True(t, hasExpirations)

		// Update user without any changes
		_, err = users.UpsertUser(expiredUser, services.HostUsersInfo{
			Mode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		})
		require.NoError(t, err)

		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.False(t, hasExpirations)

		// Reinstate expirations again
		require.NoError(t, expireUser(expiredUser))
		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.True(t, hasExpirations)

		// Update user with changes
		_, err = users.UpsertUser(expiredUser, services.HostUsersInfo{
			Mode:   types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			Groups: []string{"test-group"},
		})
		require.NoError(t, err)

		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.False(t, hasExpirations)
	})
}

type hostUsersBackendWithExp struct {
	srv.HostUsersBackend
}

func (u *hostUsersBackendWithExp) CreateUser(name string, groups []string, home, uid, gid string) error {
	if err := u.HostUsersBackend.CreateUser(name, groups, home, uid, gid); err != nil {
		return trace.Wrap(err)
	}

	chageBin, err := exec.LookPath("chage")
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.Command(chageBin, "-E", "1", "-I", "1", "-M", "1", name)
	return cmd.Run()
}

func TestRootLoginAsHostUser(t *testing.T) {
	requireRoot(t)
	// Create test instance.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	instance := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Log:         utils.NewLoggerForTests(),
	})

	// Create a user that can create a host user.
	username := "test-user"
	login := generateLocalUsername(t)
	groups := []string{"foo", "bar"}
	role, err := types.NewRole("ssh-host-user", types.RoleSpecV6{
		Options: types.RoleOptions{
			CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		},
		Allow: types.RoleConditions{
			Logins:     []string{login},
			HostGroups: groups,
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)
	instance.Secrets.Users[username] = &helpers.User{
		Username:      username,
		AllowedLogins: []string{login},
		Roles:         []types.Role{role},
	}

	require.NoError(t, instance.Create(t, nil, true, nil))
	require.NoError(t, instance.Start())
	t.Cleanup(func() {
		require.NoError(t, instance.StopAll())
	})

	tests := []struct {
		name      string
		command   []string
		stdinText string
	}{
		{
			name:    "non-interactive session",
			command: []string{"whoami"},
		},
		{
			name:      "interactive session",
			stdinText: "whoami\nexit\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdin := bytes.NewBufferString(tc.stdinText)
			stdout := utils.NewSyncBuffer()
			t.Cleanup(func() {
				require.NoError(t, stdout.Close())
			})

			client, err := instance.NewClient(helpers.ClientConfig{
				TeleportUser: username,
				Login:        login,
				Cluster:      helpers.Site,
				Host:         Host,
				Port:         helpers.Port(t, instance.SSH),
				Stdin:        stdin,
				Stdout:       stdout,
			})
			require.NoError(t, err)

			// Run an SSH session to completion.
			t.Cleanup(cleanupUsersAndGroups([]string{login}, groups))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			err = client.SSH(ctx, tc.command)
			require.NoError(t, err)
			// Check for correct result from whoami command.
			require.Contains(t, stdout.String(), login)

			// Verify that a host user was created.
			u, err := user.Lookup(login)
			require.NoError(t, err)
			createdGroups, err := u.GroupIds()
			require.NoError(t, err)
			for _, group := range createdGroups {
				_, err := user.LookupGroupId(group)
				require.NoError(t, err)
			}
		})
	}
}
