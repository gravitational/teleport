//go:build linux
// +build linux

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

package integration

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
)

const testuser = "teleport-testuser"
const testgroup = "teleport-testgroup"

func getUserShells(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	userShells := make(map[string]string)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		userShells[parts[0]] = parts[len(parts)-1]
	}

	return userShells, nil
}

func TestRootHostUsersBackend(t *testing.T) {
	utils.RequireRoot(t)
	sudoersTestDir := t.TempDir()
	usersbk := srv.HostUsersProvisioningBackend{}
	sudoersbk := srv.HostSudoersProvisioningBackend{
		SudoersPath: sudoersTestDir,
		HostUUID:    "hostuuid",
	}

	t.Run("Test CreateGroup", func(t *testing.T) {
		t.Cleanup(func() { cleanupUsersAndGroups(nil, []string{testgroup}) })

		err := usersbk.CreateGroup(testgroup, "")
		require.NoError(t, err)
		err = usersbk.CreateGroup(testgroup, "")
		require.True(t, trace.IsAlreadyExists(err))

		_, err = usersbk.LookupGroup(testgroup)
		require.NoError(t, err)
	})

	t.Run("Test CreateUser and group", func(t *testing.T) {
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, []string{testgroup}) })
		err := usersbk.CreateGroup(testgroup, "")
		require.NoError(t, err)

		testHome := filepath.Join("/home", testuser)
		err = usersbk.CreateUser(testuser, []string{testgroup}, host.UserOpts{Home: testHome})
		require.NoError(t, err)

		tuser, err := usersbk.Lookup(testuser)
		require.NoError(t, err)

		group, err := usersbk.LookupGroup(testgroup)
		require.NoError(t, err)

		tuserGids, err := tuser.GroupIds()
		require.NoError(t, err)
		require.Contains(t, tuserGids, group.Gid)

		err = usersbk.CreateUser(testuser, []string{}, host.UserOpts{Home: testHome})
		require.True(t, trace.IsAlreadyExists(err))

		require.NoFileExists(t, testHome)
		err = usersbk.CreateHomeDirectory(testHome, tuser.Uid, tuser.Gid)
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(testHome)
		})
		require.FileExists(t, filepath.Join(testHome, ".bashrc"))
	})

	t.Run("Test DeleteUser", func(t *testing.T) {
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, nil) })
		err := usersbk.CreateUser(testuser, nil, host.UserOpts{})
		require.NoError(t, err)
		_, err = usersbk.Lookup(testuser)
		require.NoError(t, err)

		err = usersbk.DeleteUser(testuser)
		require.NoError(t, err)

		_, err = user.Lookup(testuser)
		require.Equal(t, err, user.UnknownUserError(testuser))
	})

	t.Run("Test GetAllUsers", func(t *testing.T) {
		checkUsers := []string{"teleport-usera", "teleport-userb", "teleport-userc"}
		t.Cleanup(func() { cleanupUsersAndGroups(checkUsers, nil) })

		for _, u := range checkUsers {
			err := usersbk.CreateUser(u, []string{}, host.UserOpts{})
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
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, nil) })
		err := usersbk.CreateUser(testuser, nil, host.UserOpts{})
		require.NoError(t, err)

		tuser, err := usersbk.Lookup(testuser)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile("/etc/skel/testfile", []byte("test\n"), 0o700))

		bashrcPath := filepath.Join("/home", testuser, ".bashrc")
		require.NoFileExists(t, bashrcPath)

		testHome := filepath.Join("/home", testuser)
		require.NoError(t, os.MkdirAll(testHome, 0o700))

		require.NoError(t, os.Symlink("/tmp/ignoreme", bashrcPath))
		require.NoFileExists(t, "/tmp/ignoreme")

		err = usersbk.CreateHomeDirectory(testHome, tuser.Uid, tuser.Gid)
		t.Cleanup(func() {
			os.RemoveAll(testHome)
		})
		require.ErrorIs(t, err, os.ErrExist)
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

func cleanupUsersAndGroups(users []string, groups []string) {
	for _, group := range groups {
		cmd := exec.Command("groupdel", group)
		err := cmd.Run()
		if err != nil {
			slog.DebugContext(context.Background(), "Error deleting group", "group", group, "error", err)
		}
	}
	for _, user := range users {
		host.UserDel(user)
	}
}

func sudoersPath(username, uuid string) string {
	return fmt.Sprintf("/etc/sudoers.d/teleport-%s-%s", uuid, username)
}

