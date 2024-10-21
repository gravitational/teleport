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
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/teleport/lib/utils/uds"
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
							require.Equal(t, teleport.ParkSubCommand, arg[0])
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

func newSocketPair(t *testing.T) (localConn *net.UnixConn, remoteFD *os.File) {
	localConn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, remoteConn.Close())
		require.NoError(t, localConn.Close())
	})
	remoteFD, err = remoteConn.File()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, remoteFD.Close())
	})
	return localConn, remoteFD
}

func newHTTPTestServer(t *testing.T, listener net.Listener) *httptest.Server {
	var err error
	if listener == nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
	}
	tsrv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world")
	}))
	tsrv.Listener = listener
	tsrv.Start()
	t.Cleanup(tsrv.Close)
	return tsrv
}

func TestLocalPortForwardCommand(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)
	scx.ExecType = teleport.ChanDirectTCPIP

	// Start forwarding subprocess.
	controlConn, controlFD := newSocketPair(t)
	command, err := ConfigureCommand(scx, controlFD)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, command.Process.Kill())
	})
	require.NoError(t, command.Start())

	// Create a client that will dial via the forwarder.
	httpClient := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				dialConn, dialFD := newSocketPair(t)
				if _, _, err := uds.WriteWithFDs(controlConn, []byte(addr), []*os.File{dialFD}); err != nil {
					return nil, trace.Wrap(err)
				}
				return dialConn, nil
			},
		},
	}

	// Test the dialer on an http server.
	tsrv := newHTTPTestServer(t, nil)
	resp, err := httpClient.Get(tsrv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello, world", string(body))
}

func testRemotePortForwardCommand(t *testing.T, login string) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)
	scx.ExecType = teleport.TCPIPForwardRequest
	if login != "" {
		scx.Identity.Login = login
	}

	// Start forwarding subprocess.
	controlConn, controlFD := newSocketPair(t)
	command, err := ConfigureCommand(scx, controlFD)
	require.NoError(t, err)
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		require.NoError(t, command.Process.Kill())
		_, err := command.Process.Wait()
		require.NoError(t, err)
	})

	// Request a listener from the forwarder.
	replyConn, replyFD := newSocketPair(t)
	_, _, err = uds.WriteWithFDs(controlConn, []byte("127.0.0.1:0"), []*os.File{replyFD})
	require.NoError(t, err)
	var fbuf [1]*os.File
	_, fn, err := uds.ReadWithFDs(replyConn, nil, fbuf[:])
	require.NoError(t, err)
	require.Equal(t, 1, fn)
	listener, err := net.FileListener(fbuf[0])
	require.NoError(t, err)

	// Test the listener on an http server.
	tsrv := newHTTPTestServer(t, listener)
	resp, err := http.Get(tsrv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello, world", string(body))
}

func TestRemotePortForwardCommand(t *testing.T) {
	t.Parallel()
	testRemotePortForwardCommand(t, "")
}

// TestRootRemotePortForwardCommand tests that remote port forwarding works
// for a user different than the one running a node (which we need to run
// as root to create).
func TestRootRemotePortForwardCommand(t *testing.T) {
	utils.RequireRoot(t)

	login := utils.GenerateLocalUsername(t)
	_, err := host.UserAdd(login, nil, "", "", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := host.UserDel(login)
		require.NoError(t, err)
	})

	testRemotePortForwardCommand(t, login)
}

func TestRootCheckHomeDir(t *testing.T) {
	utils.RequireRoot(t)

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

	login := utils.GenerateLocalUsername(t)
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

func TestRootOpenFileAsUser(t *testing.T) {
	utils.RequireRoot(t)
	euid := os.Geteuid()
	egid := os.Getegid()

	username := "processing-user"

	arg := os.Args[1]
	os.Args[1] = teleport.ExecSubCommand
	defer func() {
		os.Args[1] = arg
	}()

	_, err := host.UserAdd(username, nil, "", "", "")
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
