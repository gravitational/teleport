/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package srv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/host"
)

type stubUser struct {
	gid      string
	uid      string
	groupIDS []string
}

func (s *stubUser) GID() string {
	return s.gid
}

func (s *stubUser) UID() string {
	return s.uid
}

func (s *stubUser) GroupIds() ([]string, error) {
	return s.groupIDS, nil
}

func TestStartNewParker(t *testing.T) {
	currentUser, err := user.Current()
	require.NoError(t, err)
	currentUID, err := strconv.ParseUint(currentUser.Uid, 10, 32)
	require.NoError(t, err)
	currentGID, err := strconv.ParseUint(currentUser.Gid, 10, 32)
	require.NoError(t, err)

	t.Parallel()

	type args struct {
		credential  *syscall.Credential
		loginAsUser string
		localUser   *stubUser
	}
	tests := []struct {
		name      string
		args      args
		newOsPack func(t *testing.T) (*osWrapper, func())
		wantErr   require.ErrorAssertionFunc
	}{
		{
			name:    "empty credentials does nothing",
			wantErr: require.NoError,
			newOsPack: func(t *testing.T) (*osWrapper, func()) {
				return &osWrapper{}, func() {}
			},
		},
		{
			name:    "missing Teleport group returns no error",
			wantErr: require.NoError,
			newOsPack: func(t *testing.T) (*osWrapper, func()) {
				return &osWrapper{
					LookupGroup: func(name string) (*user.Group, error) {
						require.Equal(t, types.TeleportDropGroup, name)
						return nil, user.UnknownGroupError(types.TeleportDropGroup)
					},
				}, func() {}
			},
		},
		{
			name:    "different group doesn't start parker",
			wantErr: require.NoError,
			newOsPack: func(t *testing.T) (*osWrapper, func()) {
				return &osWrapper{
					LookupGroup: func(name string) (*user.Group, error) {
						require.Equal(t, types.TeleportDropGroup, name)
						return &user.Group{Gid: "1234"}, nil
					},
					CommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
						require.FailNow(t, "CommandContext should not be called")
						return nil
					},
				}, func() {}
			},
			args: args{
				credential: &syscall.Credential{Gid: 1000},
				localUser: &stubUser{
					uid:      "1001",
					gid:      "1003",
					groupIDS: []string{"1003"},
				},
			},
		},
		{
			name:    "parker is started",
			wantErr: require.NoError,
			newOsPack: func(t *testing.T) (*osWrapper, func()) {
				parkerStarted := false

				return &osWrapper{
						LookupGroup: func(name string) (*user.Group, error) {
							require.Equal(t, types.TeleportDropGroup, name)
							return &user.Group{Gid: currentUser.Gid}, nil
						},
						CommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
							require.NotNil(t, ctx)
							require.Len(t, arg, 1)
							require.Equal(t, arg[0], teleport.ParkSubCommand)
							parkerStarted = true
							return exec.CommandContext(ctx, name, arg...)
						},
						LookupUser: func(username string) (*user.User, error) {
							return &user.User{Uid: currentUser.Uid, Gid: currentUser.Gid}, nil
						},
					}, func() {
						require.True(t, parkerStarted, "parker process didn't start")
					}
			},
			args: args{
				credential: &syscall.Credential{
					Uid: uint32(currentUID),
					Gid: uint32(currentGID),
					// Changing to false causes "fork/exec /proc/self/exe: operation not permitted"
					// to be returned when creating the park process.
					NoSetGroups: true,
				},
				localUser: &stubUser{
					uid:      currentUser.Uid,
					gid:      currentUser.Gid,
					groupIDS: []string{currentUser.Gid},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osPack, assertExpected := tt.newOsPack(t)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel) // cancel to stop the park process.

			err := osPack.startNewParker(ctx, tt.args.credential, tt.args.loginAsUser, tt.args.localUser)
			tt.wantErr(t, err, fmt.Sprintf("startNewParker(%v, %+v, %v, %+v)", ctx, tt.args.credential, tt.args.loginAsUser, tt.args.localUser))

			assertExpected()
		})
	}
}

func TestRootCheckHomeDir(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("This test will be skipped because tests are not being run as root")
	}

	tmp := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(tmp), 0777))
	require.NoError(t, os.Chmod(tmp, 0777))

	home := filepath.Join(tmp, "home")
	noAccess := filepath.Join(tmp, "no_access")
	file := filepath.Join(tmp, "file")
	notFound := filepath.Join(tmp, "not_found")

	require.NoError(t, os.Mkdir(home, 0700))
	require.NoError(t, os.Mkdir(noAccess, 0700))
	_, err := os.Create(file)
	require.NoError(t, err)

	login := "test-user-check-home-dir"
	_, err = host.UserAdd(login, nil, home, "", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		// change back to accessible home so deletion works
		changeHomeDir(t, login, home)
		_, err := host.UserDel(login)
		require.NoError(t, err)
	})

	testUser, err := user.Lookup(login)
	require.NoError(t, err)

	uid, err := strconv.Atoi(testUser.Uid)
	require.NoError(t, err)

	gid, err := strconv.Atoi(testUser.Gid)
	require.NoError(t, err)

	require.NoError(t, os.Chown(home, uid, gid))
	require.NoError(t, os.Chown(file, uid, gid))

	hasAccess, err := CheckHomeDir(testUser)
	require.NoError(t, err)
	require.True(t, hasAccess)

	changeHomeDir(t, login, file)
	hasAccess, err = CheckHomeDir(testUser)
	require.NoError(t, err)
	require.False(t, hasAccess)

	changeHomeDir(t, login, notFound)
	hasAccess, err = CheckHomeDir(testUser)
	require.NoError(t, err)
	require.False(t, hasAccess)

	changeHomeDir(t, login, noAccess)
	hasAccess, err = CheckHomeDir(testUser)
	require.NoError(t, err)
	require.False(t, hasAccess)
}

func changeHomeDir(t *testing.T, username, home string) {
	usermodBin, err := exec.LookPath("usermod")
	assert.NoError(t, err, "usermod binary must be present")

	cmd := exec.Command(usermodBin, "--home", home, username)
	_, err = cmd.CombinedOutput()
	assert.NoError(t, err, "changing home should not error")
	assert.Equal(t, 0, cmd.ProcessState.ExitCode(), "changing home should exit 0")
}