func TestRootHostUsers(t *testing.T) {
	utils.RequireRoot(t)
	ctx := context.Background()
	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	presence := local.NewPresenceService(bk)

	t.Run("test create temporary user without home dir", func(t *testing.T) {
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")

		testGroups := []string{"group1", "group2"}
		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Groups: testGroups, Mode: services.HostUserModeDrop})
		require.NoError(t, err)

		testGroups = append(testGroups, types.TeleportDropGroup)
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, testGroups) })

		u, err := user.Lookup(testuser)
		require.NoError(t, err)
		requireUserInGroups(t, u, testGroups)
		require.Equal(t, string(os.PathSeparator), u.HomeDir)

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
			Mode: services.HostUserModeDrop,
			UID:  testUID,
			GID:  testGID,
		})
		require.NoError(t, err)

		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, []string{types.TeleportDropGroup}) })

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

		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Mode: services.HostUserModeKeep})
		require.NoError(t, err)
		require.Nil(t, closer)
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, []string{types.TeleportKeepGroup}) })

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

		t.Cleanup(func() {
			os.Remove(sudoersPath(testuser, uuid))
			cleanupUsersAndGroups([]string{testuser}, nil)
		})
		closer, err := users.UpsertUser(testuser,
			services.HostUsersInfo{
				Mode: services.HostUserModeDrop,
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
			_, err := users.UpsertUser(user, services.HostUsersInfo{Mode: services.HostUserModeDrop})
			require.NoError(t, err)
		}

		// this user should not be in the service group as it was created with mode keep.
		closer, err := users.UpsertUser("teleport-user4", services.HostUsersInfo{
			Mode: services.HostUserModeKeep,
		})
		require.NoError(t, err)
		require.Nil(t, closer)

		t.Cleanup(func() {
			cleanupUsersAndGroups(
				[]string{"teleport-user1", "teleport-user2", "teleport-user3", "teleport-user4"},
				[]string{types.TeleportDropGroup, types.TeleportKeepGroup})
		})

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
				t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, slices.Concat(tc.firstGroups, tc.secondGroups)) })

				// Verify that the user is created with the first set of groups.
				users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
				_, err := users.UpsertUser(testuser, services.HostUsersInfo{
					Groups: tc.firstGroups,
					Mode:   services.HostUserModeKeep,
				})
				require.NoError(t, err)
				u, err := user.Lookup(testuser)
				require.NoError(t, err)
				requireUserInGroups(t, u, tc.firstGroups)

				// Verify that the user is updated with the second set of groups.
				_, err = users.UpsertUser(testuser, services.HostUsersInfo{
					Groups: tc.secondGroups,
					Mode:   services.HostUserModeKeep,
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

	t.Run("Test default shell assignment", func(t *testing.T) {
		defaultShellUser := "default-shell"
		namedShellUser := "named-shell"
		absoluteShellUser := "absolute-shell"

		t.Cleanup(func() { cleanupUsersAndGroups([]string{defaultShellUser, namedShellUser, absoluteShellUser}, nil) })

		// Create a user with a named shell expected to be available in the PATH
		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
		_, err := users.UpsertUser(namedShellUser, services.HostUsersInfo{
			Mode:  services.HostUserModeKeep,
			Shell: "bash",
		})
		require.NoError(t, err)

		// Create a user with an absolute path to a shell
		_, err = users.UpsertUser(absoluteShellUser, services.HostUsersInfo{
			Mode:  services.HostUserModeKeep,
			Shell: "/usr/bin/bash",
		})
		require.NoError(t, err)

		// Create a user with the host default shell (default behavior)
		_, err = users.UpsertUser(defaultShellUser, services.HostUsersInfo{
			Mode:  services.HostUserModeKeep,
			Shell: "zsh",
		})
		require.NoError(t, err)

		_, err = user.Lookup(namedShellUser)
		require.NoError(t, err)

		_, err = user.Lookup(absoluteShellUser)
		require.NoError(t, err)

		_, err = user.Lookup(defaultShellUser)
		require.NoError(t, err)

		// Verify users have the correct shell assigned
		userShells, err := getUserShells("/etc/passwd")
		require.NoError(t, err)

		// Using bash and sh for this test because they should be present on predictable paths for most reasonable places we might
		// be running integration tests
		expectedShell := "/usr/bin/bash"

		assert.Equal(t, expectedShell, userShells[namedShellUser])
		assert.Equal(t, expectedShell, userShells[absoluteShellUser])
		assert.NotEqual(t, expectedShell, userShells[defaultShellUser])

		// User's shell should not be overwritten when updating, only when creating a new host user
		_, err = users.UpsertUser(namedShellUser, services.HostUsersInfo{
			Mode:  services.HostUserModeKeep,
			Shell: "sh",
		})
		require.NoError(t, err)

		userShells, err = getUserShells("/etc/passwd")
		require.NoError(t, err)
		assert.Equal(t, expectedShell, userShells[namedShellUser])
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
		err = backend.CreateUser("backend-expired-user", nil, host.UserOpts{})
		require.NoError(t, err)

		hasExpirations, _, err := host.UserHasExpirations(backendExpiredUser)
		require.NoError(t, err)
		require.True(t, hasExpirations)

		// Upsert a new user which should have the expirations removed
		_, err = users.UpsertUser(expiredUser, services.HostUsersInfo{
			Mode: services.HostUserModeKeep,
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
			Mode: services.HostUserModeKeep,
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
			Mode:   services.HostUserModeKeep,
			Groups: []string{"test-group"},
		})
		require.NoError(t, err)

		hasExpirations, _, err = host.UserHasExpirations(expiredUser)
		require.NoError(t, err)
		require.False(t, hasExpirations)
	})

	t.Run("Test migrate unmanaged user", func(t *testing.T) {
		t.Cleanup(func() { cleanupUsersAndGroups([]string{testuser}, []string{types.TeleportKeepGroup}) })

		users := srv.NewHostUsers(context.Background(), presence, "host_uuid")
		_, err := host.UserAdd(testuser, nil, host.UserOpts{})
		require.NoError(t, err)

		closer, err := users.UpsertUser(testuser, services.HostUsersInfo{Mode: services.HostUserModeKeep, Groups: []string{types.TeleportKeepGroup}})
		require.NoError(t, err)
		require.Nil(t, closer)

		u, err := user.Lookup(testuser)
		require.NoError(t, err)

		gids, err := u.GroupIds()
		require.NoError(t, err)

		keepGroup, err := user.LookupGroup(types.TeleportKeepGroup)
		require.NoError(t, err)
		require.Contains(t, gids, keepGroup.Gid)
	})
}

type hostUsersBackendWithExp struct {
	srv.HostUsersBackend
}

func (u *hostUsersBackendWithExp) CreateUser(name string, groups []string, opts host.UserOpts) error {
	if err := u.HostUsersBackend.CreateUser(name, groups, opts); err != nil {
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
	utils.RequireRoot(t)
	// Create test instance.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	instance := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	// Create a user that can create a host user.
	username := "test-user"
	login := utils.GenerateLocalUsername(t)
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

	require.NoError(t, instance.Create(t, nil, true))
	require.NoError(t, instance.Start())
	t.Cleanup(func() {
		require.NoError(t, instance.StopAll())
	})

	instance.WaitForNodeCount(context.Background(), helpers.Site, 1)

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
			t.Cleanup(func() { cleanupUsersAndGroups([]string{login}, groups) })
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

func TestRootStaticHostUsers(t *testing.T) {
	utils.RequireRoot(t)
	// Create test instance.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	instance := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	require.NoError(t, instance.Create(t, nil, false))
	require.NoError(t, instance.Start())
	t.Cleanup(func() {
		require.NoError(t, instance.StopAll())
	})
	nodeCfg := servicecfg.MakeDefaultConfig()
	nodeCfg.SSH.Labels = map[string]string{
		"foo": "bar",
	}
	_, err = instance.StartNode(nodeCfg)
	require.NoError(t, err)

	instance.WaitForNodeCount(context.Background(), helpers.Site, 2)

	// Create host user resources.
	groups := []string{"foo", "bar"}
	goodLogin := utils.GenerateLocalUsername(t)
	goodUser := userprovisioning.NewStaticHostUser(goodLogin, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups:  groups,
				Sudoers: []string{"All = (root) NOPASSWD: /usr/bin/systemctl restart nginx.service"},
			},
		},
	})
	goodLoginWithShell := utils.GenerateLocalUsername(t)
	goodUserWithShell := userprovisioning.NewStaticHostUser(goodLoginWithShell, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups:       groups,
				DefaultShell: "bash",
			},
		},
	})
	nonMatchingLogin := utils.GenerateLocalUsername(t)
	nonMatchingUser := userprovisioning.NewStaticHostUser(nonMatchingLogin, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"baz"},
					},
				},
				Groups: groups,
			},
		},
	})
	conflictingLogin := utils.GenerateLocalUsername(t)
	conflictingUser := userprovisioning.NewStaticHostUser(conflictingLogin, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups: groups,
			},
			{
				NodeLabelsExpression: `labels["foo"] == "bar"`,
				Groups:               groups,
			},
		},
	})

	clt := instance.Process.GetAuthServer()
	for _, hostUser := range []*userprovisioningpb.StaticHostUser{goodUser, goodUserWithShell, nonMatchingUser, conflictingUser} {
		_, err := clt.UpsertStaticHostUser(context.Background(), hostUser)
		require.NoError(t, err)
	}
	t.Cleanup(func() { cleanupUsersAndGroups([]string{goodLogin, nonMatchingLogin, conflictingLogin}, groups) })

	// Test that a node picks up new host users from the cache.
	testStaticHostUsers(t, nodeCfg.HostUUID, goodLogin, goodLoginWithShell, nonMatchingLogin, conflictingLogin, groups)
	cleanupUsersAndGroups([]string{goodLogin, nonMatchingLogin, conflictingLogin}, groups)

	require.NoError(t, instance.StopNodes())
	_, err = instance.StartNode(nodeCfg)
	require.NoError(t, err)
	// Test that a new node picks up existing host users on startup.
	testStaticHostUsers(t, nodeCfg.HostUUID, goodLogin, goodLoginWithShell, nonMatchingLogin, conflictingLogin, groups)

	// Check that a deleted resource doesn't affect the host user.
	require.NoError(t, clt.DeleteStaticHostUser(context.Background(), goodLogin))
	require.NoError(t, clt.DeleteStaticHostUser(context.Background(), goodLoginWithShell))
	var lookupErr error
	var homeDirErr error
	var sudoerErr error
	require.Never(t, func() bool {
		_, lookupErr = user.Lookup(goodLogin)
		_, homeDirErr = os.Stat("/home/" + goodLogin)
		_, sudoerErr = os.Stat(sudoersPath(goodLogin, nodeCfg.HostUUID))
		return lookupErr != nil || homeDirErr != nil || sudoerErr != nil
	}, 5*time.Second, time.Second,
		"lookup err: %v\nhome dir err: %v\nsudoer err: %v\n",
		lookupErr, homeDirErr, sudoerErr)
}

