// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reexec

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiconstants "github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/session/host"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
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
						require.Equal(t, apiconstants.TeleportDropGroup, name)
						return nil, user.UnknownGroupError(apiconstants.TeleportDropGroup)
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
						require.Equal(t, apiconstants.TeleportDropGroup, name)
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
							require.Equal(t, apiconstants.TeleportDropGroup, name)
							return &user.Group{Gid: currentUser.Gid}, nil
						},
						CommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
							require.NotNil(t, ctx)
							require.Len(t, arg, 1)
							require.Equal(t, reexecconstants.ParkSubCommand, arg[0])
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
	requireRoot(t)

	// this test manipulates global state, ensure we're not going to run it in
	// parallel with something else
	t.Setenv("foo", "bar")

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

	login := generateLocalUsername(t)
	_, err = host.UserAdd(login, nil, host.UserOpts{Home: home})
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

func TestRootOpenFileAsUser(t *testing.T) {
	requireRoot(t)
	euid := os.Geteuid()
	egid := os.Getegid()

	username := "processing-user"

	arg := os.Args[1]
	os.Args[1] = reexecconstants.ExecSubCommand
	defer func() {
		os.Args[1] = arg
	}()

	_, err := host.UserAdd(username, nil, host.UserOpts{})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := host.UserDel(username)
		require.NoError(t, err)
	})

	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "testfile")
	fileContent := "one does not simply open without permission"

	err = os.WriteFile(testFile, []byte(fileContent), 0777)
	require.NoError(t, err)

	testUser, err := user.Lookup(username)
	require.NoError(t, err)

	// no access
	file, err := openFileAsUser(testUser, testFile)
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, file)

	// ensure we fallback to root after
	file, err = os.Open(testFile)
	require.NoError(t, err)
	require.NotNil(t, file)
	file.Close()

	// has access
	uid, err := strconv.Atoi(testUser.Uid)
	require.NoError(t, err)

	gid, err := strconv.Atoi(testUser.Gid)
	require.NoError(t, err)

	err = os.Chown(filepath.Dir(tmp), uid, gid)
	require.NoError(t, err)

	err = os.Chown(tmp, uid, gid)
	require.NoError(t, err)

	err = os.Chown(testFile, uid, gid)
	require.NoError(t, err)

	file, err = openFileAsUser(testUser, testFile)
	require.NoError(t, err)
	require.NotNil(t, file)

	data, err := io.ReadAll(file)
	file.Close()
	require.NoError(t, err)
	require.Equal(t, fileContent, string(data))

	// not exist
	file, err = openFileAsUser(testUser, filepath.Join(tmp, "no_exist"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)

	require.Equal(t, euid, os.Geteuid())
	require.Equal(t, egid, os.Getegid())
}

// requireRoot is [testutils.RequireRoot] but inlined.
func requireRoot(tb testing.TB) {
	tb.Helper()
	if os.Geteuid() != 0 {
		tb.Skip("This test will be skipped because tests are not being run as root.")
	}
}

func generateUsername(tb testing.TB) string {
	suffix := make([]byte, 8)
	_, err := rand.Read(suffix)
	require.NoError(tb, err)
	return fmt.Sprintf("teleport-%x", suffix)
}

// generateLocalUsername is [testutils.GenerateLocalUsername] but inlined.
func generateLocalUsername(tb testing.TB) string {
	const maxAttempts = 10
	for range maxAttempts {
		login := generateUsername(tb)
		_, err := user.Lookup(login)
		if errors.Is(err, user.UnknownUserError(login)) {
			return login
		}
		require.NoError(tb, err)
	}
	tb.Fatalf("Unable to generate unused username after %v attempts", maxAttempts)
	return ""
}