func testStaticHostUsers(t *testing.T, nodeUUID, goodLogin, goodLoginWithShell, nonMatchingLogin, conflictingLogin string, groups []string) {
	t.Cleanup(func() {
		os.Remove(sudoersPath(goodLogin, nodeUUID))
	})

	// Check that the good user was correctly applied.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		// Check that the user was created.
		existingUser, err := user.Lookup(goodLogin)
		assert.NoError(collect, err)
		assert.DirExists(collect, existingUser.HomeDir)
		// Check that the user has the right groups, including teleport-static.
		groupIDs, err := existingUser.GroupIds()
		assert.NoError(collect, err)
		userGroups := make([]string, 0, len(groupIDs))
		for _, gid := range groupIDs {
			group, err := user.LookupGroupId(gid)
			assert.NoError(collect, err)
			userGroups = append(userGroups, group.Name)
		}
		assert.Subset(collect, userGroups, groups)
		assert.Contains(collect, userGroups, types.TeleportStaticGroup)
		// Check that the sudoers file was created.
		assert.FileExists(collect, sudoersPath(goodLogin, nodeUUID))
		userShells, err := getUserShells("/etc/passwd")
		assert.NoError(collect, err)
		assert.Equal(collect, "/usr/bin/bash", userShells[goodLoginWithShell])
	}, 10*time.Second, time.Second)

	// Check that the nonmatching and conflicting users were not created.
	var nonmatchingUserErr error
	var conflictingUserErr error
	require.Never(t, func() bool {
		_, nonmatchingUserErr = user.Lookup(nonMatchingLogin)
		_, conflictingUserErr = user.Lookup(conflictingLogin)
		return nonmatchingUserErr == nil && conflictingUserErr == nil
	}, 5*time.Second, time.Second,
		"nonmatching user error: %v\nconflicting user error: %v\n",
		nonmatchingUserErr, conflictingUserErr,
	)
}
